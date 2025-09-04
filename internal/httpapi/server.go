package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

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
		if err := json.NewEncoder(w).Encode(map[string]any{"models": []any{}}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		s := mgr.Snapshot()
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"state":         s.State,
			"current_model": s.CurrentModel,
			"error":         s.Err,
		}); err != nil {
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

		// Ensure model as per rules: if empty -> no change; if same -> no change; if diff -> switch
		if err := mgr.EnsureModel(r.Context(), req.Model); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Simple NDJSON streaming placeholder
		w.Header().Set("Content-Type", "application/x-ndjson")
		flusher, _ := w.(http.Flusher)
		// Emit a few fake tokens quickly to illustrate streaming
		chunks := []string{"{\"token\":\"Hello\"}", "{\"token\":\",\"}", "{\"token\":\" world\"}", "{\"done\":true}"}
		for i, ch := range chunks {
			if _, err := w.Write([]byte(ch + "\n")); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
			if i < len(chunks)-1 {
				time.Sleep(10 * time.Millisecond)
			}
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
