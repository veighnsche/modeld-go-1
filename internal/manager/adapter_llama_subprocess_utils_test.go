//go:build integration
// +build integration

package manager

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestIsHealthyTrueAndFalse(t *testing.T) {
	tsOK := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" { w.WriteHeader(200); return }
		w.WriteHeader(404)
	}))
	defer tsOK.Close()
	ad := &llamaSubprocessAdapter{httpClient: &http.Client{Timeout: 0}}
	if !ad.isHealthy(tsOK.URL, 500*time.Millisecond) {
		t.Fatalf("expected healthy for %s", tsOK.URL)
	}
	// Unreachable URL should be unhealthy
	if ad.isHealthy("http://127.0.0.1:1", 100*time.Millisecond) {
		t.Fatalf("expected unhealthy for unreachable host")
	}
}

func TestPickPortInRangeAvoidsBusy(t *testing.T) {
	// Find two free ports, bind one, and ensure pickPortInRange returns the other
	host := "127.0.0.1"
	l1, err := net.Listen("tcp", host+":0")
	if err != nil { t.Fatalf("listen1: %v", err) }
	p1 := l1.Addr().(*net.TCPAddr).Port
	l2, err := net.Listen("tcp", host+":0")
	if err != nil { t.Fatalf("listen2: %v", err) }
	p2 := l2.Addr().(*net.TCPAddr).Port
	// Reserve lower port as busy
	busy, free := p1, p2
	if p2 < p1 { busy, free = p2, p1 }
	defer l1.Close()
	defer l2.Close()
	// Ask for range including both; since both are currently bound, close the free one first
	l2.Close()
	picked, err := pickPortInRange(host, busy, free)
	if err != nil { t.Fatalf("pickPortInRange error: %v", err) }
	if picked != free {
		t.Fatalf("expected pick %d, got %d (busy=%d)", free, picked, busy)
	}
}
