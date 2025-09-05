package testctl

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// chooseFreePort finds an available TCP port by asking the kernel for :0
func chooseFreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil { return 0, err }
	defer l.Close()
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
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
