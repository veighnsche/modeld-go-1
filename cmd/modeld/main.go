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
	modelsDir := flag.String("models-dir", "~/models/llm", "Directory to scan for *.gguf model files")
	vramBudgetMB := flag.Int("vram-budget-mb", 0, "VRAM budget in MB for all instances (0=unlimited)")
	vramMarginMB := flag.Int("vram-margin-mb", 0, "Reserved VRAM margin in MB to keep free")
	defaultModel := flag.String("default-model", "", "Default model id when request omits model")
	flag.Parse()

	// Load registry by scanning modelsDir for *.gguf
	reg, err := registry.LoadDir(*modelsDir)
	if err != nil {
		log.Fatalf("failed to load models: %v", err)
	}
	mgr := manager.New(reg, *vramBudgetMB, *vramMarginMB, *defaultModel)

	mux := httpapi.NewMux(mgr) // registers /models, /status, /switch, /events, /healthz (stubs)
	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Printf("modeld listening on %s (models dir: %s)", *addr, *modelsDir)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Graceful shutdown (Ctrl+C / SIGTERM)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown error: %v", err)
	}
}
