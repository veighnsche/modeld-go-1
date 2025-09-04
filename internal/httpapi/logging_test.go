package httpapi

import (
	"bytes"
	"log"
	"net/http/httptest"
	"testing"
	"strings"
)

func TestParseLevel(t *testing.T) {
	cases := map[string]LogLevel{
		"":      LevelOff,
		"off":   LevelOff,
		"error": LevelError,
		"info":  LevelInfo,
		"debug": LevelDebug,
		"weird": LevelInfo, // default
	}
	for in, want := range cases {
		if got := parseLevel(in); got != want {
			t.Fatalf("parseLevel(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestRequestLogLevel_Overrides(t *testing.T) {
	// query param ?log=debug
	r := httptest.NewRequest("GET", "/x?log=debug", nil)
	if got := requestLogLevel(r); got != LevelDebug {
		t.Fatalf("query override failed: %v", got)
	}
	// legacy query param ?log=1
	r = httptest.NewRequest("GET", "/x?log=1", nil)
	if got := requestLogLevel(r); got != LevelDebug {
		t.Fatalf("legacy query override failed: %v", got)
	}
	// header X-Log-Level
	r = httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("X-Log-Level", "error")
	if got := requestLogLevel(r); got != LevelError {
		t.Fatalf("header override failed: %v", got)
	}
	// legacy header X-Log-Infer
	r = httptest.NewRequest("GET", "/x", nil)
	r.Header.Set("X-Log-Infer", "1")
	if got := requestLogLevel(r); got != LevelDebug {
		t.Fatalf("legacy header override failed: %v", got)
	}
}

func TestLoggingLineWriter_SplitsLines(t *testing.T) {
	var buf bytes.Buffer
	orig := log.Writer()
	defer log.SetOutput(orig)
	log.SetOutput(&buf)

	lw := &loggingLineWriter{}
	_, _ = lw.Write([]byte("a line\npartial"))
	_, _ = lw.Write([]byte("-cont\nlast\n"))

	out := buf.String()
	if !strings.Contains(out, "infer> a line") {
		t.Fatalf("missing logged line: %q", out)
	}
	if !strings.Contains(out, "infer> partial-cont") {
		t.Fatalf("missing joined line: %q", out)
	}
	if !strings.Contains(out, "infer> last") {
		t.Fatalf("missing last line: %q", out)
	}
}
