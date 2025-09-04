package manager

// tooBusyError signals queue timeout/overflow for 429 mapping.
type tooBusyError struct{ modelID string }
func (e tooBusyError) Error() string { return "too busy: " + e.modelID }

// IsTooBusy reports whether err indicates backpressure (return 429).
func IsTooBusy(err error) bool {
    _, ok := err.(tooBusyError)
    return ok
}

// ErrModelNotFound returns an error when a requested model id is not present in the registry.
type modelNotFoundError struct{ id string }

func (e modelNotFoundError) Error() string { return "model not found: " + e.id }

func ErrModelNotFound(id string) error { return modelNotFoundError{id: id} }

// IsModelNotFound reports whether the error indicates a missing model id.
func IsModelNotFound(err error) bool {
    _, ok := err.(modelNotFoundError)
    return ok
}
