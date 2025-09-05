package testctl

import (
	"os"
	"runtime"
	"strings"
)

// isArchLike returns true if running on an Arch Linux or Arch-like distro.
func isArchLike() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return false
	}
	s := string(data)
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "ID=") || strings.HasPrefix(line, "ID_LIKE=") {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "arch") {
				return true
			}
		}
	}
	return false
}
