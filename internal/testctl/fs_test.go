package testctl

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFirstGGUF(t *testing.T) {
	d := t.TempDir()
	if _, err := firstGGUF(d); err == nil {
		t.Fatalf("expected error on empty dir")
	}
	if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	if _, err := firstGGUF(d); err == nil {
		t.Fatalf("expected error when no .gguf present")
	}
	if err := os.WriteFile(filepath.Join(d, "m.GGUf"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	name, err := firstGGUF(d)
	if err != nil { t.Fatalf("unexpected err: %v", err) }
	if name != "m.GGUf" { t.Fatalf("unexpected name: %q", name) }
}

func TestHasHostModelsAndHomeDir(t *testing.T) {
	// point HOME to temp
	d := t.TempDir()
	old := os.Getenv("HOME")
	os.Setenv("HOME", d)
	t.Cleanup(func(){ os.Setenv("HOME", old) })
	if homeDir() != d { t.Fatalf("homeDir mismatch") }
	if hasHostModels() { t.Fatalf("unexpected host models present") }
	llm := filepath.Join(d, "models", "llm")
	if err := os.MkdirAll(llm, 0o755); err != nil { t.Fatal(err) }
	if err := os.WriteFile(filepath.Join(llm, "model.gguf"), []byte("x"), 0o644); err != nil { t.Fatal(err) }
	if !hasHostModels() { t.Fatalf("expected host models detected") }
}
