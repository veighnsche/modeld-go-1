package httpapi

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
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

func NewMux(svc Service) http.Handler {
	r := chi.NewRouter()
	// Basic middlewares: request id, real ip, recoverer
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	// Metrics instrumentation
	r.Use(MetricsMiddleware)
	// Compression for JSON endpoints
	r.Use(middleware.Compress(5))
	// Security headers
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})
	// Optional CORS
	if corsEnabled {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   corsAllowedOrigins,
			AllowedMethods:   corsAllowedMethods,
			AllowedHeaders:   corsAllowedHeaders,
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	// Register routes
	r.Get("/models", getModels(svc))

	r.Get("/status", getStatus(svc))

	r.Post("/infer", postInfer(svc))

	r.Get("/healthz", getHealthz())

	r.Get("/readyz", getReadyz(svc))

	// Prometheus metrics endpoint
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	// Optionally mount Swagger UI and docs (no-op unless built with -tags=swagger)
	MountSwagger(r)

	return r
}

// writeJSONError is implemented in errors.go

// getModels lists available models.
// @Summary List models
// @Description Returns available models discovered by the registry.
// @Tags models
// @Produce json
// @Success 200 {object} types.ModelsResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /models [get]
func getModels(svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := types.ModelsResponse{Models: svc.ListModels()}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}
	}
}

// getStatus returns server status.
// @Summary Server status
// @Description Returns runtime status of loaded instances and resource usage.
// @Tags status
// @Produce json
// @Success 200 {object} types.StatusResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /status [get]
func getStatus(svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(svc.Status()); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "failed to encode response")
			return
		}
	}
}

// postInfer performs inference and streams NDJSON tokens.
// @Summary Inference
// @Description Streams inference results as NDJSON lines. Each line is a JSON object.
// @Tags infer
// @Accept json
// @Produce application/x-ndjson
// @Param request body types.InferRequest true "Inference request"
// @Param log query string false "Optional per-request log level override: off|error|info|debug"
// @Param X-Log-Level header string false "Optional per-request log level override: off|error|info|debug"
// @Param X-Log-Infer header string false "Legacy flag; when '1' enables token debug logging"
// @Success 200 {string} string "NDJSON stream"
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 415 {object} types.ErrorResponse
// @Failure 429 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /infer [post]
func postInfer(svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
				if rid := middleware.GetReqID(r.Context()); rid != "" {
					z = z.Str("request_id", rid)
				}
				z.Msg("infer start")
			} else {
				log.Printf("infer start path=%s model=%s", r.URL.Path, req.Model)
			}
		}
		// Join server base context with request context so shutdown cancels work too.
		joinedCtx, cancel := joinContexts(serverBaseCtx, r.Context())
		// Apply optional per-handler timeout if configured
		if inferTimeout > 0 {
			var tctx context.Context
			tctx, cancel = context.WithTimeout(joinedCtx, time.Duration(inferTimeout)*time.Second)
			joinedCtx = tctx
		}
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
						if rid := middleware.GetReqID(r.Context()); rid != "" {
							z = z.Str("request_id", rid)
						}
						z.Err(err).Msg("infer end")
					} else {
						log.Printf("infer end status=404 dur=%s err=%v", time.Since(start), err)
					}
				}
				return
			}
			if manager.IsTooBusy(err) {
				writeJSONError(w, http.StatusTooManyRequests, err.Error())
				IncrementBackpressure("queue")
				if lvl >= LevelInfo {
					if zlog != nil {
						z := zlog.Info().Str("status", "429").Dur("dur", time.Since(start))
						if rid := middleware.GetReqID(r.Context()); rid != "" {
							z = z.Str("request_id", rid)
						}
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
						if rid := middleware.GetReqID(r.Context()); rid != "" {
							z = z.Str("request_id", rid)
						}
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
					if rid := middleware.GetReqID(r.Context()); rid != "" {
						z = z.Str("request_id", rid)
					}
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
				if rid := middleware.GetReqID(r.Context()); rid != "" {
					z = z.Str("request_id", rid)
				}
				z.Msg("infer end")
			} else {
				log.Printf("infer end status=200 dur=%s", time.Since(start))
			}
		}
	}
}

// getHealthz returns OK for liveness checks.
// @Summary Health check
// @Tags health
// @Produce text/plain
// @Success 200 {string} string "ok"
// @Router /healthz [get]
func getHealthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}
}

// getReadyz returns readiness status.
// @Summary Readiness check
// @Tags health
// @Produce text/plain
// @Success 200 {string} string "ready"
// @Failure 503 {string} string "loading"
// @Router /readyz [get]
func getReadyz(svc Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if svc.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("loading"))
	}
}
