package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"modeld/internal/manager"
)

func TestInfer_ModelNotFoundMaps404(t *testing.T) {
	svc := &mockService{inferErr: manager.ErrModelNotFound("m-missing")}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestInfer_DependencyUnavailableMaps503(t *testing.T) {
	svc := &mockService{inferErr: manager.ErrDependencyUnavailable("llama adapter not initialized")}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
