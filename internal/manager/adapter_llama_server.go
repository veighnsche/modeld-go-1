package manager

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// llamaServerAdapter implements InferenceAdapter by talking to a running llama.cpp server over HTTP.
// It prefers OpenAI-compatible endpoints and falls back to native endpoints when necessary.
type llamaServerAdapter struct {
	baseURL          string
	apiKey           string
	useOpenAI        bool
	reqTimeout       time.Duration
	connectTimeout   time.Duration
	httpClient       *http.Client
	baseParams       InferParams
}

// NewLlamaServerAdapter constructs a server-backed adapter.
func NewLlamaServerAdapter(baseURL, apiKey string, useOpenAI bool, reqTimeout, connectTimeout time.Duration) InferenceAdapter {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout: connectTimeout,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	// Intentionally set Timeout=0 here: all requests must carry context-based timeouts.
    // See Generate() which applies reqTimeout via context, and individual calls use
    // http.NewRequestWithContext to enforce deadlines.
    cli := &http.Client{Transport: tr, Timeout: 0}
	return &llamaServerAdapter{
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		useOpenAI:      useOpenAI,
		reqTimeout:     reqTimeout,
		connectTimeout: connectTimeout,
		httpClient:     cli,
	}
}

// llamaServerSession holds per-session state (mostly the base params).
type llamaServerSession struct {
	adapter    *llamaServerAdapter
	modelID    string // selected model if provided
	baseParams InferParams
}

func (a *llamaServerAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
	// In server mode, model selection is conveyed by name/id to the server; we do not use on-disk path.
	return &llamaServerSession{
		adapter:    a,
		modelID:    strings.TrimSpace(modelPath), // we pass it as model field when available
		baseParams: params,
	}, nil
}

// openAICompletionRequest represents the payload for /v1/completions.
type openAICompletionRequest struct {
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt"`
	MaxTokens   int      `json:"max_tokens,omitempty"`
	Temperature float32  `json:"temperature,omitempty"`
	TopP        float32  `json:"top_p,omitempty"`
	TopK        int      `json:"top_k,omitempty"`
	Stop        []string `json:"stop,omitempty"`
	Seed        int      `json:"seed,omitempty"`
	Stream      bool     `json:"stream"`
	// RepeatPenalty is not standard OpenAI; some llama.cpp builds accept it under different names.
	// We include it using the common key if present; servers that ignore it will safely ignore.
	RepeatPenalty float32 `json:"repeat_penalty,omitempty"`
}

// openAIStreamChoiceDelta is a minimal subset of OpenAI streaming response.
type openAIStreamChoiceDelta struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type openAIStreamResponse struct {
	Object  string                    `json:"object"`
	Choices []openAIStreamChoiceDelta `json:"choices"`
}

func (s *llamaServerSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	if s.adapter == nil || s.adapter.httpClient == nil {
		return FinalResult{}, errors.New("llama server adapter not initialized")
	}
	// Apply request timeout via context, if configured
	if s.adapter.reqTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, s.adapter.reqTimeout)
		defer cancel()
	}

	if s.adapter.useOpenAI {
		return s.generateOpenAI(ctx, prompt, onToken)
	}
	// Fallback to native if desired; for initial implementation reuse OpenAI path.
	return s.generateOpenAI(ctx, prompt, onToken)
}

func (s *llamaServerSession) generateOpenAI(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
	payload := openAICompletionRequest{
		Model:         s.modelID,
		Prompt:        prompt,
		MaxTokens:     s.baseParams.MaxTokens,
		Temperature:   s.baseParams.Temperature,
		TopP:          s.baseParams.TopP,
		TopK:          s.baseParams.TopK,
		Stop:          s.baseParams.Stop,
		Seed:          s.baseParams.Seed,
		Stream:        true,
		RepeatPenalty: s.baseParams.RepeatPenalty,
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.adapter.baseURL+"/v1/completions", bytes.NewReader(body))
	if err != nil {
		return FinalResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	if s.adapter.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+s.adapter.apiKey)
	}
	resp, err := s.adapter.httpClient.Do(req)
	if err != nil {
		// Translate context timeouts/cancels
		if ctx.Err() != nil {
			return FinalResult{}, ctx.Err()
		}
		return FinalResult{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return FinalResult{}, errors.New("llama server http error: "+resp.Status+": "+string(b))
	}
	// Stream parse. Many servers emit Server-Sent Events with lines beginning with "data: ".
	r := bufio.NewReader(resp.Body)
	var final FinalResult
	for {
		line, err := r.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimSpace(line)
			if line == "" {
				// skip heartbeats/empties
			} else if strings.HasPrefix(strings.ToLower(line), "data:") {
				data := strings.TrimSpace(line[len("data:"):])
				if data == "[DONE]" {
					break
				}
				var msg openAIStreamResponse
				if err := json.Unmarshal([]byte(data), &msg); err == nil && len(msg.Choices) > 0 {
					frag := msg.Choices[0].Delta.Content
					if frag != "" {
						if cbErr := onToken(frag); cbErr != nil {
							return final, cbErr
						}
					}
					if fr := msg.Choices[0].FinishReason; fr != "" {
						final.FinishReason = fr
					}
					continue
				}
				// Some servers stream raw JSON objects per line (non-SSE). Attempt to parse token fields.
				var generic map[string]any
				if err := json.Unmarshal([]byte(data), &generic); err == nil {
					if tok, ok := generic["content"].(string); ok && tok != "" {
						if cbErr := onToken(tok); cbErr != nil {
							return final, cbErr
						}
						continue
					}
				}
				log.Printf("adapter=llama_server event=unknown_stream_line line=%q", line)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			// Respect context errors
			if ctx.Err() != nil {
				return final, ctx.Err()
			}
			log.Printf("adapter=llama_server event=stream_read_error err=%v", err)
			return final, err
		}
	}
	return final, nil
}

func (s *llamaServerSession) Close() error { return nil }
