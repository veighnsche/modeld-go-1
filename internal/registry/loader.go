package registry

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modeld/internal/common/fsutil"
	"modeld/pkg/types"
)

// Scanner discovers models from a source (filesystem, manifest, remote, etc.).
type Scanner interface {
	Scan(dir string) ([]types.Model, error)
}

// GGUFScanner scans a directory for *.gguf files and builds a model list.
type GGUFScanner struct{}

// NewGGUFScanner returns a scanner that discovers local GGUF files.
func NewGGUFScanner() *GGUFScanner { return &GGUFScanner{} }

// Scan implements Scanner.
func (s *GGUFScanner) Scan(dir string) ([]types.Model, error) {
	base, err := fsutil.ExpandHome(dir)
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
		if !isGGUF(name) { continue }
		// Use full filename as ID (e.g., "llama-3.1-8b-q4_k_m.gguf")
		id := name
		p := filepath.Join(abs, name)
		models = append(models, types.Model{ID: id, Name: id, Path: p})
	}
	return models, nil
}

// LoadDir is kept for backward compatibility; it uses GGUFScanner under the hood.
func LoadDir(dir string) ([]types.Model, error) {
	return NewGGUFScanner().Scan(dir)
}

func isGGUF(name string) bool {
	return strings.HasSuffix(strings.ToLower(name), ".gguf")
}
