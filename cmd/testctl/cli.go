package main

import (
	"flag"
	"fmt"
	"os"
)

type Config struct {
	WebPort int
	LogLvl  string
}

func usage() {
	fmt.Println("Usage: testctl [--web-port N] [--log-level info] <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install all|js|go|py")
	fmt.Println("  test go")
	fmt.Println("  test api:py")
	fmt.Println("  test web mock|live:host|auto")
	fmt.Println("  test all auto")
}

// run dispatches the CLI command. It returns an error instead of exiting,
// enabling reuse from other packages or tests.
func run(args []string, cfg *Config) error {
	switch args[0] {
	case "install":
		if len(args) < 2 { return fmt.Errorf("install requires a subcommand: all|js|go|py") }
		switch args[1] {
		case "all":
			if err := installJS(); err != nil { return err }
			if err := installGo(); err != nil { return err }
			if err := installPy(); err != nil { return err }
			return nil
		case "js":
			return installJS()
		case "go":
			return installGo()
		case "py":
			return installPy()
		default:
			return fmt.Errorf("unknown install subcommand: %s", args[1])
		}
	case "test":
		if len(args) < 2 { return fmt.Errorf("test requires a subcommand: go|api:py|web|all") }
		switch args[1] {
		case "go":
			return runGoTests()
		case "api:py":
			return runPyTests()
		case "web":
			if len(args) < 3 { return fmt.Errorf("test web requires a mode: mock|live:host|auto") }
			switch args[2] {
			case "mock":
				return testWebMock(cfg)
			case "live:host":
				return testWebLiveHost(cfg)
			case "auto":
				if hasHostModels() {
					info("[testctl] Detected host models, running live:host UI suite")
					return testWebLiveHost(cfg)
				}
				info("[testctl] No host models, running mock UI suite")
				return testWebMock(cfg)
			default:
				return fmt.Errorf("unknown test web mode: %s", args[2])
			}
		case "all":
			if len(args) < 3 || args[2] != "auto" { return fmt.Errorf("test all requires 'auto'") }
			if err := runGoTests(); err != nil { return err }
			if err := runPyTests(); err != nil { return err }
			if hasHostModels() {
				info("[testctl] Detected host models, running live:host UI suite")
				return testWebLiveHost(cfg)
			}
			info("[testctl] No host models, running mock UI suite")
			return testWebMock(cfg)
		default:
			return fmt.Errorf("unknown test subcommand: %s", args[1])
		}
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func parseConfig() (*Config, []string) {
	cfg := &Config{}
	webPort := flag.Int("web-port", envInt("WEB_PORT", 5173), "Port for Vite preview")
	logLvl := flag.String("log-level", envStr("TESTCTL_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	flag.Parse()
	cfg.WebPort = *webPort
	cfg.LogLvl = *logLvl
	return cfg, flag.Args()
}

func main() {
	cfg, args := parseConfig()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	if err := run(args, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
