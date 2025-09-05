package testctl

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChooseFreePort(t *testing.T) {
	p, err := chooseFreePort()
	if err != nil {
		t.Fatalf("chooseFreePort: %v", err)
	}
	if p <= 0 {
		t.Fatalf("invalid port: %d", p)
	}
}

func TestIsPortBusy(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	busy, _ := isPortBusy(port)
	if !busy {
		t.Fatalf("expected port busy for %d", port)
	}
	// pick an unbound port
	free, err := chooseFreePort()
	if err != nil {
		t.Fatal(err)
	}
	// close it immediately so it becomes free
	busy, _ = isPortBusy(free)
	if busy {
		t.Fatalf("expected port %d to be free", free)
	}
}

func TestWaitHTTP(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	defer ts.Close()
	if err := waitHTTP(ts.URL, 200, 3*time.Second); err != nil {
		t.Fatalf("waitHTTP: %v", err)
	}
}

func TestEnsurePorts(t *testing.T) {
	// Free port case
	p, err := chooseFreePort()
	if err != nil {
		t.Fatal(err)
	}
	if err := ensurePorts([]int{p}, false); err != nil {
		t.Fatalf("ensurePorts free: %v", err)
	}
	// Busy port without force should error
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if err := ensurePorts([]int{port}, false); err == nil {
		t.Fatalf("expected error for busy port without force")
	}
}
