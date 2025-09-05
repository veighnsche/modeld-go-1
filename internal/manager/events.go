package manager

// Event represents a manager lifecycle event.
// Minimal and stable: name + model ID and optional fields via key/values.
type Event struct {
	Name    string
	ModelID string
	Fields  map[string]any
}

// EventPublisher receives events from the manager. Implementations should be
// lightweight and non-blocking; Publish must not panic.
type EventPublisher interface {
	Publish(Event)
}

// noopPublisher is the default; it drops events.
type noopPublisher struct{}

func (noopPublisher) Publish(Event) {}
