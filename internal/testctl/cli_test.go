package testctl

import (
	"errors"
	"testing"
)

// helper to restore stubs after each test
func withCLIStubs(t *testing.T, stubs func()) func() {
	t.Helper()
	oldInstallNodeJS := fnInstallNodeJS
	oldInstallGo := fnInstallGo
	oldInstallPy := fnInstallPy
	oldRunGoTests := fnRunGoTests
	oldRunPyTests := fnRunPyTests
	oldTestWebMock := fnTestWebMock
	oldTestWebLiveHost := fnTestWebLiveHost
	oldHasHostModels := fnHasHostModels
	stubs()
	return func() {
		fnInstallNodeJS = oldInstallNodeJS
		fnInstallGo = oldInstallGo
		fnInstallPy = oldInstallPy
		fnRunGoTests = oldRunGoTests
		fnRunPyTests = oldRunPyTests
		fnTestWebMock = oldTestWebMock
		fnTestWebLiveHost = oldTestWebLiveHost
		fnHasHostModels = oldHasHostModels
	}
}

func TestRun_InstallCommands(t *testing.T) {
	cfg := &Config{WebPort: 5173, LogLvl: "info"}

	// js
	cleanup := withCLIStubs(t, func() {
		fnInstallNodeJS = func() error { return nil }
	})
	defer cleanup()
	if err := Run([]string{"install", "js"}, cfg); err != nil {
		t.Fatalf("install js: unexpected err: %v", err)
	}

	// all
	calls := make(map[string]int)
	cleanup = withCLIStubs(t, func() {
		fnInstallNodeJS = func() error { calls["js"]++; return nil }
		fnInstallGo = func() error { calls["go"]++; return nil }
		fnInstallPy = func() error { calls["py"]++; return nil }
	})
	defer cleanup()
	if err := Run([]string{"install", "all"}, cfg); err != nil {
		t.Fatalf("install all: unexpected err: %v", err)
	}
	if calls["js"] != 1 || calls["go"] != 1 || calls["py"] != 1 {
		t.Fatalf("install all did not fan out correctly: %+v", calls)
	}
}

func TestRun_TestCommands(t *testing.T) {
	cfg := &Config{WebPort: 5173, LogLvl: "info"}

	// go
	cleanup := withCLIStubs(t, func() {
		fnRunGoTests = func() error { return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "go"}, cfg); err != nil {
		t.Fatalf("test go: unexpected err: %v", err)
	}

	// api:py
	cleanup = withCLIStubs(t, func() {
		fnRunPyTests = func() error { return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "api:py"}, cfg); err != nil {
		t.Fatalf("test api:py: unexpected err: %v", err)
	}

	// web mock
	calledMock := 0
	cleanup = withCLIStubs(t, func() {
		fnTestWebMock = func(c *Config) error {
			if c.WebPort != cfg.WebPort {
				t.Fatalf("cfg mismatch")
			}
			calledMock++
			return nil
		}
	})
	defer cleanup()
	if err := Run([]string{"test", "web", "mock"}, cfg); err != nil {
		t.Fatalf("test web mock: unexpected err: %v", err)
	}
	if calledMock != 1 {
		t.Fatalf("mock not called")
	}

	// web live:host
	calledLive := 0
	cleanup = withCLIStubs(t, func() {
		fnTestWebLiveHost = func(c *Config) error { calledLive++; return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "web", "live:host"}, cfg); err != nil {
		t.Fatalf("test web live:host: unexpected err: %v", err)
	}
	if calledLive != 1 {
		t.Fatalf("live not called")
	}

	// web auto: with host models
	calledLive = 0
	cleanup = withCLIStubs(t, func() {
		fnHasHostModels = func() bool { return true }
		fnTestWebLiveHost = func(c *Config) error { calledLive++; return nil }
		fnTestWebMock = func(c *Config) error { t.Fatalf("mock should not be called when host models exist"); return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "web", "auto"}, cfg); err != nil {
		t.Fatalf("test web auto (host): unexpected err: %v", err)
	}
	if calledLive != 1 {
		t.Fatalf("live not called in auto when host models present")
	}

	// web auto: without host models
	calledMock = 0
	cleanup = withCLIStubs(t, func() {
		fnHasHostModels = func() bool { return false }
		fnTestWebMock = func(c *Config) error { calledMock++; return nil }
		fnTestWebLiveHost = func(c *Config) error { t.Fatalf("live should not be called when no host models"); return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "web", "auto"}, cfg); err != nil {
		t.Fatalf("test web auto (mock): unexpected err: %v", err)
	}
	if calledMock != 1 {
		t.Fatalf("mock not called in auto when no host models")
	}

	// all auto: fanout
	calls := make(map[string]int)
	cleanup = withCLIStubs(t, func() {
		fnRunGoTests = func() error { calls["go"]++; return nil }
		fnRunPyTests = func() error { calls["py"]++; return nil }
		fnHasHostModels = func() bool { return false }
		fnTestWebMock = func(c *Config) error { calls["web:mock"]++; return nil }
		fnTestWebLiveHost = func(c *Config) error { calls["web:live"]++; return nil }
	})
	defer cleanup()
	if err := Run([]string{"test", "all", "auto"}, cfg); err != nil {
		t.Fatalf("test all auto: unexpected err: %v", err)
	}
	if calls["go"] != 1 || calls["py"] != 1 || calls["web:mock"] != 1 || calls["web:live"] != 0 {
		t.Fatalf("test all auto fanout incorrect: %+v", calls)
	}
}

func TestRun_Errors(t *testing.T) {
	cfg := &Config{WebPort: 5173, LogLvl: "info"}

	// unknown command
	if err := Run([]string{"wat"}, cfg); err == nil {
		t.Fatalf("expected error for unknown command")
	}

	// missing subcommands
	if err := Run([]string{"install"}, cfg); err == nil {
		t.Fatalf("expected error for install without subcommand")
	}
	if err := Run([]string{"test"}, cfg); err == nil {
		t.Fatalf("expected error for test without subcommand")
	}
	if err := Run([]string{"test", "web"}, cfg); err == nil {
		t.Fatalf("expected error for test web without mode")
	}

	// propagate sub-action errors
	cleanup := withCLIStubs(t, func() {
		fnInstallNodeJS = func() error { return errors.New("boom") }
	})
	defer cleanup()
	if err := Run([]string{"install", "js"}, cfg); err == nil {
		t.Fatalf("expected error to propagate from sub-action")
	}
}
