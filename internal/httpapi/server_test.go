package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"modeld/pkg/types"
)

type mockService struct{
	models []types.Model
	status types.StatusResponse
	ready bool
	inferErr error
}

func (m *mockService) ListModels() []types.Model { return append([]types.Model(nil), m.models...) }
func (m *mockService) Status() types.StatusResponse { return m.status }
func (m *mockService) Ready() bool { return m.ready }
func (m *mockService) Infer(ctx context.Context, req types.InferRequest, w io.Writer, flush func()) error {
	// Write two NDJSON lines if no error
	if m.inferErr != nil { return m.inferErr }
	enc := json.NewEncoder(w)
	_ = enc.Encode(map[string]any{"delta":"hi"})
	if flush != nil { flush() }
	_ = enc.Encode(map[string]any{"done":true})
	if flush != nil { flush() }
	return nil
}

type mockHTTPError struct{ msg string; code int }
func (e mockHTTPError) Error() string { return e.msg }
func (e mockHTTPError) StatusCode() int { return e.code }

func TestModelsHandler(t *testing.T) {
	svc := &mockService{models: []types.Model{{ID: "m1"}, {ID: "m2"}}}
	r := NewMux(svc)
	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK { t.Fatalf("status=%d", w.Code) }
	if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") { t.Fatalf("content-type=%s", ct) }
	var body map[string][]types.Model
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil { t.Fatalf("json: %v", err) }
	if len(body["models"]) != 2 { t.Fatalf("models len=%d", len(body["models"])) }
}

func TestStatusHandler(t *testing.T) {
	svc := &mockService{status: types.StatusResponse{BudgetMB: 10}}
	r := NewMux(svc)
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK { t.Fatalf("status=%d", w.Code) }
	var body types.StatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil { t.Fatalf("json: %v", err) }
	if body.BudgetMB != 10 { t.Fatalf("unexpected body: %+v", body) }
}

func TestReadyz(t *testing.T) {
	svc := &mockService{ready: true}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusOK { t.Fatalf("status=%d", w.Code) }
}

func TestReadyz_NotReady(t *testing.T) {
	svc := &mockService{ready: false}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if w.Code != http.StatusServiceUnavailable { t.Fatalf("status=%d", w.Code) }
	if !strings.Contains(w.Body.String(), "loading") { t.Fatalf("body=%q", w.Body.String()) }
}

func TestInferStreams(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK { t.Fatalf("status=%d body=%s", w.Code, w.Body.String()) }
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 2 { t.Fatalf("expected 2 ndjson lines, got %d", len(lines)) }
}

func TestInferBadJSON(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString("not-json"))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("status=%d", w.Code) }
}

func TestInferHTTPErrorMapping(t *testing.T) {
	svc := &mockService{inferErr: mockHTTPError{msg: "too busy", code: http.StatusTooManyRequests}}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests { t.Fatalf("status=%d", w.Code) }
}

func TestInferGenericErrorMaps500(t *testing.T) {
	svc := &mockService{inferErr: io.EOF}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError { t.Fatalf("status=%d", w.Code) }
}

func TestInferUnsupportedMediaType(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "text/plain")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnsupportedMediaType { t.Fatalf("status=%d", w.Code) }
}

func TestInferBodyTooLarge(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	// Create >1MiB body
	big := make([]byte, (1<<20)+10)
	for i := range big { big[i] = 'a' }
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewReader(big))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400 for too-large body, got %d", w.Code) }
}

func TestInferPromptRequired(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"   "}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest { t.Fatalf("expected 400 for missing prompt, got %d", w.Code) }
}

func TestHealthz(t *testing.T) {
	svc := &mockService{}
	r := NewMux(svc)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if w.Code != http.StatusOK { t.Fatalf("status=%d", w.Code) }
}
