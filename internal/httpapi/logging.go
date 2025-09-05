package httpapi

import (
	"bytes"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog"
)

// zlog is an optional structured logger. If unset, falls back to log.Printf.
var zlog *zerolog.Logger

// SetLogger installs a structured logger used by the HTTP layer.
func SetLogger(l zerolog.Logger) { zlog = &l }

// loggingLineWriter logs complete NDJSON lines to the standard logger.
type loggingLineWriter struct {
	buf []byte
}

func (lw *loggingLineWriter) Write(p []byte) (int, error) {
	lw.buf = append(lw.buf, p...)
	for {
		idx := bytes.IndexByte(lw.buf, '\n')
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

// LogLevel controls per-request logging behavior.
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

// logInferStart emits a standardized "infer start" message when LevelInfo or higher.
func logInferStart(lvl LogLevel, r *http.Request, model string) {
	if lvl < LevelInfo {
		return
	}
	if zlog != nil {
		z := zlog.Info().Str("path", r.URL.Path).Str("model", model)
		if rid := middleware.GetReqID(r.Context()); rid != "" {
			z = z.Str("request_id", rid)
		}
		z.Msg("infer start")
		return
	}
	log.Printf("infer start path=%s model=%s", r.URL.Path, model)
}

// logInferEnd emits a standardized "infer end" message with status and optional error.
func logInferEnd(lvl LogLevel, start time.Time, r *http.Request, status string, err error) {
	if lvl < LevelInfo {
		return
	}
	if zlog != nil {
		z := zlog.Info().Str("status", status).Dur("dur", time.Since(start))
		if rid := middleware.GetReqID(r.Context()); rid != "" {
			z = z.Str("request_id", rid)
		}
		if err != nil {
			z = z.Err(err)
		}
		z.Msg("infer end")
		return
	}
	if err != nil {
		log.Printf("infer end status=%s dur=%s err=%v", status, time.Since(start), err)
	} else {
		log.Printf("infer end status=%s dur=%s", status, time.Since(start))
	}
}
