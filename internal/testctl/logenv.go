package testctl

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Logging with levels
type logLevel int

const (
	levelDebug logLevel = iota
	levelInfo
	levelWarn
	levelError
)

var currentLevel = levelInfo

func init() {
	// default from env if present
	SetLogLevel(envStr("TESTCTL_LOG_LEVEL", "info"))
}

func SetLogLevel(level string) {
	switch strings.ToLower(level) {
	case "debug":
		currentLevel = levelDebug
	case "info":
		currentLevel = levelInfo
	case "warn", "warning":
		currentLevel = levelWarn
	case "error", "err":
		currentLevel = levelError
	default:
		currentLevel = levelInfo
	}
}

func ts() string { return time.Now().Format(time.RFC3339) }

func logf(lvl string, min logLevel, format string, a ...any) {
	if currentLevel > min {
		return
	}
	fmt.Printf("[%s] %s %s\n", ts(), strings.ToUpper(lvl), fmt.Sprintf(format, a...))
}

func debug(format string, a ...any) { logf("DEBUG", levelDebug, format, a...) }
func info(format string, a ...any)  { logf("INFO", levelInfo, format, a...) }
func warn(format string, a ...any)  { logf("WARN", levelWarn, format, a...) }
func errl(format string, a ...any)  { logf("ERROR", levelError, format, a...) }

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
