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
	Addr         string `json:"addr" yaml:"addr" toml:"addr"`
	ModelsDir    string `json:"models_dir" yaml:"models_dir" toml:"models_dir"`
	VRAMBudgetMB int    `json:"vram_budget_mb" yaml:"vram_budget_mb" toml:"vram_budget_mb"`
	VRAMMarginMB int    `json:"vram_margin_mb" yaml:"vram_margin_mb" toml:"vram_margin_mb"`
	DefaultModel string `json:"default_model" yaml:"default_model" toml:"default_model"`
	// Observability & HTTP
	LogLevel     string `json:"log_level" yaml:"log_level" toml:"log_level"`
	MaxBodyBytes int64  `json:"max_body_bytes" yaml:"max_body_bytes" toml:"max_body_bytes"`
	InferTimeout string `json:"infer_timeout" yaml:"infer_timeout" toml:"infer_timeout"`
	// CORS
	CORSEnabled        bool     `json:"cors_enabled" yaml:"cors_enabled" toml:"cors_enabled"`
	CORSAllowedOrigins []string `json:"cors_allowed_origins" yaml:"cors_allowed_origins" toml:"cors_allowed_origins"`
	CORSAllowedMethods []string `json:"cors_allowed_methods" yaml:"cors_allowed_methods" toml:"cors_allowed_methods"`
	CORSAllowedHeaders []string `json:"cors_allowed_headers" yaml:"cors_allowed_headers" toml:"cors_allowed_headers"`
	// Backpressure
	MaxQueueDepth int    `json:"max_queue_depth" yaml:"max_queue_depth" toml:"max_queue_depth"`
	MaxWait       string `json:"max_wait" yaml:"max_wait" toml:"max_wait"`
	// Real inference / llama.cpp
	RealInferEnabled bool   `json:"real_infer" yaml:"real_infer" toml:"real_infer"`
	LlamaBin         string `json:"llama_bin" yaml:"llama_bin" toml:"llama_bin"`
	LlamaCtx         int    `json:"llama_ctx" yaml:"llama_ctx" toml:"llama_ctx"`
	LlamaThreads     int    `json:"llama_threads" yaml:"llama_threads" toml:"llama_threads"`
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
		if err := yaml.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	case ".json":
		if err := json.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	case ".toml":
		if err := toml.Unmarshal(b, &cfg); err != nil {
			return cfg, err
		}
	default:
		return cfg, fmt.Errorf("unsupported config extension: %s", ext)
	}
	return cfg, nil
}
