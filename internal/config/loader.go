package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

// Config holds runtime parameters for the service.
// Zero values mean "unspecified" and will be replaced by defaults in main.
type Config struct {
	Addr          string `json:"addr" yaml:"addr" toml:"addr"`
	ModelsDir     string `json:"models_dir" yaml:"models_dir" toml:"models_dir"`
	VRAMBudgetMB  int    `json:"vram_budget_mb" yaml:"vram_budget_mb" toml:"vram_budget_mb"`
	VRAMMarginMB  int    `json:"vram_margin_mb" yaml:"vram_margin_mb" toml:"vram_margin_mb"`
	DefaultModel  string `json:"default_model" yaml:"default_model" toml:"default_model"`
}

// Load reads a configuration file based on its extension.
// Supports: .yaml/.yml, .json, .toml
func Load(path string) (Config, error) {
	var cfg Config
	if path == "" {
		return cfg, fmt.Errorf("empty config path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	switch ext := strings.ToLower(filepath.Ext(path)); ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(b, &cfg); err != nil { return cfg, err }
	case ".json":
		if err := json.Unmarshal(b, &cfg); err != nil { return cfg, err }
	case ".toml":
		if err := toml.Unmarshal(b, &cfg); err != nil { return cfg, err }
	default:
		return cfg, fmt.Errorf("unsupported config extension: %s", ext)
	}
	return cfg, nil
}
