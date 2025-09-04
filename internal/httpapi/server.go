package httpapi

import (
	"encoding/json"
	"net/http"
	"context"
	"io"
	"log"
	"os"
	"time"
	"strings"
	
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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

// joinContexts returns a context that is canceled when either a or b is done.
// The returned cancel func must be called to release the goroutine when handler ends.
func joinContexts(a, b context.Context) (context.Context, context.CancelFunc) {
    ctx, cancel := context.WithCancel(context.Background())
    go func() {
        select {
        case <-a.Done():
            cancel()
        case <-b.Done():
            cancel()
        }
    }()
    return ctx, cancel
}

// serverBaseCtx is a process-level context that can be canceled on shutdown.
// Defaults to Background if not set.
var serverBaseCtx = context.Background()

// SetBaseContext sets the process-level base context used by handlers.
func SetBaseContext(ctx context.Context) {
    if ctx == nil {
        serverBaseCtx = context.Background()
        return
    }
    serverBaseCtx = ctx
}

// maxBodyBytes controls the maximum allowed request body size for JSON endpoints.
// Default remains 1 MiB for backward compatibility.
var maxBodyBytes int64 = 1 << 20

// SetMaxBodyBytes allows configuring the maximum request body size.
func SetMaxBodyBytes(n int64) {
    if n <= 0 {
        maxBodyBytes = 1 << 20
        return
    }
    maxBodyBytes = n
}

// zlog is an optional structured logger. If unset, falls back to log.Printf.
var zlog *zerolog.Logger

// SetLogger installs a structured logger used by the HTTP layer.
func SetLogger(l zerolog.Logger) { zlog = &l }

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
	// Basic middlewares: request id, real ip, recoverer
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// Compression for JSON endpoints
	r.Use(middleware.Compress(5))
	// Security headers
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})

	r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
		// TODO: read from config/registry
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"models": svc.ListModels()}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(svc.Status()); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}
	})

	r.Post("/infer", func(w http.ResponseWriter, r *http.Request) {
		// Content-Type check
		ct := r.Header.Get("Content-Type")
		if ct == "" || !strings.HasPrefix(strings.ToLower(ct), "application/json") {
			writeJSONError(w, http.StatusUnsupportedMediaType, "Content-Type must be application/json")
			return
		}
		// Limit body size (configurable, default 1MiB)
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		var req types.InferRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			// If exceeded size, MaxBytesReader may cause an error; still return 400 to avoid size leak details
			writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		// Basic validation
		if strings.TrimSpace(req.Prompt) == "" {
			writeJSONError(w, http.StatusBadRequest, "prompt is required")
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
			if zlog != nil {
				z := zlog.Info().Str("path", r.URL.Path).Str("model", req.Model)
				if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
				z.Msg("infer start")
			} else {
				log.Printf("infer start path=%s model=%s", r.URL.Path, req.Model)
			}
		}
		// Join server base context with request context so shutdown cancels work too.
		joinedCtx, cancel := joinContexts(serverBaseCtx, r.Context())
		defer cancel()
		if err := svc.Infer(joinedCtx, req, writer, flush); err != nil {
			// If context was canceled (client disconnect), just return.
			if r.Context().Err() != nil || serverBaseCtx.Err() != nil {
				return
			}
			// Map well-known manager errors to HTTP status codes
			if manager.IsModelNotFound(err) {
				writeJSONError(w, http.StatusNotFound, err.Error())
				if lvl >= LevelInfo {
					if zlog != nil {
						z := zlog.Info().Str("status", "404").Dur("dur", time.Since(start))
						if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
						z.Err(err).Msg("infer end")
					} else {
						log.Printf("infer end status=404 dur=%s err=%v", time.Since(start), err)
					}
				}
				return
			}
			if manager.IsTooBusy(err) {
				writeJSONError(w, http.StatusTooManyRequests, err.Error())
				if lvl >= LevelInfo {
					if zlog != nil {
						z := zlog.Info().Str("status", "429").Dur("dur", time.Since(start))
						if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
						z.Err(err).Msg("infer end")
					} else {
						log.Printf("infer end status=429 dur=%s err=%v", time.Since(start), err)
					}
				}
				return
			}
			if he, ok := err.(HTTPError); ok {
				writeJSONError(w, he.StatusCode(), he.Error())
				if lvl >= LevelInfo {
					if zlog != nil {
						z := zlog.Info().Int("status", he.StatusCode()).Dur("dur", time.Since(start))
						if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
						z.Err(err).Msg("infer end")
					} else {
						log.Printf("infer end status=%d dur=%s err=%v", he.StatusCode(), time.Since(start), err)
					}
				}
				return
			}
			writeJSONError(w, http.StatusInternalServerError, err.Error())
			if lvl >= LevelInfo {
				if zlog != nil {
					z := zlog.Info().Str("status", "500").Dur("dur", time.Since(start))
					if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
					z.Err(err).Msg("infer end")
				} else {
					log.Printf("infer end status=500 dur=%s err=%v", time.Since(start), err)
				}
			}
			return
		}
		if lvl >= LevelInfo {
			if zlog != nil {
				z := zlog.Info().Str("status", "200").Dur("dur", time.Since(start))
				if rid := middleware.GetReqID(r.Context()); rid != "" { z = z.Str("request_id", rid) }
				z.Msg("infer end")
			} else {
				log.Printf("infer end status=200 dur=%s", time.Since(start))
			}
		}
	})

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if svc.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("loading"))
	})

	// Prometheus metrics endpoint
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	return r
}

// writeJSONError writes a consistent JSON error payload.
func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": msg,
		"code":  status,
	})
}
