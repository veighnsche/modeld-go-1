package registry

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGGUFScanner_ScanFiltersGGUF(t *testing.T) {
	dir := t.TempDir()
	// create files
	files := []string{
		"a.gguf",
		"b.GGUF", // case-insensitive
		"not-model.txt",
		"model.bin",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte(""), 0o644); err != nil {
			t.Fatalf("write temp file: %v", err)
		}
	}
	s := NewGGUFScanner()
	models, err := s.Scan(dir)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	// ensure IDs are filenames
	ids := []string{models[0].ID, models[1].ID}
	for _, id := range ids {
		if !strings.HasSuffix(strings.ToLower(id), ".gguf") {
			t.Fatalf("id not gguf: %s", id)
		}
	}
}

func TestGGUFScanner_ExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir on this platform: %v", err)
	}
	// create temporary directory under home
	hTmp, err := os.MkdirTemp(home, "modeld-registry-*")
	if err != nil {
		t.Skipf("cannot create temp under home: %v", err)
	}
	defer os.RemoveAll(hTmp)
	// create a gguf file inside it
	if err := os.WriteFile(filepath.Join(hTmp, "x.gguf"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	// build path with ~ prefix
	var tildePath string
	if runtime.GOOS == "windows" {
		// On Windows, home might contain drive; ExpandHome still handles ~/<rest>
		tildePath = filepath.Join("~", filepath.Base(hTmp))
	} else {
		tildePath = "~/" + filepath.Base(hTmp)
	}
	s := NewGGUFScanner()
	models, err := s.Scan(tildePath)
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(models) != 1 || models[0].ID != "x.gguf" {
		t.Fatalf("unexpected models: %+v", models)
	}
}

func TestLoadDirWrapper(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "m.gguf"), []byte(""), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	models, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(models) != 1 || models[0].ID != "m.gguf" {
		t.Fatalf("unexpected: %+v", models)
	}
}
