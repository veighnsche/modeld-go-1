package manager

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// createModelFile creates a file of approximately sizeMB megabytes and returns its path.
func createModelFile(t *testing.T, dir, name string, sizeMB int) string {
	t.Helper()
	if sizeMB <= 0 {
		sizeMB = 1
	}
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	defer f.Close()
	// write sizeMB megabytes (use 1MiB blocks)
	block := make([]byte, 1024*1024)
	for i := 0; i < sizeMB; i++ {
		if _, err := f.Write(block); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	if err := f.Sync(); err != nil {
		t.Fatalf("sync: %v", err)
	}
	return p
}

// fakeAdapter is a lightweight in-memory adapter used for tests.
type fakeAdapter struct {
	startErr   error
	genErr     error
	tokens     []string
	final      FinalResult
	receivedMP string
}

func (f *fakeAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	f.receivedMP = modelPath
	if f.startErr != nil {
		return nil, f.startErr
	}
	return fakeSession{f: f}, nil
}

type fakeSession struct{ f *fakeAdapter }

func (s fakeSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	if s.f.genErr != nil {
		return FinalResult{}, s.f.genErr
	}
	for _, t := range s.f.tokens {
		select {
		case <-ctx.Done():
			return FinalResult{}, ctx.Err()
		default:
		}
		if err := onToken(t); err != nil {
			return FinalResult{}, err
		}
	}
	return s.f.final, nil
}

func (s fakeSession) Close() error { return nil }

// errWriter writes once, then returns an error on subsequent writes.
type errWriter struct{ wrote int }

func (e *errWriter) Write(p []byte) (int, error) {
	if e.wrote == 0 {
		e.wrote += len(p)
		return len(p), nil
	}
	return 0, errors.New("write fail")
}

// testCtx returns a context with a short timeout, canceled on test cleanup.
func testCtx(t *testing.T) context.Context {
    t.Helper()
    c, cancel := context.WithTimeout(context.Background(), 2*time.Second)
    t.Cleanup(cancel)
    return c
}
