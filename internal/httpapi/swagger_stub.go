//go:build !swagger

package httpapi

import (
    "github.com/go-chi/chi/v5"
)

// MountSwagger is a no-op by default. Build with -tags=swagger to enable.
func MountSwagger(r chi.Router) {}
