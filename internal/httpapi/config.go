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
