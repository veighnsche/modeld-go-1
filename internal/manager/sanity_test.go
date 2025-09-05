package manager

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"modeld/pkg/types"
)

func TestSanityCheck_NoAdapter(t *testing.T) {
	m := NewWithConfig(ManagerConfig{})
	r := m.SanityCheck()
	if r.LlamaFound || r.Error == "" {
		t.Fatalf("expected no adapter error, got %+v", r)
	}
}

func TestSanityCheck_ServerAdapterSkipsFS(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer ts.Close()
	cfg := ManagerConfig{Registry: []types.Model{{ID: "m", Path: "/does/not/exist.gguf"}}, DefaultModel: "m"}
	ad := NewLlamaServerAdapter(ts.URL, "", true, 1*time.Second, 1*time.Second)
	m := NewWithConfig(cfg)
	m.SetInferenceAdapter(ad)
	r := m.SanityCheck()
	if !r.LlamaFound {
		t.Fatalf("expected LlamaFound true, got %+v", r)
	}
}

func TestPreflight_ServerReachable_ListsModel(t *testing.T) {
	// Fake server that returns a model id list including default model
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"data":[{"id":"m"}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer ts.Close()
	ad := NewLlamaServerAdapter(ts.URL, "", true, 1*time.Second, 1*time.Second)
	m := NewWithConfig(ManagerConfig{Registry: []types.Model{{ID: "m", Path: "m.gguf"}}, DefaultModel: "m"})
	m.SetInferenceAdapter(ad)
	checks := m.Preflight()
	var serverOK, modelOK bool
	for _, c := range checks {
		if c.Name == "server_reachable" && c.OK { serverOK = true }
		if c.Name == "default_model_available_on_server" && c.OK { modelOK = true }
	}
	if !serverOK || !modelOK {
		t.Fatalf("expected server reachable and model present; got: %+v", checks)
	}
}
