package httpapi

// maxBodyBytes controls the maximum allowed request body size for JSON endpoints.
// Default remains 1 MiB for backward compatibility.
var maxBodyBytes int64 = 1 << 20

// SetMaxBodyBytes allows configuring the maximum request body size.
func SetMaxBodyBytes(n int64) {
	if n <= 0 {
		maxBodyBytes = 1 << 20
		return
	}
	maxBodyBytes = n
}

// inferTimeout controls the maximum duration an /infer request may run before timing out.
// Zero means no additional timeout beyond server/connection timeouts.
var inferTimeout = int64(0) // seconds

// SetInferTimeoutSeconds sets the infer timeout in seconds (0 disables).
func SetInferTimeoutSeconds(sec int64) {
	if sec < 0 {
		sec = 0
	}
	inferTimeout = sec
}

// CORS configuration (opt-in). If disabled, no CORS middleware is added.
var (
	corsEnabled        bool
	corsAllowedOrigins []string
	corsAllowedMethods []string
	corsAllowedHeaders []string
)

// SetCORSOptions configures CORS behavior for the HTTP server.
func SetCORSOptions(enabled bool, origins, methods, headers []string) {
	corsEnabled = enabled
	corsAllowedOrigins = append([]string(nil), origins...)
	corsAllowedMethods = append([]string(nil), methods...)
	corsAllowedHeaders = append([]string(nil), headers...)
}
