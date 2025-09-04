package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandHome(t *testing.T) {
	// Determine home dir; if unavailable in env, skip
	home, err := os.UserHomeDir()
	if err != nil { t.Skip("no home dir available") }
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
