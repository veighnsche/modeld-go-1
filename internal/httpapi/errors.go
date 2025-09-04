package httpapi

import (
	"encoding/json"
	"net/http"
)

// HTTPError allows services to provide an HTTP status code for an error.
type HTTPError interface {
	error
	StatusCode() int
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
