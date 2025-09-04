package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modeld/pkg/types"
)

// LoadDir scans a directory for *.gguf files and builds a registry from filenames.
// ID is the full filename (including extension); Path is the absolute file path. Other metadata is empty.
func LoadDir(dir string) ([]types.Model, error) {
	base, err := expandHome(dir)
	if err != nil {
		return nil, err
	}
	abs, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}
	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, fmt.Errorf("read dir: %w", err)
	}
	var models []types.Model
	for _, e := range entries {
		if e.IsDir() { continue }
		name := e.Name()
		if !strings.HasSuffix(strings.ToLower(name), ".gguf") { continue }
		// Use full filename as ID (e.g., "llama-3.1-8b-q4_k_m.gguf")
		id := name
		p := filepath.Join(abs, name)
		models = append(models, types.Model{ID: id, Name: id, Path: p})
	}
	return models, nil
}

// expandHome expands a leading '~' to the user's home directory.
func expandHome(path string) (string, error) {
	if path == "" { return path, nil }
	if path[0] != '~' { return path, nil }
	home, err := os.UserHomeDir()
	if err != nil { return "", fmt.Errorf("home dir: %w", err) }
	if path == "~" { return home, nil }
	// handle cases like ~/models/llm
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}
