package fsutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ExpandHome expands a leading '~' to the user's home directory.
func ExpandHome(path string) (string, error) {
	if path == "" {
		return path, nil
	}
	if path[0] != '~' {
		return path, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	if path == "~" {
		return home, nil
	}
	// handle cases like ~/models/llm
	return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
}

// PathExists checks if the given path exists.
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || !errors.Is(err, os.ErrNotExist)
}
