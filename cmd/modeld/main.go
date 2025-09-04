package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"modeld/internal/httpapi"
	"modeld/internal/config"
	"modeld/internal/registry"
	"modeld/internal/manager"
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
	flag.Parse()

	// Determine which flags were explicitly set to give CLI precedence over config file
	setFlags := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { setFlags[f.Name] = true })

	// If config file provided, load and merge (only apply fields for flags not explicitly set)
	if *configPath != "" {
		if cfg, err := config.Load(*configPath); err != nil {
			log.Fatalf("failed to load config file: %v", err)
		} else {
			if !setFlags["addr"] && cfg.Addr != "" { *addr = cfg.Addr }
			if !setFlags["models-dir"] && cfg.ModelsDir != "" { *modelsDir = cfg.ModelsDir }
			if !setFlags["vram-budget-mb"] && cfg.VRAMBudgetMB != 0 { *vramBudgetMB = cfg.VRAMBudgetMB }
			if !setFlags["vram-margin-mb"] && cfg.VRAMMarginMB != 0 { *vramMarginMB = cfg.VRAMMarginMB }
			if !setFlags["default-model"] && cfg.DefaultModel != "" { *defaultModel = cfg.DefaultModel }
		}
	}

	// Load registry by scanning modelsDir for *.gguf
	scanner := registry.NewGGUFScanner()
	reg, err := scanner.Scan(*modelsDir)
	if err != nil {
		log.Fatalf("failed to load models: %v", err)
	}
	mgr := manager.New(reg, *vramBudgetMB, *vramMarginMB, *defaultModel)

	// Set a base context that we will cancel on shutdown to propagate cancellation to handlers.
	baseCtx, baseCancel := context.WithCancel(context.Background())
	defer baseCancel()
	httpapi.SetBaseContext(baseCtx)
	mux := httpapi.NewMux(mgr) // registers /models, /status, /switch, /events, /healthz (stubs)
	srv := &http.Server{Addr: *addr, Handler: mux}

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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}
