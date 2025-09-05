package manager

import "sync"

// MemoryPublisher stores events in-memory for tests.
type MemoryPublisher struct {
	mu     sync.Mutex
	events []Event
}

func NewMemoryPublisher() *MemoryPublisher { return &MemoryPublisher{} }

func (p *MemoryPublisher) Publish(e Event) {
	p.mu.Lock()
	p.events = append(p.events, e)
	p.mu.Unlock()
}

func (p *MemoryPublisher) Events() []Event {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]Event, len(p.events))
	copy(out, p.events)
	return out
}
