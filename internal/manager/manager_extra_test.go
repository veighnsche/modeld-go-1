package manager

import (
	"context"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestSwitchReturnsOpID(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	op, err := m.Switch(context.Background(), "m1")
	if err != nil { t.Fatalf("Switch err: %v", err) }
	if op == "" || op[:3] != "op-" { t.Fatalf("unexpected op id: %q", op) }
}

func TestEnsureInstance_FastPathNoDoubleCount(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 2)
	reg := []types.Model{{ID: "m", Path: p}}
	m := NewWithConfig(ManagerConfig{Registry: reg, DefaultModel: "m"})
	if err := m.EnsureInstance(context.Background(), "m"); err != nil { t.Fatalf("ensure: %v", err) }
	m.mu.RLock(); inst := m.instances["m"]; used := m.usedEstMB; last := inst.LastUsed; m.mu.RUnlock()
	time.Sleep(5 * time.Millisecond)
	if err := m.EnsureInstance(context.Background(), "m"); err != nil { t.Fatalf("ensure fast: %v", err) }
	m.mu.RLock(); inst2 := m.instances["m"]; used2 := m.usedEstMB; last2 := inst2.LastUsed; m.mu.RUnlock()
	if used2 != used { t.Fatalf("usedEstMB changed on fast path: %d -> %d", used, used2) }
	if !last2.After(last) { t.Fatalf("LastUsed not updated on fast path") }
}

func TestEnsureModel_NoChangeWhenSame(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	if err := m.EnsureModel(context.Background(), "m1"); err != nil { t.Fatalf("ensure model: %v", err) }
	if err := m.EnsureModel(context.Background(), "m1"); err != nil { t.Fatalf("ensure same: %v", err) }
	snap := m.Snapshot()
	if snap.State != StateReady || snap.CurrentModel == nil || snap.CurrentModel.ID != "m1" { t.Fatalf("unexpected snap: %+v", snap) }
}

func TestEnsureModel_ContextCanceledSetsError(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := m.EnsureModel(ctx, "m2"); err == nil { t.Fatalf("expected error on canceled ctx") }
	snap := m.Snapshot()
	if snap.State != StateError || snap.Err == "" { t.Fatalf("expected error state, got %+v", snap) }
}

func TestEvictUntilFits_NoIdleInstances(t *testing.T) {
	dir := t.TempDir()
	p1 := createModelFile(t, dir, "a.bin", 10)
	p2 := createModelFile(t, dir, "b.bin", 10)
	reg := []types.Model{{ID: "a", Path: p1}, {ID: "b", Path: p2}}
	// Budget 25 allows both (10+10=20) to be loaded, but a subsequent request of 10 would not fit (20+10=30 > 25)
	m := NewWithConfig(ManagerConfig{Registry: reg, BudgetMB: 25, MarginMB: 0, MaxQueueDepth: 1})
	if err := m.EnsureInstance(context.Background(), "a"); err != nil { t.Fatalf("ensure a: %v", err) }
	if err := m.EnsureInstance(context.Background(), "b"); err != nil { t.Fatalf("ensure b: %v", err) }
	// Occupy both instances so they are not idle
	m.mu.RLock(); ia := m.instances["a"]; ib := m.instances["b"]; m.mu.RUnlock()
	ia.queueCh <- struct{}{}; ia.genCh <- struct{}{}
	ib.queueCh <- struct{}{}; ib.genCh <- struct{}{}
	usedBefore := m.usedEstMB
	// Request that would require more space; function should return without evicting due to no idle
	_ = m.evictUntilFits(20)
	m.mu.RLock(); _, hasA := m.instances["a"]; _, hasB := m.instances["b"]; usedAfter := m.usedEstMB; m.mu.RUnlock()
	if !hasA || !hasB { t.Fatalf("instances should remain when no idle") }
	if usedAfter != usedBefore { t.Fatalf("usedEstMB changed: %d -> %d", usedBefore, usedAfter) }
	// cleanup channels
	<-ia.genCh; <-ia.queueCh; <-ib.genCh; <-ib.queueCh
}

func TestGetModelByID(t *testing.T) {
	reg := []types.Model{{ID: "a"}, {ID: "b"}}
	m := NewWithConfig(ManagerConfig{Registry: reg})
	if mdl, ok := m.getModelByID("b"); !ok || mdl.ID != "b" { t.Fatalf("expected to find model b, got %+v ok=%v", mdl, ok) }
	if _, ok := m.getModelByID("z"); ok { t.Fatalf("expected not found for z") }
}

func TestBeginGenerationHappyPath(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	reg := []types.Model{{ID: "m", Path: p}}
	m := NewWithConfig(ManagerConfig{Registry: reg, DefaultModel: "m", MaxQueueDepth: 1, MaxWait: 50 * time.Millisecond})

	if err := m.EnsureInstance(context.Background(), "m"); err != nil { t.Fatalf("ensure: %v", err) }

	rel, err := m.beginGeneration(context.Background(), "m")
	if err != nil { t.Fatalf("beginGeneration: %v", err) }
	// While one generation is in-flight, a second should time out with tooBusy
	m.maxWait = 10 * time.Millisecond
	if _, err2 := m.beginGeneration(context.Background(), "m"); err2 == nil || !IsTooBusy(err2) {
		t.Fatalf("expected too busy on second begin, got %v", err2)
	}
	// Release the first generation; subsequent begin should succeed again
	rel()
	m.maxWait = 50 * time.Millisecond
	if rel2, err3 := m.beginGeneration(context.Background(), "m"); err3 != nil {
		t.Fatalf("beginGeneration after release: %v", err3)
	} else {
		rel2()
	}
}
