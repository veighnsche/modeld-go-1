package manager

import (
	"bytes"
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestNewWithConfigDefaults(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	if m.maxQueueDepth != defaultMaxQueueDepth {
		t.Fatalf("expected default maxQueueDepth=%d got %d", defaultMaxQueueDepth, m.maxQueueDepth)
	}
	if m.maxWait != defaultMaxWait {
		t.Fatalf("expected default maxWait=%v got %v", defaultMaxWait, m.maxWait)
	}
}

func TestListModelsReturnsCopy(t *testing.T) {
	reg := []types.Model{{ID: "a"}, {ID: "b"}}
	m := NewWithConfig(ManagerConfig{Registry: reg})
	out := m.ListModels()
	if len(out) != 2 {
		t.Fatalf("expected 2 got %d", len(out))
	}
	// mutate returned slice and ensure internal registry remains intact
	out[0].ID = "z"
	out2 := m.ListModels()
	if out2[0].ID != "a" {
		t.Fatalf("registry mutated via returned slice")
	}
}

func TestReadyReflectsInstance(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m1.bin", 1)
	reg := []types.Model{{ID: "m1", Path: p}}
	m := NewWithConfig(ManagerConfig{Registry: reg, DefaultModel: "m1"})
	if m.Ready() {
		t.Fatalf("expected not ready initially")
	}
	ctx := context.Background()
	if err := m.EnsureInstance(ctx, "m1"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	if !m.Ready() {
		t.Fatalf("expected ready after ensure")
	}
}

func TestEnsureInstance_ModelNotFound(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	err := m.EnsureInstance(context.Background(), "missing")
	if err == nil || !IsModelNotFound(err) {
		t.Fatalf("expected model not found error, got %v", err)
	}
}

func TestEstimateVRAMMBUsesFileSize(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m1.bin", 2)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m1", Path: p}}})
	if mb := m.estimateVRAMMB(types.Model{Path: p}); mb < 2 {
		t.Fatalf("expected >=2MB, got %d", mb)
	}
}

func TestEvictionLRUUntilFits(t *testing.T) {
	// budget that will require evicting an older instance
	dir := t.TempDir()
	p1 := createModelFile(t, dir, "a.bin", 10)
	p2 := createModelFile(t, dir, "b.bin", 10)
	p3 := createModelFile(t, dir, "c.bin", 15)

	reg := []types.Model{{ID: "a", Path: p1}, {ID: "b", Path: p2}, {ID: "c", Path: p3}}
	m := NewWithConfig(ManagerConfig{Registry: reg, BudgetMB: 30, MarginMB: 0})

	// seed two ready instances: a (older), b (newer)
	if err := m.EnsureInstance(context.Background(), "a"); err != nil {
		t.Fatalf("ensure a: %v", err)
	}
	// make a older
	time.Sleep(5 * time.Millisecond)
	if err := m.EnsureInstance(context.Background(), "b"); err != nil {
		t.Fatalf("ensure b: %v", err)
	}

	// now require c (15MB). used ~ 10+10=20; adding 15 would exceed 30, so must evict LRU (a)
	if err := m.EnsureInstance(context.Background(), "c"); err != nil {
		t.Fatalf("ensure c: %v", err)
	}

	m.mu.RLock()
	_, hasA := m.instances["a"]
	_, hasB := m.instances["b"]
	_, hasC := m.instances["c"]
	used := m.usedEstMB
	m.mu.RUnlock()

	if hasA {
		t.Fatalf("expected instance 'a' evicted")
	}
	if !hasB || !hasC {
		t.Fatalf("expected instances 'b' and 'c' present")
	}
	// used should be close to 10 (b) + 15 (c) = 25; allow >=25 for conservative rounding
	if used < 25 {
		t.Fatalf("expected used >= 25, got %d", used)
	}
}

func TestBeginGenerationBackpressureTooBusy(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	reg := []types.Model{{ID: "m", Path: p}}
	m := NewWithConfig(ManagerConfig{Registry: reg, DefaultModel: "m", MaxQueueDepth: 1, MaxWait: 10 * time.Millisecond})

	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	// Saturate queue and gen to force backpressure
	m.mu.RLock()
	inst := m.instances["m"]
	m.mu.RUnlock()
	inst.queueCh <- struct{}{}
	inst.genCh <- struct{}{}

	// call Infer which uses beginGeneration under the hood
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Model: "m", Prompt: "hi", Stream: true}, &buf, func() {})
	if err == nil || !IsTooBusy(err) {
		t.Fatalf("expected too busy error, got %v", err)
	}
	// cleanup
	<-inst.genCh
	<-inst.queueCh
}

func TestInferNoDefaultModelError(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Prompt: "hi", Stream: true}, &buf, nil)
	if err == nil || !IsModelNotFound(err) {
		t.Fatalf("expected model not found for unspecified model without default, got %v", err)
	}
}

func TestStatusAndSnapshot(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	reg := []types.Model{{ID: "m", Path: p}}
	m := NewWithConfig(ManagerConfig{Registry: reg, DefaultModel: "m", BudgetMB: 100, MarginMB: 5})

	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}

	snap := m.Snapshot()
	if snap.State != StateReady || snap.CurrentModel == nil || snap.CurrentModel.ID != "m" {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	st := m.Status()
	if st.BudgetMB != 100 || st.MarginMB != 5 {
		t.Fatalf("unexpected status budget/margin: %+v", st)
	}
	if len(st.Instances) != 1 || st.Instances[0].ModelID != "m" {
		t.Fatalf("unexpected instances in status: %+v", st.Instances)
	}
}
