package testctl

import (
	"flag"
	"os"
	"testing"
)

func TestParseConfigWith_FlagsAndEnv(t *testing.T) {
	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	os.Setenv("WEB_PORT", "6000")
	os.Setenv("TESTCTL_LOG_LEVEL", "debug")
	t.Cleanup(func(){ os.Unsetenv("WEB_PORT"); os.Unsetenv("TESTCTL_LOG_LEVEL") })
	cfg, rest := ParseConfigWith(fs, []string{"--web-port", "7000", "--log-level", "warn", "test", "go"})
	if cfg.WebPort != 7000 || cfg.LogLvl != "warn" { t.Fatalf("flag precedence wrong: %+v", cfg) }
	if len(rest) != 2 || rest[0] != "test" || rest[1] != "go" { t.Fatalf("unexpected rest: %+v", rest) }
}

func TestParseConfigWith_DefaultsFromEnv(t *testing.T) {
	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	os.Setenv("WEB_PORT", "6500")
	os.Setenv("TESTCTL_LOG_LEVEL", "debug")
	t.Cleanup(func(){ os.Unsetenv("WEB_PORT"); os.Unsetenv("TESTCTL_LOG_LEVEL") })
	cfg, rest := ParseConfigWith(fs, []string{"test", "go"})
	if cfg.WebPort != 6500 || cfg.LogLvl != "debug" { t.Fatalf("env defaults wrong: %+v", cfg) }
	if len(rest) != 2 || rest[0] != "test" { t.Fatalf("unexpected rest: %+v", rest) }
}

func TestMainWithArgs_Codes(t *testing.T) {
	if code := MainWithArgs([]string{"--help"}); code != 0 { t.Fatalf("help expected 0, got %d", code) }
	if code := MainWithArgs([]string{}); code != 2 { t.Fatalf("empty expected 2, got %d", code) }
	// stub Run path via indirections
	cleanup := withCLIStubs(t, func(){
		fnRunGoTests = func() error { return nil }
	})
	defer cleanup()
	if code := MainWithArgs([]string{"test", "go"}); code != 0 { t.Fatalf("expected 0, got %d", code) }
}
