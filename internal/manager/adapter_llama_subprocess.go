package manager

import (
    "bytes"
    "bufio"
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "net/http"
    "os/exec"
    "strconv"
    "strings"
    "sync"
    "time"
    "syscall"
)

// llamaSubprocessAdapter spawns and manages a llama.cpp server per model path.

type llamaSubprocessAdapter struct {
    cfg        ManagerConfig
    mu         sync.Mutex
    procs      map[string]*procInfo // key: modelPath
    httpClient *http.Client
    publisher  EventPublisher
}

// isHealthy checks if the llama-server at baseURL responds OK to /v1/models.
func (a *llamaSubprocessAdapter) isHealthy(baseURL string, timeout time.Duration) bool {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
    if err != nil { return false }
    resp, err := a.httpClient.Do(req)
    if err != nil { return false }
    defer resp.Body.Close()
    return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func pickPortInRange(host string, start, end int) (int, error) {
    for p := start; p <= end; p++ {
        l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
        if err != nil { continue }
        _ = l.Close()
        return p, nil
    }
    return 0, fmt.Errorf("no free port in range %d-%d", start, end)
}

// StopAll terminates all managed subprocesses. Best effort.
func (a *llamaSubprocessAdapter) StopAll() {
    a.mu.Lock()
    paths := make([]string, 0, len(a.procs))
    for k := range a.procs { paths = append(paths, k) }
    a.mu.Unlock()
    for _, path := range paths {
        _ = a.Stop(path)
    }
}

// NewLlamaSubprocessAdapter constructs a subprocess-backed adapter (stub).
func NewLlamaSubprocessAdapter(cfg ManagerConfig) InferenceAdapter {
    host := strings.TrimSpace(cfg.LlamaHost)
    if host == "" { host = "127.0.0.1" }
    // Intentionally set Timeout=0: all calls must use context-based timeouts.
    // ensureProcess() and Generate() create requests with contexts carrying deadlines.
    cli := &http.Client{ Timeout: 0 }
    return &llamaSubprocessAdapter{cfg: cfg, procs: make(map[string]*procInfo), httpClient: cli, publisher: noopPublisher{}}
}

type procInfo struct {
    cmd    *exec.Cmd
    baseURL string
    ready  bool
    pid    int
}

// llamaSubprocessSession represents a session in spawn mode.
type llamaSubprocessSession struct {
    a         *llamaSubprocessAdapter
    modelPath string
    baseURL   string
    params    InferParams
}

func (a *llamaSubprocessAdapter) Start(modelPath string, params InferParams) (InferSession, error) {
    if strings.TrimSpace(modelPath) == "" {
        return nil, errors.New("modelPath is empty")
    }
    baseURL, err := a.ensureProcess(modelPath)
    if err != nil {
        return nil, err
    }
    return &llamaSubprocessSession{a: a, modelPath: modelPath, baseURL: baseURL, params: params}, nil
}

func (s *llamaSubprocessSession) Generate(ctx context.Context, prompt string, onToken func(string) error) (FinalResult, error) {
    // Reuse OpenAI-compatible streaming similar to server adapter
    payload := openAICompletionRequest{
        Model:         "", // let server default
        Prompt:        prompt,
        MaxTokens:     s.params.MaxTokens,
        Temperature:   s.params.Temperature,
        TopP:          s.params.TopP,
        TopK:          s.params.TopK,
        Stop:          s.params.Stop,
        Seed:          s.params.Seed,
        Stream:        true,
        RepeatPenalty: s.params.RepeatPenalty,
    }
    body, _ := json.Marshal(payload)
    req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/v1/completions", bytes.NewReader(body))
    if err != nil { return FinalResult{}, err }
    req.Header.Set("Content-Type", "application/json")
    resp, err := s.a.httpClient.Do(req)
    if err != nil {
        if ctx.Err() != nil { return FinalResult{}, ctx.Err() }
        return FinalResult{}, err
    }
    defer resp.Body.Close()
    if resp.StatusCode < 200 || resp.StatusCode >= 300 {
        b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
        return FinalResult{}, fmt.Errorf("llama server http error: %s: %s", resp.Status, string(b))
    }
    r := bufio.NewReader(resp.Body)
    var final FinalResult
    for {
        line, err := r.ReadString('\n')
        if len(line) > 0 {
            l := strings.TrimSpace(line)
            if l != "" && strings.HasPrefix(strings.ToLower(l), "data:") {
                data := strings.TrimSpace(l[len("data:"):])
                if data == "[DONE]" { break }
                var msg openAIStreamResponse
                if e := json.Unmarshal([]byte(data), &msg); e == nil && len(msg.Choices) > 0 {
                    frag := msg.Choices[0].Delta.Content
                    if frag != "" {
                        if cbErr := onToken(frag); cbErr != nil { return final, cbErr }
                    }
                    if fr := msg.Choices[0].FinishReason; fr != "" { final.FinishReason = fr }
                }
            }
        }
        if err != nil {
            if errors.Is(err, io.EOF) { break }
            if ctx.Err() != nil { return final, ctx.Err() }
            return final, err
        }
    }
    return final, nil
}

func (s *llamaSubprocessSession) Close() error { return nil }

// ensureProcess starts (or returns existing) llama-server for given modelPath and waits readiness.
func (a *llamaSubprocessAdapter) ensureProcess(modelPath string) (string, error) {
    a.mu.Lock()
    if a.procs == nil { a.procs = make(map[string]*procInfo) }
    if p := a.procs[modelPath]; p != nil {
        base := p.baseURL
        // If marked ready, quickly health-check; if unhealthy, drop and restart.
        if p.ready {
            a.mu.Unlock()
            if a.isHealthy(base, 1*time.Second) {
                return base, nil
            }
            // unhealthy: fall through to restart
            a.mu.Lock()
            // best effort stop; ignore error
            _ = a.Stop(modelPath)
        } else {
            // Not ready yet: try health just in case; else continue to wait/spawn
            a.mu.Unlock()
            if a.isHealthy(base, 1*time.Second) {
                a.mu.Lock()
                if q := a.procs[modelPath]; q != nil { q.ready = true }
                a.mu.Unlock()
                return base, nil
            }
            a.mu.Lock()
            _ = a.Stop(modelPath)
        }
    }
    a.mu.Unlock()

    // Create process
    host := strings.TrimSpace(a.cfg.LlamaHost)
    if host == "" { host = "127.0.0.1" }
    // Choose port (respect configured range if set)
    var port int
    var err error
    if a.cfg.LlamaPortStart > 0 && a.cfg.LlamaPortEnd >= a.cfg.LlamaPortStart {
        port, err = pickPortInRange(host, a.cfg.LlamaPortStart, a.cfg.LlamaPortEnd)
    } else {
        port, err = pickFreePort(host)
    }
    if err != nil { return "", err }
    baseURL := fmt.Sprintf("http://%s:%d", host, port)

    args := []string{
        "-m", modelPath,
        "--host", host,
        "--port", fmt.Sprint(port),
    }
    if a.cfg.LlamaCtxSize > 0 { args = append(args, "-c", fmt.Sprint(a.cfg.LlamaCtxSize)) }
    if a.cfg.LlamaNGL > 0 { args = append(args, "-ngl", fmt.Sprint(a.cfg.LlamaNGL)) }
    if a.cfg.LlamaThreads > 0 { args = append(args, "-t", fmt.Sprint(a.cfg.LlamaThreads)) }
    if len(a.cfg.LlamaExtraArgs) > 0 { args = append(args, a.cfg.LlamaExtraArgs...) }

    cmd := exec.Command(a.cfg.LlamaBin, args...)
    // Inherit stdout/stderr to aid debugging. Could swap for logger later.
    // cmd.Stdout = os.Stdout; cmd.Stderr = os.Stderr
    // Capture stderr for diagnostics (kept in-memory; tail is included on failure)
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    if err := cmd.Start(); err != nil {
        return "", fmt.Errorf("start llama-server: %w", err)
    }
    log.Printf("adapter=llama_subprocess event=start model=%q pid=%d host=%s port=%d", modelPath, cmd.Process.Pid, host, port)
    a.publisher.Publish(Event{Name: "spawn_start", ModelID: modelPath, Fields: map[string]any{"pid": cmd.Process.Pid, "host": host, "port": port}})

    // Save proc before readiness wait
    a.mu.Lock()
    a.procs[modelPath] = &procInfo{cmd: cmd, baseURL: baseURL, ready: false, pid: cmd.Process.Pid}
    a.mu.Unlock()

    // Early-exit watcher: surface non-zero exit before readiness
    waitErrCh := make(chan error, 1)
    go func() {
        waitErrCh <- cmd.Wait()
    }()

    // Wait readiness with deadline and early failure detection
    deadline := time.Now().Add(30 * time.Second)
    for {
        if time.Now().After(deadline) {
            // Cleanup proc entry on timeout
            a.mu.Lock()
            delete(a.procs, modelPath)
            a.mu.Unlock()
            log.Printf("adapter=llama_subprocess event=timeout model=%q pid=%d", modelPath, cmd.Process.Pid)
            a.publisher.Publish(Event{Name: "spawn_timeout", ModelID: modelPath, Fields: map[string]any{"pid": cmd.Process.Pid}})
            return "", fmt.Errorf("llama-server not ready in time: %s", baseURL)
        }
        // Check if process exited
        select {
        case werr := <-waitErrCh:
            if werr != nil {
                // Include a small tail of stderr for context
                tail := stderr.String()
                if len(tail) > 4096 { tail = tail[len(tail)-4096:] }
                // Cleanup proc entry on failure
                a.mu.Lock()
                delete(a.procs, modelPath)
                a.mu.Unlock()
                log.Printf("adapter=llama_subprocess event=exit_early model=%q pid=%d err=%v", modelPath, cmd.Process.Pid, werr)
                a.publisher.Publish(Event{Name: "spawn_exit", ModelID: modelPath, Fields: map[string]any{"pid": cmd.Process.Pid, "error": werr.Error()}})
                return "", fmt.Errorf("llama-server exited early: %v; stderr tail: %s", werr, tail)
            }
            // Unexpected nil error here means process exited cleanly before ready
            a.mu.Lock()
            delete(a.procs, modelPath)
            a.mu.Unlock()
            log.Printf("adapter=llama_subprocess event=exit_clean model=%q pid=%d before_ready=1", modelPath, cmd.Process.Pid)
            a.publisher.Publish(Event{Name: "spawn_exit", ModelID: modelPath, Fields: map[string]any{"pid": cmd.Process.Pid, "before_ready": true}})
            return "", fmt.Errorf("llama-server exited before ready: %s", baseURL)
        default:
            // proceed to health check
        }

        ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
        req, _ := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/v1/models", nil)
        resp, err := a.httpClient.Do(req)
        if err == nil {
            _ = resp.Body.Close()
            if resp.StatusCode >= 200 && resp.StatusCode < 300 {
                cancel()
                log.Printf("adapter=llama_subprocess event=ready model=%q pid=%d url=%s", modelPath, cmd.Process.Pid, baseURL)
                a.publisher.Publish(Event{Name: "spawn_ready", ModelID: modelPath, Fields: map[string]any{"pid": cmd.Process.Pid, "url": baseURL}})
                break
            }
        }
        cancel()
        time.Sleep(100 * time.Millisecond)
    }
    a.mu.Lock()
    if p := a.procs[modelPath]; p != nil { p.ready = true }
    a.mu.Unlock()
    return baseURL, nil
}

func pickFreePort(host string) (int, error) {
    l, err := net.Listen("tcp", host+":0")
    if err != nil { return 0, err }
    defer l.Close()
    addr := l.Addr().String()
    // addr like 127.0.0.1:54321
    lastColon := strings.LastIndex(addr, ":")
    if lastColon < 0 { return 0, fmt.Errorf("unexpected addr: %s", addr) }
    p, err := strconv.Atoi(addr[lastColon+1:])
    if err != nil { return 0, err }
    return p, nil
}

// Accessor to safely read proc info under lock and return a snapshot
func (a *llamaSubprocessAdapter) getProcInfo(modelPath string) (pid int, baseURL string, ready bool, ok bool) {
    a.mu.Lock()
    defer a.mu.Unlock()
    if a.procs == nil {
        return 0, "", false, false
    }
    if p := a.procs[modelPath]; p != nil {
        return p.pid, p.baseURL, p.ready, true
    }
    return 0, "", false, false
}

// Stop terminates a spawned llama-server process for the given modelPath, if present.
func (a *llamaSubprocessAdapter) Stop(modelPath string) error {
    a.mu.Lock()
    p := a.procs[modelPath]
    a.mu.Unlock()
    if p == nil || p.cmd == nil || p.cmd.Process == nil {
        return nil
    }
    // Try to gracefully terminate first, then fall back to kill.
    // Best-effort: platform-specific; on Unix send SIGTERM.
    _ = p.cmd.Process.Signal(syscall.SIGTERM)
    done := make(chan struct{})
    go func() {
        _, _ = p.cmd.Process.Wait()
        close(done)
    }()
    select {
    case <-done:
        // exited gracefully
    case <-time.After(2 * time.Second):
        // force kill
        _ = p.cmd.Process.Kill()
        _, _ = p.cmd.Process.Wait()
    }
    a.mu.Lock()
    delete(a.procs, modelPath)
    a.mu.Unlock()
    a.publisher.Publish(Event{Name: "spawn_stop", ModelID: modelPath, Fields: map[string]any{}})
    return nil
}

// setPublisher installs an EventPublisher for emitting adapter events.
func (a *llamaSubprocessAdapter) setPublisher(p EventPublisher) {
    if p == nil { a.publisher = noopPublisher{}; return }
    a.publisher = p
}
