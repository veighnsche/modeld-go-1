package testctl

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Unified command runner
type Cmd struct {
	Path   string
	Args   []string
	Env    map[string]string // additional env vars
	Dir    string            // working directory
	Stream bool              // if true, stream stdout/err via scanner
}

func RunCmd(ctx context.Context, c Cmd) error {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	if c.Dir != "" {
		cmd.Dir = c.Dir
	}
	// inherit environment
	cmd.Env = os.Environ()
	for k, v := range c.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if c.Stream {
		stdout, _ := cmd.StdoutPipe()
		stderr, _ := cmd.StderrPipe()
		if err := cmd.Start(); err != nil {
			return err
		}
		go stream("OUT", stdout)
		go stream("ERR", stderr)
		return cmd.Wait()
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Backwards-compatible helpers built on RunCmd
func runCmdVerbose(ctx context.Context, name string, args ...string) error {
	return RunCmd(ctx, Cmd{Path: name, Args: args, Stream: false})
}

func runCmdStreaming(ctx context.Context, name string, args ...string) error {
	return RunCmd(ctx, Cmd{Path: name, Args: args, Stream: true})
}

func runEnvCmdStreaming(ctx context.Context, env map[string]string, name string, args ...string) error {
	return RunCmd(ctx, Cmd{Path: name, Args: args, Env: env, Stream: false})
}

type ioReader interface {
	Read(p []byte) (n int, err error)
}

func stream(prefix string, r ioReader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		fmt.Println(s.Text())
	}
}
