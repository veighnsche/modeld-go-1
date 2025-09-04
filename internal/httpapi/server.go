package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"modeld/internal/manager"
	"modeld/pkg/types"
)

func NewMux(mgr *manager.Manager) http.Handler {
	r := chi.NewRouter()
	// TODO: add middlewares (request id, recoverer, CORS, gzip) later

	r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
		// TODO: read from config/registry
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"models": mgr.ListModels()}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(mgr.Status()); err != nil {
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
		if err := mgr.Infer(r.Context(), req, w, flush); err != nil {
			// If context was canceled (client disconnect), just return.
			if r.Context().Err() != nil {
				return
			}
			// Map queue/backpressure to 429
			if manager.IsTooBusy(err) {
				http.Error(w, err.Error(), http.StatusTooManyRequests)
				return
			}
			// Map model-not-found to 404
			if manager.IsModelNotFound(err) {
				http.Error(w, err.Error(), http.StatusNotFound)
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
		if mgr.Ready() {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("ready"))
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("loading"))
	})

	return r
}
