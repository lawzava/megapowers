package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	os.Exit(runCLI(context.Background(), os.Args[1:], os.Getenv, osExecutor{}, os.Stdout, os.Stderr))
}

func runCLI(ctx context.Context, args []string, getenv func(string) string, runner commandExecutor, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: ci <staticcheck|eval-gate|release-verify|manifest-versions|release-smoke|plugin-tree|attestation|release-fast-forward|windows-hooks>")
		return 2
	}
	var err error
	switch args[0] {
	case "staticcheck":
		err = runStaticcheck(ctx, "v0.6.1", runner)
	case "eval-gate":
		err = requireEvalSuccess(getenv("EVAL_OUTCOME"), getenv("SCORE_OUTCOME"))
	case "release-verify":
		var sha string
		sha, err = verifyRelease(ctx, getenv("TAG"), getenv("EXPECTED_SHA"), runner)
		if err == nil {
			err = appendWorkflowValues(getenv("GITHUB_OUTPUT"), "tag="+getenv("TAG"), "sha="+sha)
		}
	case "manifest-versions":
		var claudeVersion, codexVersion string
		claudeVersion, codexVersion, err = readManifestVersions(
			"plugins/megapowers/.claude-plugin/plugin.json",
			"plugins/megapowers/.codex-plugin/plugin.json",
		)
		if err == nil {
			err = appendWorkflowValues(getenv("GITHUB_OUTPUT"), "claude="+claudeVersion, "codex="+codexVersion)
		}
	case "release-smoke":
		err = runReleaseSmoke(ctx, getenv, runner)
	case "plugin-tree":
		var result pluginTreeResult
		result, err = hashPluginTreeWithPrefix("plugins/megapowers", "plugins/megapowers")
		if err == nil {
			fmt.Fprintln(stdout, "plugin tree sha256sum manifest:")
			fmt.Fprint(stdout, result.HashManifest)
			fmt.Fprintln(stdout, "plugin modes manifest:")
			fmt.Fprint(stdout, result.ModeManifest)
			err = appendWorkflowValues(getenv("GITHUB_OUTPUT"), "tree="+result.TreeHash, "modes="+result.ModeHash)
		}
	case "attestation":
		var content bytes.Buffer
		err = emitAttestation(&content, attestationInput{
			Version: getenv("CLAUDE_VERSION"), Tag: getenv("TAG"), SHA: getenv("SHA"), RunID: getenv("RUN_ID"),
			ClaudeVersion: getenv("CLAUDE_VERSION"), CodexVersion: getenv("CODEX_VERSION"),
			PluginTreeSHA256: getenv("PLUGIN_TREE_SHA256"), ModesManifestSHA256: getenv("MODES_MANIFEST_SHA256"),
			SmokeClaude: getenv("SMOKE_CLAUDE"), SmokeCodex: getenv("SMOKE_CODEX"),
		})
		if err == nil {
			path := getenv("ATTESTATION_PATH")
			if path == "" {
				path = "attestation.json"
			}
			err = os.WriteFile(path, content.Bytes(), 0o644)
			if err == nil {
				_, err = io.Copy(stdout, &content)
			}
		}
	case "release-fast-forward":
		err = fastForwardRelease(ctx, getenv("TAG"), runner)
	case "windows-hooks":
		err = checkWindowsHooks(ctx, filepath.FromSlash("plugins/megapowers/hooks/run-hook.cmd"), runner)
	default:
		fmt.Fprintf(stderr, "ci: unknown command %q\n", args[0])
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "ci %s: %v\n", args[0], err)
		return 1
	}
	return 0
}

func appendWorkflowValues(path string, values ...string) error {
	if path == "" {
		return fmt.Errorf("workflow output path is empty")
	}
	for _, value := range values {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("workflow output value contains a line break")
		}
	}
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open workflow output: %w", err)
	}
	defer file.Close()
	for _, value := range values {
		if _, err := fmt.Fprintln(file, value); err != nil {
			return fmt.Errorf("write workflow output: %w", err)
		}
	}
	return nil
}
