package testctl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func firstGGUF(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil { return "", err }
	for _, e := range entries {
		if e.IsDir() { continue }
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".gguf") {
			return name, nil
		}
	}
	return "", fmt.Errorf("no .gguf models found in %s", dir)
}

func hasHostModels() bool {
	dir := filepath.Join(homeDir(), "models", "llm")
	entries, err := os.ReadDir(dir)
	if err != nil { return false }
	for _, e := range entries {
		if e.IsDir() { continue }
		if strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			return true
		}
	}
	return false
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" { return h }
	h, _ := os.UserHomeDir()
	return h
}
