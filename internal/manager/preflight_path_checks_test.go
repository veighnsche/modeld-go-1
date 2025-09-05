package manager

import (
	"os"
	"path/filepath"
	"testing"

	"modeld/pkg/types"
)

func TestPreflight_DefaultModelPathDirectory(t *testing.T) {
	dir := t.TempDir()
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: dir}}, DefaultModel: "m"})
	// Install a non-server adapter to exercise filesystem checks
	m.SetInferenceAdapter(&llamaSubprocessAdapter{})
	checks := m.Preflight()
	var isFileOK bool
	for _, c := range checks {
		if c.Name == "default_model_path_is_file" && c.OK { isFileOK = true }
	}
	if isFileOK {
		t.Fatalf("expected default_model_path_is_file to be false for directory")
	}
}

func TestPreflight_DefaultModelPathMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.gguf")
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: path}}, DefaultModel: "m"})
	m.SetInferenceAdapter(&llamaSubprocessAdapter{})
	checks := m.Preflight()
	var existsOK bool
	for _, c := range checks {
		if c.Name == "default_model_path_exists" && c.OK { existsOK = true }
	}
	if existsOK {
		t.Fatalf("expected default_model_path_exists false for missing path")
	}
}

func TestSanityCheck_DefaultModelPathSetForNonServer(t *testing.T) {
	f := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(f, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: f}}, DefaultModel: "m"})
	// Install a non-server adapter to mark LlamaFound and perform FS check
	m.SetInferenceAdapter(&llamaSubprocessAdapter{})
	r := m.SanityCheck()
	if !r.LlamaFound {
		t.Fatalf("expected LlamaFound true when adapter is set later, got %+v", r)
	}
}
