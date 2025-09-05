package main

// killProcesses is a best-effort cleanup hook invoked by web test flows.
// At present, individual processes (server/preview) are started with contexts
// and are terminated via their own deferred .Kill() calls, so this hook is a
// no-op left for future enhancements.
func killProcesses() error { return nil }
