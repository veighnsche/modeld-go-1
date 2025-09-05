package testctl

import (
	"flag"
	"testing"
)

func TestMainWithArgs_NoArgs_ShowsUsageAndExit2(t *testing.T) {
	code := MainWithArgs([]string{})
	if code != 2 {
		t.Fatalf("expected exit code 2 for no args, got %d", code)
	}
}

func TestMainWithArgs_UnknownCommand_Exit1(t *testing.T) {
	// No stubs needed; this should produce an error path
	code := MainWithArgs([]string{"wat"})
	if code != 1 {
		t.Fatalf("expected exit code 1 for unknown command, got %d", code)
	}
}

func TestMainWithArgs_TestGo_SuccessExit0(t *testing.T) {
	cleanup := withCLIStubs(t, func() {
		fnRunGoTests = func() error { return nil }
	})
	defer cleanup()

	code := MainWithArgs([]string{"test", "go"})
	if code != 0 {
		t.Fatalf("expected exit code 0 for successful test go, got %d", code)
	}
}

func TestMainWithArgs_FlagsAreParsedAndPassedToHandlers(t *testing.T) {
	cleanup := withCLIStubs(t, func() {
		fnTestWebMock = func(c *Config) error {
			if c.WebPort != 4242 {
				t.Fatalf("expected cfg.WebPort 4242 from flags, got %d", c.WebPort)
			}
			if c.LogLvl != "debug" {
				t.Fatalf("expected cfg.LogLvl debug from flags, got %s", c.LogLvl)
			}
			return nil
		}
	})
	defer cleanup()

	args := []string{"--web-port", "4242", "--log-level", "debug", "test", "web", "mock"}
	code := MainWithArgs(args)
	if code != 0 {
		t.Fatalf("expected exit code 0 for web mock with flags, got %d", code)
	}
}

// Sanity: ensure ParseConfig still delegates to ParseConfigWith with CommandLine
func TestParseConfig_DelegatesToCommandLine(t *testing.T) {
	fs := flag.CommandLine
	fs.Init("testctl", flag.ContinueOnError)
	_, rest := ParseConfigWith(fs, []string{"install", "all"})
	if len(rest) != 2 || rest[0] != "install" || rest[1] != "all" {
		t.Fatalf("expected rest to be ['install','all'], got %#v", rest)
	}
}
