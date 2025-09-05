package testctl

import (
	"fmt"
	"os"
	"strings"
)

// Logging
func info(format string, a ...any) { fmt.Println(fmt.Sprintf(format, a...)) }
func warn(format string, a ...any) { fmt.Println(fmt.Sprintf(format, a...)) }
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

// Env helpers
func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	s := strings.ToLower(v)
	return s == "1" || s == "true" || s == "yes"
}
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil {
			return n
		}
	}
	return def
}
