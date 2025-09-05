package testctl

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	WebPort int
	LogLvl  string
}

func usage() { _ = buildRootCmd().Help() }

// Run dispatches the CLI command. It returns an error instead of exiting,
// enabling reuse from other packages or tests.
func Run(args []string, cfg *Config) error {
	cmd := buildRootCmdWith(cfg)
	cmd.SetArgs(args)
	return cmd.Execute()
}

func ParseConfig() (*Config, []string) {
	return ParseConfigWith(flag.CommandLine, os.Args[1:])
}

// ParseConfigWith parses flags using the provided FlagSet and args slice.
// This enables tests to inject their own FlagSet and arguments without
// mutating global state.
func ParseConfigWith(fs *flag.FlagSet, args []string) (*Config, []string) {
	cfg := &Config{}
	// Only define flags if they are not already present on the provided FlagSet.
	if fs.Lookup("web-port") == nil {
		fs.Int("web-port", envInt("WEB_PORT", 5173), "Port for Vite preview")
	}
	if fs.Lookup("log-level") == nil {
		fs.String("log-level", envStr("TESTCTL_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	}
	_ = fs.Parse(args)
	// Read back values from the parsed FlagSet, falling back to env defaults.
	wp := envInt("WEB_PORT", 5173)
	if f := fs.Lookup("web-port"); f != nil {
		var n int
		_, _ = fmt.Sscanf(f.Value.String(), "%d", &n)
		if n != 0 {
			wp = n
		}
	}
	ll := envStr("TESTCTL_LOG_LEVEL", "info")
	if f := fs.Lookup("log-level"); f != nil {
		if v := f.Value.String(); v != "" {
			ll = v
		}
	}
	cfg.WebPort = wp
	cfg.LogLvl = ll
	return cfg, fs.Args()
}

// MainWithArgs is a testable variant of Main that accepts args explicitly.
// It returns an exit code (0 for success, non-zero on error).
func MainWithArgs(args []string) int {
	cfg := &Config{WebPort: envInt("WEB_PORT", 5173), LogLvl: envStr("TESTCTL_LOG_LEVEL", "info")}
	cmd := buildRootCmdWith(cfg)
	if len(args) == 0 {
		// Match prior behavior: show usage and exit 2 when no args are provided
		_ = cmd.Help()
		return 2
	}
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

// Main returns an exit code (0 for success, non-zero on error) for use by cmd/testctl.
func Main() int { return MainWithArgs(os.Args[1:]) }
