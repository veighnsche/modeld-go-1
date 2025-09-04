package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPathExists(t *testing.T) {
	d := t.TempDir()
	if !PathExists(d) { t.Fatalf("expected temp dir to exist") }
	p := filepath.Join(d, "x")
	if PathExists(p) { t.Fatalf("expected non-existent path to be false") }
	if err := os.WriteFile(p, []byte("hi"), 0o644); err != nil { t.Fatalf("write: %v", err) }
	if !PathExists(p) { t.Fatalf("expected file to exist") }
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir available")
	}
	// raw path unaffected
	if got, err := ExpandHome("/tmp"); err != nil || got != "/tmp" { t.Fatalf("got %q err=%v", got, err) }
	// empty path
	if got, err := ExpandHome(""); err != nil || got != "" { t.Fatalf("got %q err=%v", got, err) }
	// ~ expansion
	p, err := ExpandHome("~")
	if err != nil { t.Fatalf("err: %v", err) }
	if p != home { t.Fatalf("expected %q, got %q", home, p) }
	// ~/subdir
	sub := "test-sub"
	exp, err := ExpandHome("~/" + sub)
	if err != nil { t.Fatalf("err: %v", err) }
	if runtime.GOOS == "windows" {
		if filepath.Base(exp) != sub { t.Fatalf("unexpected expanded path: %q", exp) }
	} else {
		expected := filepath.Join(home, sub)
		if exp != expected { t.Fatalf("expected %q, got %q", expected, exp) }
	}
}
