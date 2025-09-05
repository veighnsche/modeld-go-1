package httpapi

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestIncrementBackpressure_IncrementsCounter(t *testing.T) {
	// Ensure metrics are registered (init() already does this)
	// Read baseline value for reason="queue"
	baseline := testutil.ToFloat64(backpressureTotal.WithLabelValues("queue"))
	// Increment twice
	IncrementBackpressure("queue")
	IncrementBackpressure("queue")
	// Verify incremented by 2
	got := testutil.ToFloat64(backpressureTotal.WithLabelValues("queue"))
	if got < baseline+2 {
		t.Fatalf("expected backpressure counter >= %v, got %v", baseline+2, got)
	}

	// Empty reason should default to "unspecified"
	before := testutil.ToFloat64(backpressureTotal.WithLabelValues("unspecified"))
	IncrementBackpressure("")
	after := testutil.ToFloat64(backpressureTotal.WithLabelValues("unspecified"))
	if after < before+1 {
		t.Fatalf("expected unspecified reason to increment by at least 1: before=%v after=%v", before, after)
	}

	// Touch other vectors to avoid stale code paths
	_ = prometheus.NewCounter(prometheus.CounterOpts{Name: "noop", Help: "noop"})
}
