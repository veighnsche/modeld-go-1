package testctl

import (
    "os/exec"
    "sync"
)

// ProcManager tracks started processes and can kill them all on cleanup.
type ProcManager struct {
    mu    sync.Mutex
    procs []*exec.Cmd
}

func NewProcManager() *ProcManager { return &ProcManager{} }

func (pm *ProcManager) Add(cmd *exec.Cmd) {
    pm.mu.Lock()
    pm.procs = append(pm.procs, cmd)
    pm.mu.Unlock()
}

// KillAll attempts to kill all tracked processes. It proceeds best-effort.
func (pm *ProcManager) KillAll() error {
    pm.mu.Lock()
    procs := append([]*exec.Cmd(nil), pm.procs...)
    pm.procs = nil
    pm.mu.Unlock()
    for _, c := range procs {
        if c != nil && c.Process != nil {
            _ = c.Process.Kill()
        }
    }
    return nil
}

// package-level default manager used by helpers
var defaultProcManager = NewProcManager()

// TrackProcess registers a process with the default manager for later cleanup.
func TrackProcess(cmd *exec.Cmd) { defaultProcManager.Add(cmd) }

// killProcesses is kept for backward-compatibility with existing flows.
// It delegates to the default process manager.
func killProcesses() error { return defaultProcManager.KillAll() }
