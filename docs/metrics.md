# Metrics

Prometheus metrics are exposed at `GET /metrics`.

Namespace: `modeld`, subsystem: `http`.

Current metrics:

- modeld_http_requests_total (counter)
  - Labels: `path`, `method`, `status`
- modeld_http_request_duration_seconds (histogram)
  - Labels: `path`, `method`, `status`
- modeld_http_inflight_requests (gauge)
  - Labels: `path`
- modeld_http_backpressure_total (counter)
  - Labels: `reason` (e.g., `queue_full`, `wait_timeout`)

Notes:
- The middleware instruments all HTTP handlers. Path labels currently use the request path string. Consider mapping to stable route names to reduce cardinality in high-variance environments.
