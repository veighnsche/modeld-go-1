package manager

import (
	"testing"
)

func TestGetProcInfo_EmptyAndPresent(t *testing.T) {
	a := &llamaSubprocessAdapter{}
	if pid, base, ready, ok := a.getProcInfo("missing"); ok || pid != 0 || base != "" || ready {
		t.Fatalf("expected empty snapshot, got pid=%d base=%q ready=%v ok=%v", pid, base, ready, ok)
	}
	a.procs = map[string]*procInfo{"m.gguf": {pid: 123, baseURL: "http://127.0.0.1:9999", ready: true}}
	if pid, base, ready, ok := a.getProcInfo("m.gguf"); !ok || pid != 123 || base == "" || !ready {
		t.Fatalf("unexpected snapshot: pid=%d base=%q ready=%v ok=%v", pid, base, ready, ok)
	}
}
