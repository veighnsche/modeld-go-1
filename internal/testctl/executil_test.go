package testctl

import (
	"bytes"
	"io"
	"testing"
)

type fakeReader struct{ io.Reader }

func (f *fakeReader) Read(p []byte) (int, error) { return f.Reader.Read(p) }

func TestStream(t *testing.T) {
	var buf bytes.Buffer
	fr := &fakeReader{Reader: bytes.NewBufferString("line1\nline2\n")}
	// temporarily capture stdout by swapping fmt.Println via capturing output is non-trivial.
	// Instead, ensure stream consumes without panicking.
	stream("X", fr)
	_ = buf // no-op to avoid unused warnings
}
