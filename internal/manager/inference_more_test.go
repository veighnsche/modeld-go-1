package manager

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"

	"modeld/pkg/types"
)

// fakeAdapter is a lightweight in-memory adapter used for tests.
type fakeAdapter struct {
	startErr   error
	genErr     error
	tokens     []string
	final      FinalResult
	receivedMP string
}

func TestInfer_AdapterStartError(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m"})
	m.RealInferEnabled = true
	m.adapter = &fakeAdapter{startErr: errors.New("boom")}
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Model: "m", Prompt: "p", Stream: true}, &buf, nil)
	if err == nil {
		t.Fatalf("expected error from Start")
	}
}

func TestInfer_AdapterGenerateError(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m"})
	m.RealInferEnabled = true
	m.adapter = &fakeAdapter{tokens: []string{"a"}, genErr: errors.New("gen")}
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Model: "m", Prompt: "p", Stream: true}, &buf, nil)
	if err == nil {
		t.Fatalf("expected generate error")
	}
}

func TestInfer_RealEnabled_ModelNotFound(t *testing.T) {
	// Empty registry ensures model not found when specifying unknown model
	m := NewWithConfig(ManagerConfig{DefaultModel: ""})
	m.RealInferEnabled = true
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Model: "missing", Prompt: "p", Stream: true}, &buf, nil)
	if err == nil || !IsModelNotFound(err) {
		t.Fatalf("expected model not found, got %v", err)
	}
}

type errWriter struct{ wrote int }

func (e *errWriter) Write(p []byte) (int, error) {
	if e.wrote == 0 {
		e.wrote += len(p)
		return len(p), nil
	}
	return 0, errors.New("write fail")
}

func TestInfer_TokenWriteErrorStops(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m"})
	m.RealInferEnabled = true
	m.adapter = &fakeAdapter{tokens: []string{"a", "b"}}
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	ew := &errWriter{}
	err := m.Infer(context.Background(), types.InferRequest{Model: "m", Prompt: "p", Stream: true}, ew, nil)
	if err == nil {
		t.Fatalf("expected write error")
	}
}

func (f *fakeAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	f.receivedMP = modelPath
	if f.startErr != nil {
		return nil, f.startErr
	}
	return fakeSession{f: f}, nil
}

type fakeSession struct{ f *fakeAdapter }

func (s fakeSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	if s.f.genErr != nil {
		return FinalResult{}, s.f.genErr
	}
	for _, t := range s.f.tokens {
		select {
		case <-ctx.Done():
			return FinalResult{}, ctx.Err()
		default:
		}
		if err := onToken(t); err != nil {
			return FinalResult{}, err
		}
	}
	return s.f.final, nil
}

func (s fakeSession) Close() error { return nil }

func TestInfer_DependencyUnavailableWhenRealEnabledNoAdapter(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m"})
	m.RealInferEnabled = true
	// No adapter set
	var buf bytes.Buffer
	err := m.Infer(context.Background(), types.InferRequest{Prompt: "hi", Stream: true}, &buf, nil)
	if err == nil || !IsDependencyUnavailable(err) {
		t.Fatalf("expected dependency unavailable, got %v", err)
	}
}

func TestInfer_WithAdapterStreamsAndFinal(t *testing.T) {
	dir := t.TempDir()
	p := createModelFile(t, dir, "m.bin", 1)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: p}}, DefaultModel: "m"})
	m.RealInferEnabled = true
	fa := &fakeAdapter{
		tokens: []string{"he", "llo"},
		final:  FinalResult{Content: "", FinishReason: "stop", Usage: Usage{}},
	}
	m.adapter = fa
	if err := m.EnsureInstance(context.Background(), "m"); err != nil {
		t.Fatalf("ensure: %v", err)
	}
	var buf bytes.Buffer
	flushed := 0
	flusher := func() { flushed++ }
	if err := m.Infer(context.Background(), types.InferRequest{Model: "m", Prompt: "ignored", Stream: true}, &buf, flusher); err != nil {
		t.Fatalf("infer: %v", err)
	}
	// The output should be N token lines + final line, all newline-terminated
	lines := 0
	for _, b := range buf.Bytes() {
		if b == '\n' {
			lines++
		}
	}
	if lines != len(fa.tokens)+1 {
		t.Fatalf("expected %d lines, got %d", len(fa.tokens)+1, lines)
	}
	if flushed == 0 {
		t.Fatalf("expected flusher to be called")
	}
	// Verify the last line is a JSON object with done=true and content concatenation
	parts := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var end struct {
		Done    bool        `json:"done"`
		Content string      `json:"content"`
		Usage   interface{} `json:"usage"`
	}
	if err := json.Unmarshal(parts[len(parts)-1], &end); err != nil {
		t.Fatalf("unmarshal end: %v", err)
	}
	if !end.Done {
		t.Fatalf("expected done=true")
	}
	if end.Content != "hello" { // concatenated tokens
		t.Fatalf("unexpected content: %q", end.Content)
	}
}

func TestTokenLineJSON_EscapesAndNewline(t *testing.T) {
	in := "a\"b"
	b := tokenLineJSON(in)
	if len(b) == 0 || b[len(b)-1] != '\n' {
		t.Fatalf("expected trailing newline")
	}
	var obj struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(b[:len(b)-1], &obj); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if obj.Token != in {
		t.Fatalf("roundtrip mismatch: %q", obj.Token)
	}
}

func TestEnsureInstance_CanceledDuringWarmup_SetsErrorState(t *testing.T) {
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "nonexistent"}}, DefaultModel: "m"})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := m.EnsureInstance(ctx, "m")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
	snap := m.Snapshot()
	if snap.State != StateError || snap.Err == "" {
		t.Fatalf("expected error state and non-empty err, got %+v", snap)
	}
}

func TestEstimateVRAMMB_StatErrorReturnsMinimum(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	if mb := m.estimateVRAMMB(types.Model{Path: "/path/does/not/exist"}); mb != 1 {
		t.Fatalf("expected minimum 1MB on stat error, got %d", mb)
	}
}

func TestIsDependencyUnavailable(t *testing.T) {
	err := ErrDependencyUnavailable("x")
	if !IsDependencyUnavailable(err) {
		t.Fatalf("expected IsDependencyUnavailable true")
	}
}
