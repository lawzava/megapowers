package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

func checkWindowsHooks(ctx context.Context, hook string, runner commandExecutor) error {
	run := func(args []string, input string, env map[string]string) (commandResult, error) {
		return runner.Run(ctx, commandSpec{Name: "cmd.exe", Args: append([]string{"/c", hook}, args...), Stdin: input, Env: env})
	}
	style, err := run([]string{"output-style"}, "", map[string]string{"MEGAPOWERS_HARNESS": "claude"})
	if err != nil {
		return err
	}
	if style.ExitCode != 0 || strings.TrimSpace(style.Stdout) != "" {
		return fmt.Errorf("output-style hook must pass silently for Claude (exit %d)", style.ExitCode)
	}
	bootstrap, err := run([]string{"session-start"}, `{}`, map[string]string{"MEGAPOWERS_HARNESS": "claude", "MEGAPOWERS_OUTPUT_STYLE": "off"})
	if err != nil {
		return err
	}
	if bootstrap.ExitCode != 0 || !strings.Contains(bootstrap.Stdout, "Do not claim a skill without loading it.") || strings.Contains(bootstrap.Stdout, "ASD-STE100-inspired") {
		return fmt.Errorf("session-start must retain workflow guidance with style disabled (exit %d)", bootstrap.ExitCode)
	}
	codex, err := run([]string{"session-start"}, `{}`, map[string]string{"MEGAPOWERS_HARNESS": "codex", "MEGAPOWERS_OUTPUT_STYLE": ""})
	if err != nil {
		return err
	}
	if codex.ExitCode != 0 || strings.Count(codex.Stdout, "Do not claim a skill without loading it.") != 1 || !strings.Contains(codex.Stdout, "ASD-STE100-inspired") {
		return fmt.Errorf("codex session-start must load style and one workflow reminder (exit %d)", codex.ExitCode)
	}
	safe, err := run([]string{"deny-destructive"}, `{"tool_input":{"command":"git status"}}`, nil)
	if err != nil {
		return err
	}
	if safe.ExitCode != 0 || strings.TrimSpace(safe.Stdout) != "" {
		return fmt.Errorf("safe deny-destructive check failed (exit %d)", safe.ExitCode)
	}
	denied, err := run([]string{"deny-destructive"}, `{"tool_input":{"command":"Remove-Item -Recurse C:\\"}}`, nil)
	if err != nil {
		return err
	}
	if denied.ExitCode != 0 {
		return fmt.Errorf("destructive command classification failed (exit %d)", denied.ExitCode)
	}
	var response struct {
		HookSpecificOutput struct {
			PermissionDecision string `json:"permissionDecision"`
		} `json:"hookSpecificOutput"`
	}
	if err := json.Unmarshal([]byte(denied.Stdout), &response); err != nil || response.HookSpecificOutput.PermissionDecision != "deny" {
		return fmt.Errorf("destructive command did not produce a deny decision")
	}
	invalid, err := run([]string{"deny-destructive"}, `{}`, nil)
	if err != nil {
		return err
	}
	if invalid.ExitCode == 0 || !strings.Contains(invalid.Stderr, "cannot evaluate command input") {
		return fmt.Errorf("run-hook.cmd did not propagate invalid-input failure")
	}
	invalidArguments := []struct {
		label string
		args  []string
	}{
		{label: "missing"},
		{label: "unknown", args: []string{"unknown"}},
		{label: "multiple", args: []string{"output-style", "extra"}},
	}
	for _, check := range invalidArguments {
		result, err := run(check.args, "", nil)
		if err != nil {
			return err
		}
		if result.ExitCode == 0 {
			return fmt.Errorf("run-hook.cmd accepted %s arguments", check.label)
		}
	}
	return nil
}
