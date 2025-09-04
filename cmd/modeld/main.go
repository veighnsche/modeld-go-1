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
	"modeld/internal/manager"
)

func main() {
	// Flags with environment variable defaults
	defaultAddr := ":8080"
	if v := os.Getenv("MODELD_ADDR"); v != "" {
		defaultAddr = v
	}
	addr := flag.String("addr", defaultAddr, "HTTP listen address, e.g. :8080")
	cfgPath := flag.String("config", "configs/models.yaml", "Path to models config YAML")
	flag.Parse()

	// TODO: load registry from cfgPath
	mgr := manager.New()

	mux := httpapi.NewMux(mgr) // registers /models, /status, /switch, /events, /healthz (stubs)
	srv := &http.Server{Addr: *addr, Handler: mux}

	go func() {
		log.Printf("modeld listening on %s (config: %s)", *addr, *cfgPath)
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
