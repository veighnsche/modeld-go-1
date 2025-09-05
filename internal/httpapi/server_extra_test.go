package httpapi

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rs/zerolog"
	"modeld/internal/manager"
	"modeld/pkg/types"
)

// New focused tests

// Service that blocks until the context is done; used to exercise timeout path.
type blockService struct{}

func TestInferLogsWithZerologInfo(t *testing.T) {
	// Install a zerolog logger to exercise the zlog != nil branches
	SetLogger(zerolog.New(io.Discard))
	defer SetLogger(zerolog.Logger{})

	svc := &mockService{}
	h := NewMux(svc)
	req := httptest.NewRequest(http.MethodPost, "/infer?log=info", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with info logging, got %d", rec.Code)
	}
}

func (b *blockService) ListModels() []types.Model    { return nil }
func (b *blockService) Status() types.StatusResponse { return types.StatusResponse{} }
func (b *blockService) Ready() bool                  { return true }
func (b *blockService) Infer(ctx context.Context, req types.InferRequest, w io.Writer, flush func()) error {
	<-ctx.Done()
	return ctx.Err()
}

func TestCORSAndSecurityHeaders(t *testing.T) {
	// Enable CORS temporarily
	SetCORSOptions(true, []string{"*"}, []string{"GET", "POST", "OPTIONS"}, []string{"Content-Type"})
	defer SetCORSOptions(false, nil, nil, nil)

	// Use simple mock from server_test.go
	svc := &mockService{ready: true}
	h := NewMux(svc)
	req := httptest.NewRequest(http.MethodGet, "/models", nil)
	req.Header.Set("Origin", "http://example.com")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Fatalf("expected X-Content-Type-Options=nosniff, got %q", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got == "" {
		t.Fatalf("expected CORS header Access-Control-Allow-Origin to be set, got empty")
	}
}

func TestInferTimeoutReturns500(t *testing.T) {
	defer SetInferTimeoutSeconds(0)
	SetInferTimeoutSeconds(1)

	// Service that blocks until context is canceled
	svc := &blockService{}
	h := NewMux(svc)
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 on timeout, got %d", rec.Code)
	}
}

func TestInferModelNotFound404(t *testing.T) {
	svc := &mockService{inferErr: manager.ErrModelNotFound("abc")}
	h := NewMux(svc)
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for model not found, got %d", rec.Code)
	}
}

func TestContentTypeCaseInsensitive(t *testing.T) {
	svc := &mockService{}
	h := NewMux(svc)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/infer", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "Application/JSON; charset=utf-8")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with mixed-case content-type, got %d", rec.Code)
	}
}

func TestInferStreamsWithDebugLogging(t *testing.T) {
	svc := &mockService{}
	h := NewMux(svc)
	req := httptest.NewRequest(http.MethodPost, "/infer?log=debug", bytes.NewBufferString(`{"prompt":"hi"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 with debug logging, got %d", rec.Code)
	}
	// requestLogLevel path LevelDebug exercises loggingLineWriter attachment; functional assertion done in logging_test.go
}
