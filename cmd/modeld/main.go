package main

import (
"context"
"log"
"net/http"
"os"
"time"

"modeld/internal/httpapi"
"modeld/internal/manager"
)

func main() {
addr := ":8080"
if v := os.Getenv("MODELd_ADDR"); v != "" { // optional override
addr = v
}
cfgPath := "configs/models.yaml"
if len(os.Args) > 1 && os.Args[1] == "--config" && len(os.Args) > 2 {
cfgPath = os.Args[2]
}

// TODO: load registry from cfgPath
mgr := manager.New()

mux := httpapi.NewMux(mgr) // registers /models, /status, /switch, /events, /healthz (stubs)
srv := &http.Server{Addr: addr, Handler: mux}

go func() {
log.Printf("modeld listening on %s (config: %s)", addr, cfgPath)
if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
log.Fatalf("server error: %v", err)
}
}()

// Basic graceful shutdown (Ctrl+C)
stop := make(chan os.Signal, 1)
// signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM) // enable later
_ = stop

// Block forever in MVP scaffold
select {}

// ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// defer cancel()
// _ = srv.Shutdown(ctx)
}
