package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// Command helpers
func runCmdVerbose(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCmdStreaming(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()
	if err := cmd.Start(); err != nil { return err }
	go stream("OUT", stdout)
	go stream("ERR", stderr)
	return cmd.Wait()
}

func runEnvCmdStreaming(ctx context.Context, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = os.Environ()
	for k, v := range env { cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v)) }
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type ioReader interface { Read(p []byte) (n int, err error) }

func stream(prefix string, r ioReader) {
	s := bufio.NewScanner(r)
	for s.Scan() {
		fmt.Println(s.Text())
	}
}
