package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const releaseKeyFingerprint = "C91B6252A576AEA45DE467D1D75C64FBF43F6479"

const releasePublicKey = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mDMEZ1qpuBYJKwYBBAHaRw8BAQdAB8zu8ZgEjudX3ozrHdu6Zbd5scjhWZ14Pkrt
Eb2ia020ITlhMGZmZWRjNWIzMTgyM2IgPGdpdEB3ZWxsaWNrLmRlPoiTBBMWCgA7
FiEEyRtiUqV2rqRd5GfR11xk+/Q/ZHkFAmdaqbgCGwMFCwkIBwICIgIGFQoJCAsC
BBYCAwECHgcCF4AACgkQ11xk+/Q/ZHm7eQD/VJ0FfOHUWlYxcTpEHY4ubO1eoj2D
DTXenaVOUWZckfIBAKsdqnskxlEEXv3k/ZmkXN0AynNPbmIHcLySB3hw9X0KuDgE
Z1qpuBIKKwYBBAGXVQEFAQEHQKR9aiaS9sgMS+D2EmBi2Fo0v01L6uEnKuLBwfzp
H8FzAwEIB4h4BBgWCgAgFiEEyRtiUqV2rqRd5GfR11xk+/Q/ZHkFAmdaqbgCGwwA
CgkQ11xk+/Q/ZHmZzwEA+/Ur0I6wHLzakcbB0yhuXVSkFSmga7pMUsBxjqlbvV8A
/2gD7zlbPFD31THDdnoDD1yzK2AeblDnnb9BngSp+QML
=cSOL
-----END PGP PUBLIC KEY BLOCK-----
`

func verifyRelease(ctx context.Context, tag, expectedSHA string, runner commandExecutor) (string, error) {
	if !strings.HasPrefix(tag, "v") {
		return "", fmt.Errorf("tag %q does not match v*", tag)
	}
	if expectedSHA != "" && !isLowerHex(expectedSHA, 40) {
		return "", fmt.Errorf("expected_sha must be a 40-hex commit SHA")
	}
	resolved, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"rev-parse", tag + "^{commit}"}})
	if err != nil {
		return "", err
	}
	if resolved.ExitCode != 0 {
		return "", fmt.Errorf("resolve tag %s: %s", tag, commandOutput(resolved))
	}
	sha := strings.TrimSpace(resolved.Stdout)
	if !isLowerHex(sha, 40) {
		return "", fmt.Errorf("tag %s resolved to an invalid commit SHA", tag)
	}
	expected := expectedSHA
	if expected == "" {
		expected = sha
	}
	if sha != expected {
		return "", fmt.Errorf("tag %s points at %s, expected %s", tag, sha, expected)
	}
	imported, err := runner.Run(ctx, commandSpec{Name: "gpg", Args: []string{"--batch", "--import"}, Stdin: releasePublicKey})
	if err != nil {
		return "", err
	}
	if imported.ExitCode != 0 {
		return "", fmt.Errorf("import release key: %s", commandOutput(imported))
	}
	if err := verifySignedObject(ctx, "tag", "verify-tag", tag, runner); err != nil {
		return "", err
	}
	if err := verifySignedObject(ctx, "commit", "verify-commit", sha, runner); err != nil {
		return "", err
	}
	return sha, nil
}

func verifySignedObject(ctx context.Context, kind, gitCommand, value string, runner commandExecutor) error {
	result, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{gitCommand, "--raw", value}})
	if err != nil {
		return err
	}
	output := result.Stdout + result.Stderr
	if result.ExitCode != 0 {
		return fmt.Errorf("git %s %s failed: %s", gitCommand, value, strings.TrimSpace(output))
	}
	want := "[GNUPG:] VALIDSIG " + releaseKeyFingerprint + " "
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, want) {
			return nil
		}
	}
	return fmt.Errorf("%s %s is not signed by %s: %s", kind, value, releaseKeyFingerprint, strings.TrimSpace(output))
}

func isLowerHex(value string, length int) bool {
	if len(value) != length || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func readManifestVersions(claudePath, codexPath string) (string, string, error) {
	read := func(path string) (string, error) {
		content, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		var manifest struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(content, &manifest); err != nil {
			return "", fmt.Errorf("parse %s: %w", path, err)
		}
		if manifest.Version == "" {
			return "", fmt.Errorf("%s has no version", path)
		}
		return manifest.Version, nil
	}
	claudeVersion, err := read(claudePath)
	if err != nil {
		return "", "", err
	}
	codexVersion, err := read(codexPath)
	if err != nil {
		return "", "", err
	}
	if claudeVersion != codexVersion {
		return "", "", fmt.Errorf("manifest versions differ (claude %s, codex %s)", claudeVersion, codexVersion)
	}
	return claudeVersion, codexVersion, nil
}

type pluginTreeResult struct {
	HashManifest string
	ModeManifest string
	TreeHash     string
	ModeHash     string
}

func hashPluginTree(root string) (pluginTreeResult, error) {
	return hashPluginTreeWithPrefix(root, "")
}

func hashPluginTreeWithPrefix(root, displayPrefix string) (pluginTreeResult, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink exists under %s: %s", root, path)
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("special file exists under %s: %s", root, path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		paths = append(paths, rel)
		return nil
	})
	if err != nil {
		return pluginTreeResult{}, err
	}
	sort.Strings(paths)
	var hashes strings.Builder
	var modes strings.Builder
	for _, rel := range paths {
		path := filepath.Join(root, rel)
		content, err := os.ReadFile(path)
		if err != nil {
			return pluginTreeResult{}, err
		}
		info, err := os.Stat(path)
		if err != nil {
			return pluginTreeResult{}, err
		}
		display := filepath.ToSlash(rel)
		if displayPrefix != "" {
			display = strings.TrimSuffix(filepath.ToSlash(displayPrefix), "/") + "/" + display
		}
		if strings.ContainsAny(display, "\\\r\n") {
			return pluginTreeResult{}, fmt.Errorf("path cannot be represented in release manifest: %q", display)
		}
		sum := sha256.Sum256(content)
		fmt.Fprintf(&hashes, "%x  %s\n", sum, display)
		fmt.Fprintf(&modes, "%o %s\n", fileModeBits(info.Mode()), display)
	}
	hashText := hashes.String()
	modeText := modes.String()
	treeSum := sha256.Sum256([]byte(hashText))
	modeSum := sha256.Sum256([]byte(modeText))
	return pluginTreeResult{
		HashManifest: hashText,
		ModeManifest: modeText,
		TreeHash:     hex.EncodeToString(treeSum[:]),
		ModeHash:     hex.EncodeToString(modeSum[:]),
	}, nil
}

func fileModeBits(mode os.FileMode) uint32 {
	bits := uint32(mode.Perm())
	if mode&os.ModeSetuid != 0 {
		bits |= 0o4000
	}
	if mode&os.ModeSetgid != 0 {
		bits |= 0o2000
	}
	if mode&os.ModeSticky != 0 {
		bits |= 0o1000
	}
	return bits
}

type attestationInput struct {
	Version             string
	Tag                 string
	SHA                 string
	RunID               string
	ClaudeVersion       string
	CodexVersion        string
	PluginTreeSHA256    string
	ModesManifestSHA256 string
	SmokeClaude         string
	SmokeCodex          string
}

func emitAttestation(output io.Writer, input attestationInput) error {
	runID, err := strconv.ParseInt(input.RunID, 10, 64)
	if err != nil {
		return fmt.Errorf("run_id must be an integer")
	}
	if runID <= 0 {
		return fmt.Errorf("run_id must be positive")
	}
	if input.Version == "" || input.Tag != "v"+input.Version {
		return fmt.Errorf("tag must equal v plus the release version")
	}
	if !isLowerHex(input.SHA, 40) {
		return fmt.Errorf("sha must be a 40-hex commit SHA")
	}
	if input.ClaudeVersion != input.Version || input.CodexVersion != input.Version {
		return fmt.Errorf("manifest versions must equal the release version")
	}
	if !isLowerHex(input.PluginTreeSHA256, 64) {
		return fmt.Errorf("plugin_tree_sha256 must be a 64-hex SHA-256 digest")
	}
	if !isLowerHex(input.ModesManifestSHA256, 64) {
		return fmt.Errorf("modes_manifest_sha256 must be a 64-hex SHA-256 digest")
	}
	if input.SmokeClaude != "pass" || input.SmokeCodex != "pass" {
		return fmt.Errorf("claude and codex smoke results must both be pass")
	}
	value := struct {
		Version          string `json:"version"`
		Tag              string `json:"tag"`
		SHA              string `json:"sha"`
		TagSignature     string `json:"tag_signature"`
		RunID            int64  `json:"run_id"`
		ManifestVersions struct {
			Claude string `json:"claude"`
			Codex  string `json:"codex"`
		} `json:"manifest_versions"`
		PluginTreeSHA256    string `json:"plugin_tree_sha256"`
		ModesManifestSHA256 string `json:"modes_manifest_sha256"`
		Smoke               struct {
			Claude string `json:"claude"`
			Codex  string `json:"codex"`
		} `json:"smoke"`
	}{
		Version: input.Version, Tag: input.Tag, SHA: input.SHA,
		TagSignature: "verified", RunID: runID,
		PluginTreeSHA256: input.PluginTreeSHA256, ModesManifestSHA256: input.ModesManifestSHA256,
	}
	value.ManifestVersions.Claude = input.ClaudeVersion
	value.ManifestVersions.Codex = input.CodexVersion
	value.Smoke.Claude = input.SmokeClaude
	value.Smoke.Codex = input.SmokeCodex
	encoder := json.NewEncoder(output)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func fastForwardRelease(ctx context.Context, tag string, runner commandExecutor) error {
	if !strings.HasPrefix(tag, "v") {
		return fmt.Errorf("tag %q does not match v*", tag)
	}
	resolved, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"rev-parse", tag + "^{commit}"}})
	if err != nil {
		return err
	}
	if resolved.ExitCode != 0 {
		return fmt.Errorf("resolve tag %s: %s", tag, commandOutput(resolved))
	}
	sha := strings.TrimSpace(resolved.Stdout)
	if !isLowerHex(sha, 40) {
		return fmt.Errorf("tag %s resolved to an invalid commit SHA", tag)
	}
	remote, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"ls-remote", "--exit-code", "--heads", "origin", "release"}})
	if err != nil {
		return err
	}
	switch remote.ExitCode {
	case 0:
		fetch, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"fetch", "origin", "release"}})
		if err != nil {
			return err
		}
		if fetch.ExitCode != 0 {
			return fmt.Errorf("fetch release branch: %s", commandOutput(fetch))
		}
		ancestor, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"merge-base", "--is-ancestor", "origin/release", sha}})
		if err != nil {
			return err
		}
		if ancestor.ExitCode != 0 {
			return fmt.Errorf("release branch is not an ancestor of %s; refusing non-fast-forward", tag)
		}
	case 2:
		// Exit 2 means no matching remote branch. The first release may create it.
	default:
		return fmt.Errorf("inspect remote release branch: %s", commandOutput(remote))
	}
	push, err := runner.Run(ctx, commandSpec{Name: "git", Args: []string{"push", "origin", sha + ":refs/heads/release"}})
	if err != nil {
		return err
	}
	if push.ExitCode != 0 {
		return fmt.Errorf("push release branch: %s", commandOutput(push))
	}
	return nil
}

func requireSmokePass(path, harness string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	result := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), "\t")
		if len(fields) >= 2 && fields[0] == harness {
			result = strings.ToLower(fields[1])
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if result != "pass" {
		if result == "" {
			result = "missing"
		}
		return fmt.Errorf("unexpected %s smoke result: %s", harness, result)
	}
	return nil
}

func commandOutput(result commandResult) string {
	output := strings.TrimSpace(result.Stdout + result.Stderr)
	if output == "" {
		return fmt.Sprintf("exit %d", result.ExitCode)
	}
	return output
}
