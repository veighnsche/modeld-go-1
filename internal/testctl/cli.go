package testctl

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

type Config struct {
	WebPort int
	LogLvl  string
}

func usage() {
	fmt.Println("Usage: testctl [--web-port N] [--log-level info] <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  install all|js|go|py|llama|llama:cuda|go-llama.cpp|go-llama.cpp:cuda|host:docker|host:act|host:all")
	fmt.Println("  test go")
	fmt.Println("  test api:py")
	fmt.Println("  test py:haiku")
	fmt.Println("  test web mock|live:host|auto|haiku")
	fmt.Println("  test all auto")
	fmt.Println("  test ci all [runner:catthehacker|runner:default]")
	fmt.Println("  test ci one <workflow.yml|yaml> [runner:catthehacker|runner:default]")
}

// Run dispatches the CLI command. It returns an error instead of exiting,
// enabling reuse from other packages or tests.
func Run(args []string, cfg *Config) error {
	switch args[0] {
	case "install":
		if len(args) < 2 {
			return fmt.Errorf("install requires a subcommand: all|js|go|py|llama|llama:cuda|host:docker|host:act|host:all")
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
		case "llama", "llama:cuda", "go-llama.cpp", "go-llama.cpp:cuda":
			return fnInstallLlamaCUDA()
		default:
			return fmt.Errorf("unknown install subcommand: %s", args[1])
		}
	case "test":
		if len(args) < 2 {
			return fmt.Errorf("test requires a subcommand: go|api:py|py:haiku|web|all|ci")
		}
		switch args[1] {
		case "go":
			return fnRunGoTests()
		case "api:py":
			return fnRunPyTests()
		case "py:haiku":
			return fnRunPyTestHaiku()
		case "web":
			if len(args) < 3 {
				return fmt.Errorf("test web requires a mode: mock|live:host|auto|haiku")
			}
			switch args[2] {
			case "mock":
				return fnTestWebMock(cfg)
			case "live:host":
				return fnTestWebLiveHost(cfg)
			case "haiku":
				// DO NOT MOCK THE HAIKU FOR TESTING!!!
				return fnTestWebHaikuHost(cfg)
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
		case "ci":
			if len(args) < 3 {
				return fmt.Errorf("test ci requires a subcommand: all|one")
			}
			// default to catthehacker runner mapping unless explicitly set to default
			useCat := true
			parseRunner := func(tok string) bool {
				if tok == "runner:catthehacker" {
					useCat = true
					return true
				}
				if tok == "runner:default" {
					useCat = false
					return true
				}
				return false
			}
			switch args[2] {
			case "all":
				if len(args) >= 4 {
					_ = parseRunner(args[3])
				}
				return fnRunCIAll(useCat)
			case "one":
				if len(args) < 4 {
					return fmt.Errorf("test ci one requires a workflow file name, e.g., ci-go.yml")
				}
				wf := args[3]
				if len(args) >= 5 {
					_ = parseRunner(args[4])
				}
				// If only a basename given, qualify it under .github/workflows
				if !strings.Contains(wf, "/") && !strings.HasPrefix(wf, ".github/workflows/") {
					wf = ".github/workflows/" + wf
				}
				return fnRunCIWorkflow(wf, useCat)
			default:
				return fmt.Errorf("unknown test ci mode: %s", args[2])
			}
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
