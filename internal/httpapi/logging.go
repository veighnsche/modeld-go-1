package httpapi

import (
	"log"
	"net/http"
	"os"
	
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
