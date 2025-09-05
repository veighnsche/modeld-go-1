package manager

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"
)

// SanityReport describes runtime checks for external dependencies.
type SanityReport struct {
	LlamaFound bool   `json:"llama_found"`
	LlamaPath  string `json:"llama_path,omitempty"`
	Error      string `json:"error,omitempty"`
}

// SanityCheck validates that required dependencies are available for inference.
// It does not mutate state and is safe to call at any time.
func (m *Manager) SanityCheck() SanityReport {
	r := SanityReport{}
	// Adapter must be initialized for inference
	if m.adapter == nil {
		r.LlamaFound = false
		r.Error = "llama adapter not initialized"
		return r
	}
	// For in-process adapter, optionally verify default model file exists
	r.LlamaFound = true
	if m.defaultModel != "" {
		if mdl, ok := m.getModelByID(m.defaultModel); ok && mdl.Path != "" {
			// Skip filesystem checks when using server adapter
			if _, ok := m.adapter.(*llamaServerAdapter); !ok {
				if fi, err := os.Stat(mdl.Path); err == nil && !fi.IsDir() {
					r.LlamaPath = mdl.Path
				}
			}
		}
	}
	return r
}

// Check represents one preflight check item.
type Check struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

// Preflight runs a series of non-destructive checks to validate that inference can run.
// It focuses on:
// - Adapter presence
// - Default model configured
// - For in-process mode: default model path exists and is a file
// Note: it does not attempt to load the model (avoids heavy I/O at startup).
func (m *Manager) Preflight() []Check {
	var checks []Check
	// Adapter presence
	if m.adapter == nil {
		checks = append(checks, Check{Name: "adapter_present", OK: false, Message: "llama adapter not initialized"})
	} else {
		checks = append(checks, Check{Name: "adapter_present", OK: true})
	}
	// In server mode, probe reachability and capture available models
	var serverModels map[string]struct{}
	if sa, ok := m.adapter.(*llamaServerAdapter); ok {
		// compute a reasonable timeout
		to := 5 * time.Second
		if sa.reqTimeout > 0 {
			to = sa.reqTimeout
			if to > 10*time.Second {
				to = 10 * time.Second
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), to)
		defer cancel()
		url := sa.baseURL + "/v1/models"
		req, err := httpNewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			checks = append(checks, Check{Name: "server_reachable", OK: false, Message: err.Error()})
		} else {
			if sa.apiKey != "" {
				req.Header.Set("Authorization", "Bearer "+sa.apiKey)
			}
			resp, err := sa.httpClient.Do(req)
			if err != nil {
				checks = append(checks, Check{Name: "server_reachable", OK: false, Message: err.Error()})
			} else {
				defer resp.Body.Close()
				if resp.StatusCode >= 200 && resp.StatusCode < 300 {
					checks = append(checks, Check{Name: "server_reachable", OK: true})
					// parse models list (OpenAI style: {data:[{id:...}]}, or fallback {models:[{id:...}]})
					var data struct{ Data []struct{ ID string `json:"id"` } `json:"data"` }
					if err := json.NewDecoder(resp.Body).Decode(&data); err == nil && len(data.Data) > 0 {
						serverModels = make(map[string]struct{}, len(data.Data))
						for _, m := range data.Data { serverModels[m.ID] = struct{}{} }
					} else {
						// rewind not possible; a second GET would be heavy; best effort
						// no-op: model existence check may be skipped below if unknown
					}
				} else {
					checks = append(checks, Check{Name: "server_reachable", OK: false, Message: resp.Status})
				}
			}
		}
	}
	// Default model configured
	if m.defaultModel == "" {
		checks = append(checks, Check{Name: "default_model_configured", OK: false, Message: "no default model configured"})
	} else {
		checks = append(checks, Check{Name: "default_model_configured", OK: true})
		// Resolve model id in registry
		mdl, ok := m.getModelByID(m.defaultModel)
		if !ok {
			checks = append(checks, Check{Name: "default_model_in_registry", OK: false, Message: "default model id not found in registry"})
		} else {
			checks = append(checks, Check{Name: "default_model_in_registry", OK: true})
			if _, isServer := m.adapter.(*llamaServerAdapter); !isServer {
				if mdl.Path == "" {
					checks = append(checks, Check{Name: "default_model_path_set", OK: false, Message: "default model path is empty"})
				} else if fi, err := os.Stat(mdl.Path); err != nil {
					checks = append(checks, Check{Name: "default_model_path_exists", OK: false, Message: err.Error()})
				} else if fi.IsDir() {
					checks = append(checks, Check{Name: "default_model_path_is_file", OK: false, Message: "path points to a directory, expected a .gguf file"})
				} else {
					checks = append(checks, Check{Name: "default_model_path_exists", OK: true})
					checks = append(checks, Check{Name: "default_model_path_is_file", OK: true})
				}
			} else {
				// server mode: if we had a models list, verify the default model id is present
				if serverModels != nil {
					if _, ok := serverModels[m.defaultModel]; ok {
						checks = append(checks, Check{Name: "default_model_available_on_server", OK: true})
					} else {
						checks = append(checks, Check{Name: "default_model_available_on_server", OK: false, Message: "model id not listed by server"})
					}
				}
			}
		}
	}
	return checks
}

// httpNewRequestWithContext is a small indirection to allow testing/mocking if needed.
var httpNewRequestWithContext = func(ctx context.Context, method, url string, body io.Reader) (*http.Request, error) {
	return http.NewRequestWithContext(ctx, method, url, body)
}
