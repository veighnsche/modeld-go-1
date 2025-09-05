package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"modeld/internal/config"
	"modeld/internal/httpapi"
	"modeld/internal/manager"
	"modeld/internal/registry"

	"github.com/rs/zerolog"
)

func main() {
	// Flags with environment variable defaults
	defaultAddr := ":8080"
	if v := os.Getenv("MODELD_ADDR"); v != "" {
		defaultAddr = v
	}
	addr := flag.String("addr", defaultAddr, "HTTP listen address, e.g. :8080")
	configPath := flag.String("config", "", "Optional path to config file (yaml|yml|json|toml)")
	modelsDir := flag.String("models-dir", "~/models/llm", "Directory to scan for *.gguf model files")
	vramBudgetMB := flag.Int("vram-budget-mb", 0, "VRAM budget in MB for all instances (0=unlimited)")
	vramMarginMB := flag.Int("vram-margin-mb", 0, "Reserved VRAM margin in MB to keep free")
	defaultModel := flag.String("default-model", "", "Default model id when request omits model")
	shutdownTimeout := flag.Duration("shutdown-timeout", 5*time.Second, "Graceful shutdown timeout (e.g., 5s, 30s)")
	maxBodyBytes := flag.Int64("max-body-bytes", 1<<20, "Maximum request body size in bytes for JSON endpoints (default 1MiB)")
	logLevel := flag.String("log-level", os.Getenv("MODELD_LOG_LEVEL"), "Log level: off|error|info|debug (default from MODELD_LOG_LEVEL)")
	// Backpressure knobs
	maxQueueDepth := flag.Int("max-queue-depth", 0, "Max queued requests per model instance (0=default)")
	maxWait := flag.Duration("max-wait", 0, "Max time a request may wait in an instance queue (e.g., 30s; 0=default)")
	// Infer handler timeout (separate from server timeouts)
	inferTimeout := flag.Duration("infer-timeout", 0, "Max duration for /infer request before cancellation (0=disabled)")
	// CORS
	corsEnabled := flag.Bool("cors-enabled", false, "Enable CORS middleware")
	corsOrigins := flag.String("cors-origins", "", "Comma-separated list of allowed CORS origins")
	corsMethods := flag.String("cors-methods", "", "Comma-separated list of allowed CORS methods")
	corsHeaders := flag.String("cors-headers", "", "Comma-separated list of allowed CORS request headers")
	// Inference / llama.cpp (always enabled; flags configure adapter)
	llamaBin := flag.String("llama-bin", "", "Path to llama.cpp server binary (llama-server)")
	llamaCtx := flag.Int("llama-ctx", 4096, "Context window size for llama.cpp")
	llamaThreads := flag.Int("llama-threads", 0, "Threads for llama.cpp (0=auto)")
	flag.Parse()

	// Determine which flags were explicitly set to give CLI precedence over config file
	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// If config file provided, load and merge (only apply fields for flags not explicitly set)
	if *configPath != "" {
		if cfg, err := config.Load(*configPath); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		} else {
			if !setFlags["addr"] && cfg.Addr != "" {
				*addr = cfg.Addr
			}
			if !setFlags["models-dir"] && cfg.ModelsDir != "" {
				*modelsDir = cfg.ModelsDir
			}
			if !setFlags["vram-budget-mb"] && cfg.VRAMBudgetMB != 0 {
				*vramBudgetMB = cfg.VRAMBudgetMB
			}
			if !setFlags["vram-margin-mb"] && cfg.VRAMMarginMB != 0 {
				*vramMarginMB = cfg.VRAMMarginMB
			}
			if !setFlags["default-model"] && cfg.DefaultModel != "" {
				*defaultModel = cfg.DefaultModel
			}
			if !setFlags["log-level"] && cfg.LogLevel != "" {
				*logLevel = cfg.LogLevel
			}
			if !setFlags["max-body-bytes"] && cfg.MaxBodyBytes > 0 {
				*maxBodyBytes = cfg.MaxBodyBytes
			}
			if !setFlags["infer-timeout"] && cfg.InferTimeout != "" {
				if d, err := time.ParseDuration(cfg.InferTimeout); err == nil {
					*inferTimeout = d
				}
			}
			if !setFlags["cors-enabled"] {
				*corsEnabled = cfg.CORSEnabled
			}
			if !setFlags["cors-origins"] && len(cfg.CORSAllowedOrigins) > 0 {
				*corsOrigins = strings.Join(cfg.CORSAllowedOrigins, ",")
			}
			if !setFlags["cors-methods"] && len(cfg.CORSAllowedMethods) > 0 {
				*corsMethods = strings.Join(cfg.CORSAllowedMethods, ",")
			}
			if !setFlags["cors-headers"] && len(cfg.CORSAllowedHeaders) > 0 {
				*corsHeaders = strings.Join(cfg.CORSAllowedHeaders, ",")
			}
			if !setFlags["max-queue-depth"] && cfg.MaxQueueDepth > 0 {
				*maxQueueDepth = cfg.MaxQueueDepth
			}
			if !setFlags["max-wait"] && cfg.MaxWait != "" {
				if d, err := time.ParseDuration(cfg.MaxWait); err == nil {
					*maxWait = d
				}
			}
			// Inference / llama.cpp (CLI has precedence): no enable flag; configured by options
			if !setFlags["llama-bin"] && cfg.LlamaBin != "" {
				*llamaBin = cfg.LlamaBin
			}
			if !setFlags["llama-ctx"] && cfg.LlamaCtx > 0 {
				*llamaCtx = cfg.LlamaCtx
			}
			if !setFlags["llama-threads"] && cfg.LlamaThreads >= 0 {
				*llamaThreads = cfg.LlamaThreads
			}
		}
	}

	// Expand home directory in modelsDir if prefixed with ~
	if strings.HasPrefix(*modelsDir, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			// Support cases like ~/models/llm and bare ~
			if *modelsDir == "~" {
				*modelsDir = home
			} else if strings.HasPrefix(*modelsDir, "~/") {
				*modelsDir = filepath.Join(home, (*modelsDir)[2:])
			}
		}
	}

	// Load registry by scanning modelsDir for *.gguf
	scanner := registry.NewGGUFScanner()
	reg, err := scanner.Scan(*modelsDir)
	if err != nil {
		log.Fatalf("failed to load models: %v", err)
	}
	// Use ManagerConfig to pass backpressure knobs
	mgr := manager.NewWithConfig(manager.ManagerConfig{
		Registry:         reg,
		BudgetMB:         *vramBudgetMB,
		MarginMB:         *vramMarginMB,
		DefaultModel:     *defaultModel,
		MaxQueueDepth:    *maxQueueDepth,
		MaxWait:          *maxWait,
		LlamaBin:         *llamaBin,
		LlamaCtx:         *llamaCtx,
		LlamaThreads:     *llamaThreads,
	})

	// Preflight: validate adapter presence and default model path.
	checks := mgr.Preflight()
	preflightOK := true
	for _, c := range checks {
		if !c.OK {
			preflightOK = false
			log.Printf("[preflight] %s: NOT OK - %s", c.Name, c.Message)
		} else {
			log.Printf("[preflight] %s: OK", c.Name)
		}
	}
	if !preflightOK {
		log.Fatalf("preflight failed; resolve issues above and restart")
	}

	// Set a base context that we will cancel on shutdown to propagate cancellation to handlers.
	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()
	httpapi.SetBaseContext(baseCtx)

	// Configure structured logging
	// Set global level based on flag/env
	switch strings.ToLower(strings.TrimSpace(*logLevel)) {
	case "off", "":
		zerolog.SetGlobalLevel(zerolog.Disabled)
	case "error":
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case "debug":
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
	// Use console writer for human-friendly dev logs; can be swapped for JSON
	cw := zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}
	logger := zerolog.New(cw).With().Timestamp().Str("service", "modeld").Logger()
	httpapi.SetLogger(logger)
	// Apply HTTP settings
	httpapi.SetMaxBodyBytes(*maxBodyBytes)
	if *inferTimeout != 0 {
		httpapi.SetInferTimeoutSeconds(int64((*inferTimeout).Seconds()))
	}
	// Configure CORS if enabled; provide sensible defaults if lists are empty
	var origins, methods, headers []string
	if *corsOrigins != "" {
		origins = splitCSV(*corsOrigins)
	}
	if *corsMethods != "" {
		methods = splitCSV(*corsMethods)
	} else {
		methods = []string{"GET", "POST", "OPTIONS"}
	}
	if *corsHeaders != "" {
		headers = splitCSV(*corsHeaders)
	} else {
		headers = []string{"Accept", "Authorization", "Content-Type", "X-Requested-With", "X-Log-Level"}
	}
	httpapi.SetCORSOptions(*corsEnabled, origins, methods, headers)
	// NewMux registers: /models, /status, /infer, /healthz, /readyz, /metrics
	mux := httpapi.NewMux(mgr)
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		if *configPath != "" {
			log.Printf("modeld listening on %s (models dir: %s, config: %s)", *addr, *modelsDir, *configPath)
		} else {
			log.Printf("modeld listening on %s (models dir: %s)", *addr, *modelsDir)
		}
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown (Ctrl+C / SIGTERM)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	// Cancel base context to stop in-flight handler work
	baseCancel()
	ctx, cancel := context.WithTimeout(context.Background(), *shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}

// splitCSV splits a comma-separated list into trimmed non-empty strings.
func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
