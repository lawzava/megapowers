package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"strconv"
	"strings"
)

type getenvFunc func(string) string

const maxHookInputBytes = 256 << 10

type preToolUseInput struct {
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision"`
	PermissionDecisionReason string `json:"permissionDecisionReason"`
}

func main() {
	if err := requireGo125(runtime.Version()); err != nil {
		fmt.Fprintf(os.Stderr, "megapowers hook: %v\n", err)
		os.Exit(1)
	}
	os.Exit(runHook(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}

func runHook(args []string, getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(errors, "megapowers hook: cannot run hook: expected deny-destructive, session-start, or output-style")
		return 1
	}
	switch args[0] {
	case "deny-destructive":
		return runDenyDestructive(getenv, input, output, errors)
	case "output-style":
		return runOutputStyle(getenv, input, output, errors)
	case "session-start":
		return runSessionStart(getenv, input, output, errors)
	default:
		fmt.Fprintf(errors, "megapowers hook: cannot run unknown hook: %s\n", args[0])
		return 1
	}
}

func runSessionStart(getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	if _, err := readHookInput(input); err != nil {
		fmt.Fprintln(errors, "megapowers session start: cannot read hook input")
		return 1
	}
	// Workflow guidance remains active when the operator disables prose styling.
	if _, err := io.WriteString(output, skillLoadingReminder); err != nil {
		fmt.Fprintln(errors, "megapowers session start: cannot emit workflow guidance")
		return 1
	}
	return runOutputStyle(getenv, strings.NewReader(""), output, errors)
}

func runDenyDestructive(getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	payload, err := readHookInput(input)
	if err != nil {
		fmt.Fprintln(errors, "megapowers destructive guard: cannot evaluate command input")
		return 1
	}
	var event preToolUseInput
	if err := json.Unmarshal(payload, &event); err != nil || event.ToolInput.Command == "" {
		fmt.Fprintln(errors, "megapowers destructive guard: cannot evaluate command input")
		return 1
	}

	verdict := classifyCommand(event.ToolInput.Command, getenv("HOME"))
	if !verdict.Deny {
		return 0
	}
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(hookOutput{HookSpecificOutput: hookSpecificOutput{
		HookEventName:            "PreToolUse",
		PermissionDecision:       "deny",
		PermissionDecisionReason: verdict.Reason,
	}})
	if err != nil {
		fmt.Fprintln(errors, "megapowers destructive guard: cannot emit decision")
		return 1
	}
	return 0
}

func runOutputStyle(getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	if !outputStyleEnabled(getenv) {
		return 0
	}
	if _, err := readHookInput(input); err != nil {
		fmt.Fprintln(errors, "megapowers Codex output style: cannot read hook input")
		return 1
	}
	if err := writeOutputStyle(outputStylePath(getenv), output); err != nil {
		fmt.Fprintln(errors, "megapowers Codex output style: shared style is missing")
		return 1
	}
	return 0
}

func readHookInput(input io.Reader) ([]byte, error) {
	payload, err := io.ReadAll(io.LimitReader(input, maxHookInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(payload) > maxHookInputBytes {
		return nil, fmt.Errorf("hook input exceeds %d bytes", maxHookInputBytes)
	}
	return payload, nil
}

func requireGo125(version string) error {
	original := version
	version = strings.TrimPrefix(version, "devel ")
	version = strings.TrimPrefix(version, "go")
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return fmt.Errorf("requires Go 1.25 or newer (running %s)", original)
	}
	major, majorErr := strconv.Atoi(parts[0])
	minorText := strings.TrimLeftFunc(parts[1], func(r rune) bool { return r < '0' || r > '9' })
	minorEnd := strings.IndexFunc(minorText, func(r rune) bool { return r < '0' || r > '9' })
	if minorEnd >= 0 {
		minorText = minorText[:minorEnd]
	}
	minor, minorErr := strconv.Atoi(minorText)
	if majorErr != nil || minorErr != nil || major < 1 || (major == 1 && minor < 25) {
		return fmt.Errorf("requires Go 1.25 or newer (running %s)", original)
	}
	return nil
}
