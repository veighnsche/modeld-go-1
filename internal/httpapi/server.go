package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"modeld/internal/manager"
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

	r.Post("/switch", func(w http.ResponseWriter, r *http.Request) {
		var req struct{ ModelID string `json:"model_id"` }
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON body", http.StatusBadRequest)
			return
		}
		op, err := mgr.Switch(r.Context(), req.ModelID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusAccepted)
		if err := json.NewEncoder(w).Encode(map[string]string{"op_id": op}); err != nil {
			http.Error(w, "failed to encode response", http.StatusInternalServerError)
			return
		}
	})

	r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
		// TODO: SSE hub
		w.Header().Set("Content-Type", "text/event-stream")
		w.Write([]byte("event: message\ndata: {\"event\":\"snapshot\",\"data\":{\"state\":\"loading\"}}\n\n"))
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
