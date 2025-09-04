package registry

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
	"modeld/pkg/types"
)

// Load reads a YAML registry file into a slice of types.Model.
// Example schema:
// models:
//   - id: llama31-8b-q4km
//     name: "Llama 3.1 8B Q4_K_M"
//     path: "/models/llm/llama-3.1-8b-q4_k_m.gguf"
//     quant: "Q4_K_M"
//     family: "llama-3.1"
func Load(path string) ([]types.Model, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read registry: %w", err)
	}
	var doc struct {
		Models []types.Model `yaml:"models"`
	}
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse registry: %w", err)
	}
	return doc.Models, nil
}
