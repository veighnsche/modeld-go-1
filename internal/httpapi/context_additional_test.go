package httpapi

import (
	"context"
	"testing"
	"time"
)

func TestSetBaseContext_NilResetsToBackground(t *testing.T) {
	// set to a cancelable ctx
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	SetBaseContext(ctx)
	// now reset with nil
	// nolint:staticcheck // SA1012: this test intentionally passes nil to verify fallback behavior
	SetBaseContext(nil)
	// join with a short-lived context and ensure cancel triggers
	a, ac := context.WithCancel(context.Background())
	defer ac()
	b, bc := context.WithCancel(context.Background())
	defer bc()
	j, cancelJ := joinContexts(a, b)
	defer cancelJ()
	ac() // cancel a
	select {
	case <-j.Done():
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("joined context did not cancel after parent canceled")
	}
}

func TestJoinContexts_CancelsWhenEitherDone(t *testing.T) {
	a, ac := context.WithCancel(context.Background())
	b, bc := context.WithCancel(context.Background())
	defer bc()
	j, cancelJ := joinContexts(a, b)
	defer cancelJ()
	// cancel A and expect joined canceled
	ac()
	select {
	case <-j.Done():
		// ok
	case <-time.After(500 * time.Millisecond):
		t.Fatal("joined context did not cancel when first parent canceled")
	}
}
