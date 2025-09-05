package manager

import (
	"errors"
	"testing"
)

type errWriter2 struct{ err error }

func (e errWriter2) Write(p []byte) (int, error) { return 0, e.err }

type shortWriter2 struct{}

func (shortWriter2) Write(p []byte) (int, error) { return 0, nil }

func TestWriteAll_ReturnsUnderlyingError(t *testing.T) {
	w := errWriter2{err: errors.New("boom")}
	if err := writeAll(w, []byte("abc")); err == nil {
		t.Fatalf("expected error from writeAll")
	}
}

func TestWriteAll_ShortWriteError(t *testing.T) {
	w := shortWriter2{}
	if err := writeAll(w, []byte("abc")); err == nil {
		t.Fatalf("expected short write error from writeAll")
	}
}

func TestIsTooBusy(t *testing.T) {
	if !IsTooBusy(tooBusyError{modelID: "m"}) {
		t.Fatalf("expected IsTooBusy true")
	}
	if IsTooBusy(errors.New("other")) {
		t.Fatalf("expected IsTooBusy false for generic error")
	}
}
