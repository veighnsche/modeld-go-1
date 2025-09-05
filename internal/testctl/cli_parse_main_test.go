package testctl

import (
	"os"
	"testing"
)

func TestParseConfig_UsesGlobal(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Setenv("WEB_PORT", "5555")
	os.Setenv("TESTCTL_LOG_LEVEL", "debug")
	t.Cleanup(func() { os.Unsetenv("WEB_PORT"); os.Unsetenv("TESTCTL_LOG_LEVEL") })
	os.Args = []string{"testctl", "--web-port", "7777", "--log-level", "error", "test", "go"}
	cfg, rest := ParseConfig()
	if cfg.WebPort != 7777 || cfg.LogLvl != "error" {
		t.Fatalf("flag precedence wrong: %+v", cfg)
	}
	if len(rest) != 2 || rest[0] != "test" || rest[1] != "go" {
		t.Fatalf("unexpected rest: %+v", rest)
	}
}

func TestMain_ReturnCodes(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	// help
	os.Args = []string{"testctl", "--help"}
	if code := Main(); code != 0 {
		t.Fatalf("help expected 0, got %d", code)
	}
	// empty
	os.Args = []string{"testctl"}
	if code := Main(); code != 2 {
		t.Fatalf("empty expected 2, got %d", code)
	}
	// success path with stubbed run
	cleanup := withCLIStubs(t, func() { fnRunGoTests = func() error { return nil } })
	defer cleanup()
	os.Args = []string{"testctl", "test", "go"}
	if code := Main(); code != 0 {
		t.Fatalf("expected 0, got %d", code)
	}
}
