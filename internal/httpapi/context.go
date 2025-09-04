package httpapi

import (
	"context"
)

// serverBaseCtx is a process-level context that can be canceled on shutdown.
// Defaults to Background if not set.
var serverBaseCtx = context.Background()

// SetBaseContext sets the process-level base context used by handlers.
func SetBaseContext(ctx context.Context) {
	if ctx == nil {
		serverBaseCtx = context.Background()
		return
	}
	serverBaseCtx = ctx
}

// joinContexts returns a context that is canceled when either a or b is done.
// The returned cancel func must be called to release the goroutine when handler ends.
func joinContexts(a, b context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-a.Done():
			cancel()
		case <-b.Done():
			cancel()
		}
	}()
	return ctx, cancel
}
