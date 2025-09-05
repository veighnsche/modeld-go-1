package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	WebPort int
	LogLvl  string
}

// chooseFreePort finds an available TCP port by asking the kernel for :0
func chooseFreePort() (int, error) {
    l, err := net.Listen("tcp", "127.0.0.1:0")
    if err != nil { return 0, err }
    defer l.Close()
    addr := l.Addr().(*net.TCPAddr)
    return addr.Port, nil
}

func main() {
	cfg := &Config{}

	// Global flags
	webPort := flag.Int("web-port", envInt("WEB_PORT", 5173), "Port for Vite preview")
	// deprecated: --force is no longer needed; ports are selected automatically when busy.
	logLvl := flag.String("log-level", envStr("TESTCTL_LOG_LEVEL", "info"), "Log level: debug|info|warn|error")
	flag.Parse()

	cfg.WebPort = *webPort
	cfg.LogLvl = *logLvl

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	switch args[0] {
	case "install":
		if len(args) < 2 { die("install requires a subcommand: all|js|go|py") }
		switch args[1] {
		case "all":
			must(installJS())
			must(installGo())
			must(installPy())
		case "js":
			must(installJS())
		case "go":
			must(installGo())
		case "py":
			must(installPy())
		default:
			die("unknown install subcommand: %s", args[1])
		}
	case "test":
		if len(args) < 2 { die("test requires a subcommand: go|api:py|web|all") }
		switch args[1] {
		case "go":
			must(runGoTests())
		case "api:py":
			must(runPyTests())
		case "web":
			if len(args) < 3 { die("test web requires a mode: mock|live:host|auto") }
			switch args[2] {
			case "mock":
				must(testWebMock(cfg))
			case "live:host":
				must(testWebLiveHost(cfg))
			case "auto":
				if hasHostModels() {
					info("[testctl] Detected host models, running live:host UI suite")
					must(testWebLiveHost(cfg))
				} else {
					info("[testctl] No host models, running mock UI suite")
					must(testWebMock(cfg))
				}
			default:
				die("unknown test web mode: %s", args[2])
			}
		case "all":
			if len(args) < 3 || args[2] != "auto" { die("test all requires 'auto'") }
			must(runGoTests())
			must(runPyTests())
			if hasHostModels() {
				info("[testctl] Detected host models, running live:host UI suite")
				must(testWebLiveHost(cfg))
			} else {
				info("[testctl] No host models, running mock UI suite")
				must(testWebMock(cfg))
			}
		default:
			die("unknown test subcommand: %s", args[1])
		}
	default:
		die("unknown command: %s", args[0])
	}
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

// Installers
func installJS() error {
	info("Installing JS dependencies...")
	// Try corepack to enable pnpm if not present
	if _, err := exec.LookPath("pnpm"); err != nil {
		if _, corepackErr := exec.LookPath("corepack"); corepackErr == nil {
			_ = runCmdVerbose(context.Background(), "corepack", "enable")
			_ = runCmdVerbose(context.Background(), "corepack", "prepare", "pnpm@9.7.1", "--activate")
		}
	}
	if _, err := exec.LookPath("pnpm"); err != nil {
		return fmt.Errorf("pnpm is required; install via 'npm i -g pnpm' or enable corepack")
	}
	if err := runCmdVerbose(context.Background(), "pnpm", "install", "--frozen-lockfile"); err != nil { return err }
	if err := runCmdVerbose(context.Background(), "pnpm", "-C", "web", "install", "--frozen-lockfile"); err != nil { return err }
	info("JS dependencies installed.")
	return nil
}

func installGo() error {
	info("Downloading Go modules...")
	return runCmdVerbose(context.Background(), "go", "mod", "download")
}

func installPy() error {
	info("Installing Python dependencies...")
	pyDir := filepath.Join("tests", "e2e_py")
	venv := filepath.Join(pyDir, ".venv")
	if _, err := os.Stat(venv); os.IsNotExist(err) {
		if err := os.MkdirAll(venv, 0o755); err != nil { return err }
	}
	// python3 -m venv
	if err := runCmdVerbose(context.Background(), "python3", "-m", "venv", venv); err != nil { return err }
	// pip install -r requirements.txt
	pip := filepath.Join(venv, "bin", "pip")
	req := filepath.Join(pyDir, "requirements.txt")
	if err := runCmdVerbose(context.Background(), pip, "install", "-r", req); err != nil { return err }
	info("Python dependencies installed (venv: %s).", venv)
	return nil
}

// Tests
func runGoTests() error {
	info("==== Run Go tests ====")
	return runCmdStreaming(context.Background(), "go", "test", "./...", "-v")
}

func runPyTests() error {
	info("==== Run Python E2E tests ====")
	pyDir := filepath.Join("tests", "e2e_py")
	venv := filepath.Join(pyDir, ".venv")
	pytest := filepath.Join(venv, "bin", "pytest")
	if _, err := os.Stat(pytest); os.IsNotExist(err) {
		info("Python venv not found; installing...")
		if err := installPy(); err != nil { return err }
	}
	return runCmdStreaming(context.Background(), pytest, "-q", pyDir)
}

// UI suites
func testWebMock(cfg *Config) error {
	info("==== Run Cypress (Mock) ====")
	// pick a port: prefer cfg.WebPort, but if busy choose a free one
	webPort := cfg.WebPort
	if busy, _ := isPortBusy(webPort); busy {
		p, err := chooseFreePort()
		if err != nil { return err }
		warn("[ports] %d busy; using free port %d for preview", webPort, p)
		webPort = p
	}
	defer func() { _ = killProcesses() }()
	// Build and preview with mocks
	if err := runEnvCmdStreaming(context.Background(), map[string]string{
		"VITE_USE_MOCKS": "1",
	}, "pnpm", "-C", "web", "build"); err != nil { return err }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	preview := exec.CommandContext(ctx, "pnpm", "-C", "web", "preview", "--port", fmt.Sprint(webPort))
	preview.Stdout = os.Stdout
	preview.Stderr = os.Stderr
	if err := preview.Start(); err != nil { return err }
	defer func() { _ = preview.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d", webPort), 200, 60*time.Second); err != nil { return err }
	// Run cypress with dynamic baseUrl
	return runEnvCmdStreaming(context.Background(), map[string]string{
		"CYPRESS_BASE_URL": fmt.Sprintf("http://localhost:%d", webPort),
	}, "pnpm", "run", "test:e2e:run")
}

func testWebLiveHost(cfg *Config) error {
	if !hasHostModels() { return errors.New("host models not found in $HOME/models/llm; cannot run live:host") }
	info("==== Run Cypress (Live:Host) ====")
	// determine API port (prefer 18080, else free)
	apiPort := 18080
	if busy, _ := isPortBusy(apiPort); busy {
		p, err := chooseFreePort()
		if err != nil { return err }
		warn("[ports] %d busy; using free port %d for API", apiPort, p)
		apiPort = p
	}
	// determine web preview port (prefer cfg.WebPort, else free)
	webPort := cfg.WebPort
	if busy, _ := isPortBusy(webPort); busy {
		p, err := chooseFreePort()
		if err != nil { return err }
		warn("[ports] %d busy; using free port %d for preview", webPort, p)
		webPort = p
	}
	defer func() { _ = killProcesses() }()
	// Start server with host models
	modelsDir := filepath.Join(homeDir(), "models", "llm")
	defaultModel, err := firstGGUF(modelsDir)
	if err != nil { return fmt.Errorf("finding default model: %w", err) }
	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()
	srv := exec.CommandContext(srvCtx, "bash", "-lc", fmt.Sprintf("go run ./cmd/modeld --addr :%d --models-dir '%s' --default-model '%s' --cors-enabled --cors-origins '*'", apiPort, modelsDir, defaultModel))
	srv.Stdout = os.Stdout
	srv.Stderr = os.Stderr
	if err := srv.Start(); err != nil { return err }
	defer func() { _ = srv.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d/healthz", apiPort), 200, 60*time.Second); err != nil { return err }
	// Build and preview without mocks, pointing to API
	if err := runEnvCmdStreaming(context.Background(), map[string]string{
		"VITE_USE_MOCKS":       "0",
		"VITE_API_BASE_URL":   fmt.Sprintf("http://localhost:%d", apiPort),
		"VITE_SEND_STREAM_FIELD": "true",
	}, "pnpm", "-C", "web", "build"); err != nil { return err }
	pvCtx, pvCancel := context.WithCancel(context.Background())
	defer pvCancel()
	preview := exec.CommandContext(pvCtx, "pnpm", "-C", "web", "preview", "--port", fmt.Sprint(webPort))
	preview.Stdout = os.Stdout
	preview.Stderr = os.Stderr
	if err := preview.Start(); err != nil { return err }
	defer func() { _ = preview.Process.Kill() }()
	if err := waitHTTP(fmt.Sprintf("http://localhost:%d", webPort), 200, 60*time.Second); err != nil { return err }
	// Run cypress (headless) with dynamic baseUrl
	return runEnvCmdStreaming(context.Background(), map[string]string{
		"CYPRESS_BASE_URL": fmt.Sprintf("http://localhost:%d", webPort),
	}, "pnpm", "run", "test:e2e:run")
}

// Helpers
func ensurePorts(ports []int, force bool) error {
	for _, p := range ports {
		busy, desc := isPortBusy(p)
		if !busy {
			info("[ports] Port %d is free", p)
			continue
		}
		warn("[ports] Port %d is busy: %s", p, desc)
		if !force {
			return fmt.Errorf("port %d is in use; re-run with --force or free it", p)
		}
		info("[ports] --force set; attempting to kill listeners on :%d", p)
		_ = runCmdVerbose(context.Background(), "fuser", "-k", fmt.Sprintf("%d/tcp", p))
		time.Sleep(300 * time.Millisecond)
		busy2, _ := isPortBusy(p)
		if busy2 {
			return fmt.Errorf("could not free port %d; still in use", p)
		}
		info("[ports] Freed port %d", p)
	}
	return nil
}

func isPortBusy(port int) (bool, string) {
	// Try connecting; if succeeds, someone is listening.
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
	if err == nil {
		_ = conn.Close()
		return true, "tcp listener detected"
	}
	return false, ""
}

func waitHTTP(url string, want int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	client := &http.Client{ Timeout: 2 * time.Second }
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == want {
				return nil
			}
		}
		select {
		case <-time.After(1 * time.Second):
		case <-ctx.Done():
			return fmt.Errorf("timed out waiting for %s to return %d", url, want)
		}
	}
}

func firstGGUF(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil { return "", err }
	for _, e := range entries {
		if e.IsDir() { continue }
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".gguf") {
			return name, nil
		}
	}
	return "", fmt.Errorf("no .gguf models found in %s", dir)
}

func hasHostModels() bool {
	dir := filepath.Join(homeDir(), "models", "llm")
	entries, err := os.ReadDir(dir)
	if err != nil { return false }
	for _, e := range entries {
		if e.IsDir() { continue }
		if strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			return true
		}
	}
	return false
}

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" { return h }
	h, _ := os.UserHomeDir()
	return h
}

func killProcesses() error { return nil }

// Command helpers
func runCmdVerbose(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdStreaming(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil { return err }
	go stream("OUT", stdout)
	go stream("ERR", stderr)
	return cmd.Wait()
}

func runEnvCmdStreaming(ctx context.Context, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	for k, v := range env { cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v)) }
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stream(prefix string, r ioReader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		fmt.Println(s.Text())
	}
}

type ioReader interface { Read(p []byte) (n int, err error) }

// Logging
func info(format string, a ...any) { fmt.Println(fmt.Sprintf(format, a...)) }
func warn(format string, a ...any) { fmt.Println(fmt.Sprintf(format, a...)) }
func die(format string, a ...any)  { fmt.Fprintln(os.Stderr, fmt.Sprintf(format, a...)); os.Exit(2) }
func must(err error)               { if err != nil { fmt.Fprintln(os.Stderr, err.Error()); os.Exit(1) } }

// Env helpers
func envStr(key, def string) string { if v := os.Getenv(key); v != "" { return v }; return def }
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" { return def }
	s := strings.ToLower(v)
	return s == "1" || s == "true" || s == "yes"
}
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		_, err := fmt.Sscanf(v, "%d", &n)
		if err == nil { return n }
	}
	return def
}
