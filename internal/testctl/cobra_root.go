package testctl

import (
	"fmt"
	"os"
	"strings"
	"github.com/spf13/cobra"
)

// buildRootCmd is a convenience for help-only fallbacks.
func buildRootCmd() *cobra.Command { return buildRootCmdWith(&Config{WebPort: 5173, LogLvl: "info"}) }

// buildRootCmdWith constructs a Cobra command tree wired to the existing fn* actions.
func buildRootCmdWith(cfg *Config) *cobra.Command {
	root := &cobra.Command{
		Use:   "testctl",
		Short: "Test and dev utilities grouped by environment",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Persistent flags -> Config
	root.PersistentFlags().Int("web-port", cfg.WebPort, "Port for Vite preview (defaults WEB_PORT or 5173)")
	root.PersistentFlags().String("log-level", cfg.LogLvl, "Log level: debug|info|warn|error (defaults TESTCTL_LOG_LEVEL or info)")
	root.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		if f := cmd.InheritedFlags().Lookup("web-port"); f != nil {
			var n int
			_, _ = fmt.Sscanf(f.Value.String(), "%d", &n)
			if n != 0 { cfg.WebPort = n }
		}
		if f := cmd.InheritedFlags().Lookup("log-level"); f != nil {
			if v := f.Value.String(); v != "" { cfg.LogLvl = v }
		}
		SetLogLevel(cfg.LogLvl)
	}

	// install group
	installCmd := &cobra.Command{Use: "install", Short: "Install dependencies/tools", Args: func(cmd *cobra.Command, args []string) error {
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("install requires a subcommand: all|nodejs|go|py|llama|llama:cuda|go-llama.cpp|go-llama.cpp:cuda|host:docker|host:act|host:all")
	}}
	installAll := &cobra.Command{Use: "all", Short: "Install nodejs, go, py", Example: "  testctl install all", RunE: func(cmd *cobra.Command, args []string) error {
		if err := fnInstallNodeJS(); err != nil { return err }
		if err := fnInstallGo(); err != nil { return err }
		return fnInstallPy()
	}}
	installNode := &cobra.Command{Use: "nodejs", Aliases: []string{"js"}, Short: "Ensure pnpm and install JS deps", Example: "  testctl install nodejs", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallNodeJS() }}
	installGo := &cobra.Command{Use: "go", Short: "Download Go modules", Example: "  testctl install go", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallGo() }}
	installPy := &cobra.Command{Use: "py", Short: "Create venv and install Python deps", Example: "  testctl install py", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallPy() }}
	installLlama := &cobra.Command{Use: "llama", Short: "Build llama.cpp", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallLlamaCUDA() }}
	installLlamaCUDA := &cobra.Command{Use: "llama:cuda", Short: "Build llama.cpp with CUDA", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallLlamaCUDA() }}
	installGoLlama := &cobra.Command{Use: "go-llama.cpp", Short: "Build llama.cpp for go-llama.cpp", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallLlamaCUDA() }}
	installGoLlamaCUDA := &cobra.Command{Use: "go-llama.cpp:cuda", Short: "Build llama.cpp (CUDA) for go-llama.cpp", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallLlamaCUDA() }}
	installHostDocker := &cobra.Command{Use: "host:docker", Short: "Install Docker (CI-only)", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallHostDocker() }}
	installHostAct := &cobra.Command{Use: "host:act", Short: "Install GitHub Actions local runner (CI)", RunE: func(cmd *cobra.Command, args []string) error { return fnInstallHostAct() }}
	installHostAll := &cobra.Command{Use: "host:all", Short: "Install Docker + act (CI-only)", RunE: func(cmd *cobra.Command, args []string) error {
		if err := fnInstallHostDocker(); err != nil { return err }
		return fnInstallHostAct()
	}}
	installCmd.AddCommand(installAll, installNode, installGo, installPy, installLlama, installLlamaCUDA, installGoLlama, installGoLlamaCUDA, installHostDocker, installHostAct, installHostAll)
	root.AddCommand(installCmd)

	// test group
	testCmd := &cobra.Command{Use: "test", Short: "Run tests", Args: func(cmd *cobra.Command, args []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("test requires a subcommand: go|api:py|py:haiku|web|all|ci")
	}}
	testGo := &cobra.Command{Use: "go", Short: "Run Go tests", RunE: func(cmd *cobra.Command, args []string) error { return fnRunGoTests() }}
	testPyAPI := &cobra.Command{Use: "api:py", Short: "Run Python API E2E tests", RunE: func(cmd *cobra.Command, args []string) error { return fnRunPyTests() }}
	testPyHaiku := &cobra.Command{Use: "py:haiku", Short: "Run Python haiku test only", RunE: func(cmd *cobra.Command, args []string) error { return fnRunPyTestHaiku() }}

	// test web
	testWeb := &cobra.Command{Use: "web", Short: "Run Cypress UI tests (Cypress-only)", Example: "  testctl test web mock\n  testctl test web live:host\n  testctl test web haiku\n  testctl test web auto", Args: func(cmd *cobra.Command, args []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("test web requires a mode: mock|live:host|auto|haiku")
	}}
	webMock := &cobra.Command{Use: "mock", Short: "Run Cypress against mocked API", RunE: func(cmd *cobra.Command, args []string) error { return fnTestWebMock(cfg) }}
	webLive := &cobra.Command{Use: "live:host", Short: "Run Cypress against local server using host models", RunE: func(cmd *cobra.Command, args []string) error { return fnTestWebLiveHost(cfg) }}
	webHaiku := &cobra.Command{Use: "haiku", Short: "Run only the Haiku Cypress spec (live backend)", RunE: func(cmd *cobra.Command, args []string) error { return fnTestWebHaikuHost(cfg) }}
	webAuto := &cobra.Command{Use: "auto", Short: "Choose live:host if host models exist, else mock", RunE: func(cmd *cobra.Command, args []string) error {
		if fnHasHostModels() {
			info("[testctl] Detected host models, running live:host UI suite")
			return fnTestWebLiveHost(cfg)
		}
		info("[testctl] No host models, running mock UI suite")
		return fnTestWebMock(cfg)
	}}
	testWeb.AddCommand(webMock, webLive, webHaiku, webAuto)

	// test all auto
	testAll := &cobra.Command{Use: "all", Short: "Run all test suites", Args: func(cmd *cobra.Command, args []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("test all requires 'auto'")
	}}
	testAllAuto := &cobra.Command{Use: "auto", Short: "Go → Python → Web (auto mode)", RunE: func(cmd *cobra.Command, args []string) error {
		if err := fnRunGoTests(); err != nil { return err }
		if err := fnRunPyTests(); err != nil { return err }
		if fnHasHostModels() {
			info("[testctl] Detected host models, running live:host UI suite")
			return fnTestWebLiveHost(cfg)
		}
		info("[testctl] No host models, running mock UI suite")
		return fnTestWebMock(cfg)
	}}
	testAll.AddCommand(testAllAuto)

	// test ci
	testCI := &cobra.Command{Use: "ci", Short: "Run GitHub Actions via act", Example: "  testctl test ci all runner:catthehacker -- -j\n  testctl test ci one ci-e2e-cypress.yml runner:default -- -j", Args: func(cmd *cobra.Command, args []string) error { return nil }, RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("test ci requires a subcommand: all|one")
	}}
	testCIAll := &cobra.Command{Use: "all", Short: "Run all workflows [runner:catthehacker|runner:default] [-- <extra act args>]", RunE: func(cmd *cobra.Command, args []string) error {
		useCat := true
		// parse optional runner token and extra args after --
		tail := append([]string(nil), args...)
		var extra []string
		for i, t := range tail {
			if t == "--" { extra = tail[i+1:]; tail = tail[:i]; break }
		}
		if len(tail) >= 1 {
			if tail[0] == "runner:default" { useCat = false }
		}
		return fnRunCIAll(useCat, extra)
	}}
	testCIOne := &cobra.Command{Use: "one <workflow.yml|yaml>", Short: "Run one workflow [runner:…] [-- <extra act args>]", Args: cobra.MinimumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		useCat := true
		wf := args[0]
		tail := args[1:]
		var extra []string
		for i, t := range tail {
			if t == "--" { extra = tail[i+1:]; tail = tail[:i]; break }
		}
		if len(tail) >= 1 {
			if tail[0] == "runner:default" { useCat = false }
		}
		if !strings.Contains(wf, "/") && !strings.HasPrefix(wf, ".github/workflows/") {
			wf = ".github/workflows/" + wf
		}
		return fnRunCIWorkflow(wf, useCat, extra)
	}}
	testCI.AddCommand(testCIAll, testCIOne)

	testCmd.AddCommand(testGo, testPyAPI, testPyHaiku, testWeb, testAll, testCI)
	root.AddCommand(testCmd)

	// completion command
	completionCmd := &cobra.Command{Use: "completion", Short: "Generate the autocompletion script for the specified shell"}
	completionCmd.AddCommand(&cobra.Command{Use: "bash", Short: "Bash completion", RunE: func(cmd *cobra.Command, args []string) error { return root.GenBashCompletion(os.Stdout) }})
	completionCmd.AddCommand(&cobra.Command{Use: "zsh", Short: "Zsh completion", RunE: func(cmd *cobra.Command, args []string) error { return root.GenZshCompletion(os.Stdout) }})
	completionCmd.AddCommand(&cobra.Command{Use: "fish", Short: "Fish completion", RunE: func(cmd *cobra.Command, args []string) error { return root.GenFishCompletion(os.Stdout, true) }})
	completionCmd.AddCommand(&cobra.Command{Use: "powershell", Short: "PowerShell completion", RunE: func(cmd *cobra.Command, args []string) error { return root.GenPowerShellCompletionWithDesc(os.Stdout) }})
	root.AddCommand(completionCmd)

	return root
}
