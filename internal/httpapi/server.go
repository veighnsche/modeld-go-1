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
json.NewEncoder(w).Encode(map[string]any{"models": []any{}})
})

r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
s := mgr.Snapshot()
json.NewEncoder(w).Encode(map[string]any{
"state":         s.State,
"current_model": s.CurrentModel,
"error":         s.Err,
})
})

r.Post("/switch", func(w http.ResponseWriter, r *http.Request) {
var req struct{ ModelID string `json:"model_id"` }
_ = json.NewDecoder(r.Body).Decode(&req)
op, _ := mgr.Switch(r.Context(), req.ModelID)
w.WriteHeader(http.StatusAccepted)
json.NewEncoder(w).Encode(map[string]string{"op_id": op})
})

r.Get("/events", func(w http.ResponseWriter, r *http.Request) {
// TODO: SSE hub
w.Header().Set("Content-Type", "text/event-stream")
w.Write([]byte("event: message\ndata: {\"event\":\"snapshot\",\"data\":{\"state\":\"loading\"}}\n\n"))
})

r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
if mgr.Ready() { w.WriteHeader(200); w.Write([]byte("ok")); return }
w.WriteHeader(503); w.Write([]byte("loading"))
})

return r
}
