package httpapi

import (
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestMountSwagger_NoOp(t *testing.T) {
	r := chi.NewRouter()
	// Should be a no-op and not panic
	MountSwagger(r)
}
