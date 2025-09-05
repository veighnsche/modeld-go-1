package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "modeld",
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests",
		},
		[]string{"path", "method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "modeld",
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "Duration of HTTP requests in seconds",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"path", "method", "status"},
	)

	httpInflight = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "modeld",
			Subsystem: "http",
			Name:      "inflight_requests",
			Help:      "In-flight HTTP requests",
		},
		[]string{"path"},
	)

	backpressureTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "modeld",
			Subsystem: "http",
			Name:      "backpressure_total",
			Help:      "Total backpressure rejections (429)",
		},
		[]string{"reason"},
	)
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration, httpInflight, backpressureTotal)
}

// statusRecorder wraps http.ResponseWriter to capture status code
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// MetricsMiddleware instruments requests for Prometheus
func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := routePatternOrPath(r)
		method := r.Method
		httpInflight.WithLabelValues(path).Inc()
		defer httpInflight.WithLabelValues(path).Dec()

		sr := &statusRecorder{ResponseWriter: w, status: 200}
		start := time.Now()
		next.ServeHTTP(sr, r)
		statusLabel := itoa(sr.status)
		dur := time.Since(start).Seconds()
		httpRequestsTotal.WithLabelValues(path, method, statusLabel).Inc()
		httpRequestDuration.WithLabelValues(path, method, statusLabel).Observe(dur)
	})
}

// routePatternOrPath returns the chi route pattern if available, otherwise
// falls back to URL path. This avoids high-cardinality label values.
func routePatternOrPath(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return r.URL.Path
}

// IncrementBackpressure is called when returning 429 to the client
func IncrementBackpressure(reason string) {
	if reason == "" {
		reason = "unspecified"
	}
	backpressureTotal.WithLabelValues(reason).Inc()
}

// fast integer to ascii for small set of status codes
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
