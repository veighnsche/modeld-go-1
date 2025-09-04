package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

func TestLoadYAML(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "cfg.yaml", "addr: :9999\nmodels_dir: /tmp\nvram_budget_mb: 123\nvram_margin_mb: 7\ndefault_model: m1\n")
	cfg, err := Load(p)
	if err != nil { t.Fatalf("load: %v", err) }
	if cfg.Addr != ":9999" || cfg.ModelsDir != "/tmp" || cfg.VRAMBudgetMB != 123 || cfg.VRAMMarginMB != 7 || cfg.DefaultModel != "m1" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadJSON(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "cfg.json", `{"addr":":7070","models_dir":"/m","vram_budget_mb":42,"vram_margin_mb":2,"default_model":"m2"}`)
	cfg, err := Load(p)
	if err != nil { t.Fatalf("load: %v", err) }
	if cfg.Addr != ":7070" || cfg.ModelsDir != "/m" || cfg.VRAMBudgetMB != 42 || cfg.VRAMMarginMB != 2 || cfg.DefaultModel != "m2" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadTOML(t *testing.T) {
	d := t.TempDir()
	p := writeTempFile(t, d, "cfg.toml", "addr=\":8081\"\nmodels_dir=\"/x\"\nvram_budget_mb=9\nvram_margin_mb=1\ndefault_model=\"m3\"\n")
	cfg, err := Load(p)
	if err != nil { t.Fatalf("load: %v", err) }
	if cfg.Addr != ":8081" || cfg.ModelsDir != "/x" || cfg.VRAMBudgetMB != 9 || cfg.VRAMMarginMB != 1 || cfg.DefaultModel != "m3" {
		t.Fatalf("unexpected cfg: %+v", cfg)
	}
}

func TestLoadErrors(t *testing.T) {
	if _, err := Load(""); err == nil { t.Fatalf("expected error on empty path") }
	d := t.TempDir()
	p := writeTempFile(t, d, "cfg.txt", "not supported")
	if _, err := Load(p); err == nil { t.Fatalf("expected unsupported extension error") }
}
