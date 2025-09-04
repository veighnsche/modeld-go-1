package httpapi

import (
	"encoding/json"
	"net/http"
	"context"
	"io"
	"log"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"modeld/internal/manager"
	"modeld/pkg/types"
)

// Service defines the methods required by the HTTP API layer.
type Service interface {
	ListModels() []types.Model
	Status() types.StatusResponse
	Infer(ctx context.Context, req types.InferRequest, w io.Writer, flush func()) error
	Ready() bool
}

// loggingLineWriter logs complete NDJSON lines to the standard logger.
type loggingLineWriter struct {
	buf []byte
}

func (lw *loggingLineWriter) Write(p []byte) (int, error) {
	lw.buf = append(lw.buf, p...)
	for {
		idx := indexByte(lw.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(lw.buf[:idx])
		if len(line) > 0 {
			log.Printf("infer> %s", line)
		}
		lw.buf = lw.buf[idx+1:]
	}
	return len(p), nil
}

func indexByte(b []byte, c byte) int {
	for i := range b {
		if b[i] == c {
			return i
		}
	}
	return -1
}

type LogLevel int

const (
	LevelOff LogLevel = iota
	LevelError
	LevelInfo
	LevelDebug
)

func parseLevel(s string) LogLevel {
	switch s {
	case "off", "":
		return LevelOff
	case "error":
		return LevelError
	case "info":
		return LevelInfo
	case "debug":
		return LevelDebug
	default:
		return LevelInfo
	}
}

// global default, read once
var defaultLogLevel = func() LogLevel {
	// legacy switch for compatibility
	if os.Getenv("MODELD_LOG_INFER") == "1" {
		return LevelDebug
	}
	return parseLevel(os.Getenv("MODELD_LOG_LEVEL"))
}()

func requestLogLevel(r *http.Request) LogLevel {
	// Per-request overrides
	if v := r.URL.Query().Get("log"); v != "" {
		if v == "1" {
			return LevelDebug
		}
		return parseLevel(v)
	}
	if v := r.Header.Get("X-Log-Level"); v != "" {
		return parseLevel(v)
	}
	if r.Header.Get("X-Log-Infer") == "1" { // legacy
		return LevelDebug
	}
	return defaultLogLevel
}

// HTTPError allows services to provide an HTTP status code for an error.
type HTTPError interface {
	error
	StatusCode() int
}

func NewMux(svc Service) http.Handler {
	r := chi.NewRouter()
	// TODO: add middlewares (request id, recoverer, CORS, gzip) later

	r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
		// TODO: read from config/registry
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"models": svc.ListModels()}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(svc.Status()); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.Post("/infer", func(w http.ResponseWriter, r *http.Request) {
		var req types.InferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}

		// Stream NDJSON via manager.Infer (centralized logic)
		w.Header().Set("Content-Type", "application/x-ndjson")
		var flush func()
		if f, ok := w.(http.Flusher); ok {
			flush = f.Flush
		}
		start := time.Now()
		// Optional logging of NDJSON tokens
		writer := io.Writer(w)
		lvl := requestLogLevel(r)
		if lvl >= LevelDebug {
			writer = io.MultiWriter(w, &loggingLineWriter{})
		}
		if lvl >= LevelInfo {
			log.Printf("infer start path=%s model=%s", r.URL.Path, req.Model)
		}
		if err := svc.Infer(r.Context(), req, writer, flush); err != nil {
			// If context was canceled (client disconnect), just return.
			if r.Context().Err() != nil {
				return
			}
			// Map well-known manager errors to HTTP status codes
			if manager.IsModelNotFound(err) {
				http.Error(w, err.Error(), http.StatusNotFound)
				if lvl >= LevelInfo { log.Printf("infer end status=404 dur=%s err=%v", time.Since(start), err) }
				return
			}
			if manager.IsTooBusy(err) {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				if lvl >= LevelInfo { log.Printf("infer end status=429 dur=%s err=%v", time.Since(start), err) }
				return
			}
			if he, ok := err.(HTTPError); ok {
				http.Error(w, he.Error(), he.StatusCode())
				if lvl >= LevelInfo { log.Printf("infer end status=%d dur=%s err=%v", he.StatusCode(), time.Since(start), err) }
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			if lvl >= LevelInfo { log.Printf("infer end status=500 dur=%s err=%v", time.Since(start), err) }
			return
		}
		if lvl >= LevelInfo { log.Printf("infer end status=200 dur=%s", time.Since(start)) }
	})

	// Liveness probe - process is up
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Readiness probe - model loaded and ready to serve
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if svc.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("loading"))
	})

	return r
}
