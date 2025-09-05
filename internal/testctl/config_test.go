package testctl

import (
	"flag"
	"os"
	"testing"
)

func withEnv(key, val string) func() {
	old, had := os.LookupEnv(key)
	os.Setenv(key, val)
	return func() {
		if had {
			os.Setenv(key, old)
		} else {
			os.Unsetenv(key)
		}
	}
}

func TestParseConfigWith_FlagsOverrideEnv(t *testing.T) {
	defer withEnv("WEB_PORT", "6000")()
	defer withEnv("TESTCTL_LOG_LEVEL", "warn")()

	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	cfg, rest := ParseConfigWith(fs, []string{"--web-port", "1234", "--log-level", "debug", "test", "go"})

	if cfg.WebPort != 1234 {
		t.Fatalf("web-port expected 1234, got %d", cfg.WebPort)
	}
	if cfg.LogLvl != "debug" {
		t.Fatalf("log-level expected debug, got %s", cfg.LogLvl)
	}
	if len(rest) != 2 || rest[0] != "test" || rest[1] != "go" {
		t.Fatalf("expected remaining args ['test','go'], got %#v", rest)
	}
}

func TestParseConfigWith_EnvAndDefaults(t *testing.T) {
	defer withEnv("WEB_PORT", "7007")()
	defer withEnv("TESTCTL_LOG_LEVEL", "error")()

	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	cfg, rest := ParseConfigWith(fs, []string{"test", "web", "mock"})

	if cfg.WebPort != 7007 {
		t.Fatalf("web-port expected from env 7007, got %d", cfg.WebPort)
	}
	if cfg.LogLvl != "error" {
		t.Fatalf("log-level expected from env error, got %s", cfg.LogLvl)
	}
	if len(rest) != 3 || rest[0] != "test" || rest[1] != "web" || rest[2] != "mock" {
		t.Fatalf("expected remaining args ['test','web','mock'], got %#v", rest)
	}
}

func TestParseConfigWith_DefaultsWhenNoEnvOrFlags(t *testing.T) {
	// ensure envs are unset
	os.Unsetenv("WEB_PORT")
	os.Unsetenv("TESTCTL_LOG_LEVEL")

	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	cfg, rest := ParseConfigWith(fs, []string{"install", "all"})

	if cfg.WebPort != 5173 {
		t.Fatalf("web-port expected default 5173, got %d", cfg.WebPort)
	}
	if cfg.LogLvl != "info" {
		t.Fatalf("log-level expected default info, got %s", cfg.LogLvl)
	}
	if len(rest) != 2 || rest[0] != "install" || rest[1] != "all" {
		t.Fatalf("expected remaining args ['install','all'], got %#v", rest)
	}
}
