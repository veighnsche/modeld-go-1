package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TestMetricsMiddleware_UsesRoutePattern ensures the metrics middleware labels
// by the chi route pattern instead of the raw URL path.
func TestMetricsMiddleware_UsesRoutePattern(t *testing.T) {
	r := chi.NewRouter()
	// Register a concrete route so chi can attach a pattern
	r.Post("/infer", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Wrap the router with our metrics middleware
	h := MetricsMiddleware(r)

	req := httptest.NewRequest(http.MethodPost, "/infer", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}

	// Scrape /metrics and assert the path label equals the chi pattern
	mrr := httptest.NewRecorder()
	promhttp.Handler().ServeHTTP(mrr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if mrr.Code != http.StatusOK {
		t.Fatalf("/metrics status=%d", mrr.Code)
	}
	body := mrr.Body.Bytes()
	// Expect the path label to be "/infer"
	if !bytes.Contains(body, []byte(`modeld_http_requests_total{path="/infer",method="POST"`)) {
		preview := body
		if len(preview) > 400 {
			preview = preview[:400]
		}
		t.Fatalf("expected metrics to contain route pattern label path=\"/infer\"; got: %q", string(preview))
	}
}
