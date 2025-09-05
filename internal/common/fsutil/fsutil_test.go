package fsutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestExpandHome(t *testing.T) {
	// Set a deterministic HOME for the duration of this test so we never skip.
	origHome, hadHome := os.LookupEnv("HOME")
	origUserProfile, hadUserProfile := os.LookupEnv("USERPROFILE")
	t.Cleanup(func() {
		if hadHome {
			_ = os.Setenv("HOME", origHome)
		} else {
			_ = os.Unsetenv("HOME")
		}
		if hadUserProfile {
			_ = os.Setenv("USERPROFILE", origUserProfile)
		} else {
			_ = os.Unsetenv("USERPROFILE")
		}
	})

	home := t.TempDir()
	// Configure both env vars for cross-platform behavior of os.UserHomeDir.
	_ = os.Setenv("HOME", home)
	if runtime.GOOS == "windows" {
		_ = os.Setenv("USERPROFILE", home)
	}
	// raw path unaffected
	if got, err := ExpandHome("/tmp"); err != nil || got != "/tmp" {
		t.Fatalf("got %q err=%v", got, err)
	}
	// empty path
	if got, err := ExpandHome(""); err != nil || got != "" {
		t.Fatalf("got %q err=%v", got, err)
	}
	// ~ expansion
	p, err := ExpandHome("~")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if p != home {
		t.Fatalf("expected %q, got %q", home, p)
	}
	// ~/subdir
	sub := "test-sub"
	exp, err := ExpandHome("~/" + sub)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if runtime.GOOS == "windows" {
		if filepath.Base(exp) != sub {
			t.Fatalf("unexpected expanded path: %q", exp)
		}
	} else {
		expected := filepath.Join(home, sub)
		if exp != expected {
			t.Fatalf("expected %q, got %q", expected, exp)
		}
	}
}
