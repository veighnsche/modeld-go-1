package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// TestMetricsMiddleware_EmitsRequestCounters verifies that wrapping a handler
// with MetricsMiddleware results in request metrics being exposed via the
// Prometheus /metrics handler.
func TestMetricsMiddleware_EmitsRequestCounters(t *testing.T) {
	// A trivial handler that returns 200 OK
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// Wrap with metrics middleware and perform a request
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	MetricsMiddleware(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	// Scrape the default registry and ensure our metric name is present
	mrr := httptest.NewRecorder()
	mreq := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	promhttp.Handler().ServeHTTP(mrr, mreq)
	if mrr.Code != http.StatusOK {
		t.Fatalf("/metrics status=%d", mrr.Code)
	}
	body := mrr.Body.Bytes()
	if !bytes.Contains(body, []byte("modeld_http_requests_total")) {
		// clip body preview to avoid large logs without relying on a min() helper
		previewLen := len(body)
		if previewLen > 200 {
			previewLen = 200
		}
		t.Fatalf("expected to find modeld_http_requests_total in metrics; got: %q", string(body[:previewLen]))
	}
}
