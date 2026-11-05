package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

func runStaticcheck(ctx context.Context, version string, runner commandExecutor) error {
	result, err := runner.Run(ctx, commandSpec{
		Name: "go", Args: []string{"run", "honnef.co/go/tools/cmd/staticcheck@" + version, "./..."}, ShowOutput: true,
	})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("staticcheck module exited %d", result.ExitCode)
	}
	return nil
}

func requireEvalSuccess(evalOutcome, scoreOutcome string) error {
	if evalOutcome != "success" {
		return fmt.Errorf("deterministic eval step outcome is %q", evalOutcome)
	}
	if scoreOutcome != "success" {
		return fmt.Errorf("deterministic score step outcome is %q", scoreOutcome)
	}
	return nil
}

func runReleaseSmoke(ctx context.Context, getenv func(string) string, runner commandExecutor) error {
	harness := getenv("HARNESS")
	githubEnv := getenv("GITHUB_ENV")
	if harness == "codex" {
		result, err := runner.Run(ctx, commandSpec{
			Name: "go", Args: []string{"run", "./scripts/cmd/maintainer", "codex-install-smoke"}, ShowOutput: true,
		})
		if err != nil {
			return err
		}
		if result.ExitCode != 0 {
			return fmt.Errorf("codex install smoke: %s", commandOutput(result))
		}
		return appendWorkflowValues(githubEnv, "SMOKE_CODEX=pass")
	}
	if harness != "claude" {
		return fmt.Errorf("HARNESS must be claude or codex")
	}
	out, err := os.MkdirTemp("", "megapowers-release-smoke-claude-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)
	result, err := runner.Run(ctx, commandSpec{Name: "go", Args: []string{
		"run", "./scripts/cmd/maintainer", "install-smoke",
		"--harnesses", "claude", "--out", out,
		"--source", getenv("GITHUB_REPOSITORY"), "--ref", getenv("TAG"), "--version", getenv("VERSION"),
	}, ShowOutput: true})
	if err != nil {
		return err
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("claude install smoke: %s", commandOutput(result))
	}
	if err := requireSmokePass(filepath.Join(out, "results.tsv"), "claude"); err != nil {
		return err
	}
	return appendWorkflowValues(githubEnv, "SMOKE_CLAUDE=pass")
}
