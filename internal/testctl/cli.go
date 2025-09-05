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

func usage() {
	fmt.Println("Usage: testctl [--web-port N] [--log-level info] <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install all|js|go|py|host:docker|host:act|host:all")
	fmt.Println("  test go")
	fmt.Println("  test api:py")
	fmt.Println("  test web mock|live:host|auto")
	fmt.Println("  test all auto")
}

// Run dispatches the CLI command. It returns an error instead of exiting,
// enabling reuse from other packages or tests.
func Run(args []string, cfg *Config) error {
	switch args[0] {
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("install requires a subcommand: all|js|go|py|host:docker|host:act|host:all")
		}
		switch args[1] {
		case "all":
			if err := fnInstallJS(); err != nil {
				return err
			}
			if err := fnInstallGo(); err != nil {
				return err
			}
			if err := fnInstallPy(); err != nil {
				return err
			}
			return nil
		case "host:docker":
			return fnInstallHostDocker()
		case "host:act":
			return fnInstallHostAct()
		case "host:all":
			if err := fnInstallHostDocker(); err != nil {
				return err
			}
			if err := fnInstallHostAct(); err != nil {
				return err
			}
			return nil
		case "js":
			return fnInstallJS()
		case "go":
			return fnInstallGo()
		case "py":
			return fnInstallPy()
		default:
			return fmt.Errorf("unknown install subcommand: %s", args[1])
		}
	case "test":
		if len(args) < 2 {
			return fmt.Errorf("test requires a subcommand: go|api:py|web|all")
		}
		switch args[1] {
		case "go":
			return fnRunGoTests()
		case "api:py":
			return fnRunPyTests()
		case "web":
			if len(args) < 3 {
				return fmt.Errorf("test web requires a mode: mock|live:host|auto")
			}
			switch args[2] {
			case "mock":
				return fnTestWebMock(cfg)
			case "live:host":
				return fnTestWebLiveHost(cfg)
			case "auto":
				if fnHasHostModels() {
					info("[testctl] Detected host models, running live:host UI suite")
					return fnTestWebLiveHost(cfg)
				}
				info("[testctl] No host models, running mock UI suite")
				return fnTestWebMock(cfg)
			default:
				return fmt.Errorf("unknown test web mode: %s", args[2])
			}
		case "all":
			if len(args) < 3 || args[2] != "auto" {
				return fmt.Errorf("test all requires 'auto'")
			}
			if err := fnRunGoTests(); err != nil {
				return err
			}
			if err := fnRunPyTests(); err != nil {
				return err
			}
			if fnHasHostModels() {
				info("[testctl] Detected host models, running live:host UI suite")
				return fnTestWebLiveHost(cfg)
			}
			info("[testctl] No host models, running mock UI suite")
			return fnTestWebMock(cfg)
		default:
			return fmt.Errorf("unknown test subcommand: %s", args[1])
		}
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func ParseConfig() (*Config, []string) {
	return ParseConfigWith(flag.CommandLine, os.Args[1:])
}

// ParseConfigWith parses flags using the provided FlagSet and args slice.
// This enables tests to inject their own FlagSet and arguments without
// mutating global state.
func ParseConfigWith(fs *flag.FlagSet, args []string) (*Config, []string) {
	cfg := &Config{}
	webPort := fs.Int("web-port", envInt("WEB_PORT", 5173), "Port for Vite preview")
	logLvl := fs.String("log-level", envStr("TESTCTL_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	_ = fs.Parse(args)
	cfg.WebPort = *webPort
	cfg.LogLvl = *logLvl
	return cfg, fs.Args()
}

// MainWithArgs is a testable variant of Main that accepts args explicitly.
// It returns an exit code (0 for success, non-zero on error).
func MainWithArgs(args []string) int {
	// If user explicitly asks for help, print usage and exit 0
	for _, a := range args {
		if a == "-h" || a == "--help" || a == "help" {
			usage()
			return 0
		}
	}
	fs := flag.NewFlagSet("testctl", flag.ContinueOnError)
	cfg, rest := ParseConfigWith(fs, args)
	if len(rest) == 0 {
		usage()
		return 2
	}
	if err := Run(rest, cfg); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

// Main returns an exit code (0 for success, non-zero on error) for use by cmd/testctl.
func Main() int { return MainWithArgs(os.Args[1:]) }
