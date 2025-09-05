package manager

import "testing"

func TestPickFreePort_ReturnsPositivePort(t *testing.T) {
	p, err := pickFreePort("127.0.0.1")
	if err != nil || p <= 0 {
		t.Fatalf("pickFreePort error=%v port=%d", err, p)
	}
}

func TestStopAllInstances_NoAdapterDoesNotPanic(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	// ensure no adapter
	m.SetInferenceAdapter(nil)
	m.StopAllInstances()
}
