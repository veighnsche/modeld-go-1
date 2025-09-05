package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"modeld/internal/manager"
)

// TestSpawnMode_Haiku prints a real haiku by spawning llama.cpp per model via the subprocess adapter.
// Skips unless:
// - LLAMA_BIN points to a llama-server binary, and
// - ~/models/llm contains at least one real .gguf file.
func TestSpawnMode_Haiku(t *testing.T) {
	home, _ := os.UserHomeDir()
	modelsDir := filepath.Join(home, "models", "llm")
	ents, _ := os.ReadDir(modelsDir)
	var modelID string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(strings.ToLower(e.Name()), ".gguf") {
			modelID = e.Name()
			break
		}
	}
	if modelID == "" {
		t.Skip("no GGUF found under ~/models/llm; skipping spawn-mode haiku test")
	}
	llamaBin := strings.TrimSpace(os.Getenv("LLAMA_BIN"))
	if llamaBin == "" {
		t.Skip("LLAMA_BIN not set; skipping spawn-mode haiku test")
	}

	cfg := manager.ManagerConfig{
		BudgetMB:            0,
		MarginMB:            0,
		DefaultModel:        modelID,
		MaxQueueDepth:       2,
		MaxWait:             10 * time.Second,
		SpawnLlama:          true,
		LlamaBin:            llamaBin,
		LlamaHost:           "127.0.0.1",
		LlamaThreads:        0,
		LlamaCtxSize:        2048,
		LlamaNGL:            0,
		LlamaRequestTimeout: 45 * time.Second,
		LlamaConnectTimeout: 5 * time.Second,
		LlamaUseOpenAI:      true,
	}
	srv, _ := newServerForDirWithConfig(t, modelsDir, cfg)

	prompt := "Write a 3-line haiku about the ocean."
	resp, body := httpPostJSON(t, srv.URL+"/infer", []byte("{"+
		"\"prompt\":"+jsonString(prompt)+","+
		"\"max_tokens\":128,"+
		"\"temperature\":0.7,"+
		"\"top_p\":0.95"+
		"}"))
	if resp.StatusCode != 200 {
		t.Fatalf("/infer status=%d body=%s", resp.StatusCode, string(body))
	}

	// Parse NDJSON: collect tokens and/or final content
	lines := strings.Split(string(body), "\n")
	var tokens []string
	final := ""
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" { continue }
		var m map[string]any
		if err := json.Unmarshal([]byte(ln), &m); err != nil { continue }
		if tok, ok := m["token"].(string); ok && tok != "" { tokens = append(tokens, tok) }
		if done, _ := m["done"].(bool); done {
			if c, ok := m["content"].(string); ok { final = c }
		}
	}
	content := strings.TrimSpace(func() string {
		if final != "" { return final }
		return strings.Join(tokens, "")
	}())
	if content == "" {
		t.Fatalf("expected non-empty haiku content")
	}
	t.Logf("\n----- GENERATED HAIKU (spawn mode) -----\n%s\n----------------------------------------\n", content)
}

// jsonString escapes a string for embedding inside a JSON literal we build manually.
func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
