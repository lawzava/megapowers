package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type getenvFunc func(string) string

const maxHookInputBytes = 256 << 10

const hookUsage = "expected deny-destructive, session-start, subagent-start, output-style, or doctor"

type preToolUseInput struct {
	SessionID string `json:"session_id"`
	ToolInput struct {
		Command string `json:"command"`
	} `json:"tool_input"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecificOutput `json:"hookSpecificOutput"`
}

type hookSpecificOutput struct {
	HookEventName            string `json:"hookEventName"`
	PermissionDecision       string `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string `json:"permissionDecisionReason,omitempty"`
	AdditionalContext        string `json:"additionalContext,omitempty"`
}

// The launcher checks the Go toolchain before building and keys the cached
// runner on the Go version, so the binary itself performs no version check:
// a runtime check here would turn one stale build into a permanent failure.
func main() {
	os.Exit(runHook(os.Args[1:], os.Getenv, os.Stdin, os.Stdout, os.Stderr))
}

func runHook(args []string, getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(errors, "megapowers hook: cannot run hook: "+hookUsage)
		return 1
	}
	switch args[0] {
	case "deny-destructive":
		return runDenyDestructive(getenv, input, output, errors)
	case "output-style":
		return runOutputStyle(getenv, input, output, errors)
	case "session-start":
		return runSessionStart(getenv, input, output, errors)
	case "subagent-start":
		return runSubagentStart(getenv, input, output, errors)
	case "doctor":
		return runDoctor(getenv, output, errors)
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

// runSubagentStart hands every subagent the skill-loading reminder and, unless
// the operator turned prose styling off, a compact report contract. Both
// harnesses accept the SubagentStart hookSpecificOutput.additionalContext shape.
func runSubagentStart(getenv getenvFunc, input io.Reader, output, errors io.Writer) int {
	if _, err := readHookInput(input); err != nil {
		fmt.Fprintln(errors, "megapowers subagent start: cannot read hook input")
		return 1
	}
	context := strings.TrimSpace(skillLoadingReminder)
	if getenv("MEGAPOWERS_OUTPUT_STYLE") != "off" {
		context += "\n\n" + strings.TrimSpace(subagentReportContract)
	}
	if err := emitHookOutput(output, hookSpecificOutput{HookEventName: "SubagentStart", AdditionalContext: context}); err != nil {
		fmt.Fprintln(errors, "megapowers subagent start: cannot emit context")
		return 1
	}
	return 0
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
	if verdict.Deny {
		err = emitHookOutput(output, hookSpecificOutput{
			HookEventName:            "PreToolUse",
			PermissionDecision:       "deny",
			PermissionDecisionReason: verdict.Reason,
		})
		if err != nil {
			fmt.Fprintln(errors, "megapowers destructive guard: cannot emit decision")
			return 1
		}
		return 0
	}

	// Allowed commands may still deserve a non-blocking skill reminder.
	context := gateContext(event.ToolInput.Command, event.SessionID, getenv)
	if context == "" {
		return 0
	}
	if err := emitHookOutput(output, hookSpecificOutput{HookEventName: "PreToolUse", AdditionalContext: context}); err != nil {
		fmt.Fprintln(errors, "megapowers destructive guard: cannot emit context")
		return 1
	}
	return 0
}

func emitHookOutput(output io.Writer, specific hookSpecificOutput) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(hookOutput{HookSpecificOutput: specific})
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

// requireGo125 reports whether a Go version string names a supported
// toolchain. The doctor uses it to explain a launcher build refusal.
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
