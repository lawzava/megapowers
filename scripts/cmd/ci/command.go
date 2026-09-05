package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
)

type commandSpec struct {
	Name       string
	Args       []string
	Dir        string
	Env        map[string]string
	Stdin      string
	ShowOutput bool
}

type commandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

type commandExecutor interface {
	Run(context.Context, commandSpec) (commandResult, error)
}

type osExecutor struct{}

func (osExecutor) Run(ctx context.Context, spec commandSpec) (commandResult, error) {
	cmd := exec.CommandContext(ctx, spec.Name, spec.Args...)
	cmd.Dir = spec.Dir
	cmd.Env = os.Environ()
	for key, value := range spec.Env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.Stdin = bytes.NewBufferString(spec.Stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if spec.ShowOutput {
		cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	}
	err := cmd.Run()
	result := commandResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if err == nil {
		return result, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}
	return result, fmt.Errorf("start %s: %w", spec.Name, err)
}
