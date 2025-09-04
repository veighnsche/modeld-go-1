package httpapi

import (
	"encoding/json"
	"net/http"
	"context"
	"io"

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
		if err := svc.Infer(r.Context(), req, w, flush); err != nil {
			// If context was canceled (client disconnect), just return.
			if r.Context().Err() != nil {
				return
			}
			// Map well-known manager errors to HTTP status codes
			if manager.IsModelNotFound(err) {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			if manager.IsTooBusy(err) {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}
			if he, ok := err.(HTTPError); ok {
				http.Error(w, he.Error(), he.StatusCode())
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
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
