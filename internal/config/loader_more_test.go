package config

import (
	"testing"
)

func TestLoad_NonexistentFile(t *testing.T) {
	if _, err := Load("/definitely/not/a/real/file-12345.yaml"); err == nil {
		t.Fatalf("expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "bad.yaml", "addr: :8080\n: broken\n")
	if _, err := Load(p); err == nil {
		t.Fatalf("expected YAML unmarshal error")
	}
}

func TestLoad_InvalidJSON(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "bad.json", `{ "addr": ":8080", "models_dir": }`)
	if _, err := Load(p); err == nil {
		t.Fatalf("expected JSON unmarshal error")
	}
}

func TestLoad_InvalidTOML(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "bad.toml", "addr=:8080\nmodels_dir\n")
	if _, err := Load(p); err == nil {
		t.Fatalf("expected TOML unmarshal error")
	}
}
