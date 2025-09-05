package manager

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"modeld/pkg/types"
)

// ensureLlamaRuntime ensures a llama-server is running for the given instance.
// It will start the process bound to 127.0.0.1 on a free port and perform a health check.
func (m *Manager) ensureLlamaRuntime(ctx context.Context, inst *Instance, mdl types.Model) error {
	if inst == nil {
		return errors.New("nil instance")
	}
	// If a port is already assigned and health check passes, we're done.
	if inst.Port != 0 {
		if m.checkHealth(ctx, inst.Port) == nil {
			return nil
		}
		// else fall through to restart
	}
	if strings.TrimSpace(m.LlamaBin) == "" {
		return errors.New("llama binary path is not configured")
	}
	modelPath := strings.TrimSpace(mdl.Path)
	if modelPath == "" {
		return fmt.Errorf("model %s has empty path", mdl.ID)
	}
	// Choose a free port and start server
	port, err := findFreePort()
	if err != nil {
		return err
	}
	proc, err := m.startLlamaServer(ctx, port, modelPath)
	if err != nil {
		return err
	}
	// Wait for health
	if err := waitForHealth(ctx, port, 15*time.Second); err != nil {
		_ = proc.Kill() // best-effort
		return err
	}
	inst.Port = port
	inst.Proc = proc
	return nil
}

// stopInstanceRuntime best-effort terminates the managed runtime process.
func (m *Manager) stopInstanceRuntime(inst *Instance) {
	if inst == nil || inst.Proc == nil {
		return
	}
	if p, ok := inst.Proc.(*os.Process); ok {
		_ = p.Kill()
	}
	inst.Proc = nil
	inst.Port = 0
}

// startLlamaServer launches the llama-server process.
func (m *Manager) startLlamaServer(ctx context.Context, port int, modelPath string) (*os.Process, error) {
	args := []string{
		"--server",
		"--host", "127.0.0.1",
		"--port", fmt.Sprintf("%d", port),
		"-m", modelPath,
	}
	if m.LlamaCtx > 0 {
		args = append(args, "--ctx-size", fmt.Sprintf("%d", m.LlamaCtx))
	}
	if m.LlamaThreads > 0 {
		args = append(args, "--threads", fmt.Sprintf("%d", m.LlamaThreads))
	}
	cmd := exec.CommandContext(ctx, m.LlamaBin, args...)
	// Ensure working directory is the model directory so relative assets resolve
	cmd.Dir = filepath.Dir(modelPath)
	// Pipe stdout/stderr to help debugging but don't spam logs; keep in-memory reader
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	// Background log readers to avoid blocking due to full buffers
	go drain("llama-server[stdout]", stdout)
	go drain("llama-server[stderr]", stderr)
	return cmd.Process, nil
}

func drain(prefix string, r io.Reader) {
	s := bufio.NewScanner(r)
	s.Buffer(make([]byte, 0, 64*1024), 2*1024*1024)
	for s.Scan() {
		_ = s.Text() // ignore for now; hook to logger if needed
	}
}

func findFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

func waitForHealth(ctx context.Context, port int, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		if err := checkHealth(ctx, port); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("llama-server health check timeout on :%d: %w", port, ctx.Err())
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func (m *Manager) checkHealth(ctx context.Context, port int) error {
	return checkHealth(ctx, port)
}

func checkHealth(ctx context.Context, port int) error {
	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("health status %d", resp.StatusCode)
	}
	return nil
}
