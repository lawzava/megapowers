package contracts

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSessionMineLocalBoundary(t *testing.T) {
	root := repoRoot(t)
	tool := buildSessionMine(t, root)
	base := []string{"--all-workspaces", "--claude-root", t.TempDir(), "--codex-root", t.TempDir(), "--format", "json"}
	for _, removed := range [][]string{{"--opencode-cmd", "unused"}, {"--opencode-timeout", "1s"}, {"--harness", "opencode"}, {"--harness", "all"}} {
		if out, code := run(t, root, nil, tool, append(append([]string{}, base...), removed...)...); code != 2 {
			t.Errorf("removed option accepted (%d): %s", code, out)
		}
	}
	t.Setenv("SESSION_MINE_OPENCODE_CMD", "/does/not/exist")
	t.Setenv("SESSION_MINE_OPENCODE_TIMEOUT", "invalid")
	out, code := run(t, root, nil, tool, base...)
	var report struct {
		BySource map[string]json.RawMessage `json:"by_source"`
	}
	if code != 0 || json.Unmarshal([]byte(out), &report) != nil || len(report.BySource) != 2 || report.BySource["claude"] == nil || report.BySource["codex"] == nil {
		t.Fatalf("default local sources (%d): %s", code, out)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(out), &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"skipped_sources", "partial_sources", "skipped_sessions", "deduplicated_sessions"} {
		if string(fields[key]) != "0" {
			t.Errorf("compatibility field %s: %s", key, fields[key])
		}
	}
	textArgs := append([]string{}, base[:len(base)-1]...)
	textArgs = append(textArgs, "text")
	if out, code := run(t, root, nil, tool, textArgs...); code != 0 || !strings.Contains(out, "sources: skipped 0, partial 0, skipped sessions 0, deduplicated sessions 0") {
		t.Fatalf("text compatibility (%d): %s", code, out)
	}
}

func TestShippedDocsNoLoginCLIInstructions(t *testing.T) {
	root := repoRoot(t)
	for _, dir := range []string{"plugins/megapowers", "docs"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".md") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			body := strings.ToLower(string(data))
			for _, removed := range []string{"opencode api", "opencode service", "--opencode-cmd", "--opencode-timeout", "installed agent cli", "codex --ask-for-approval"} {
				if strings.Contains(body, removed) {
					t.Errorf("%s contains login CLI instruction %q", path, removed)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestSessionMineInstalledSingleFile(t *testing.T) {
	root := repoRoot(t)
	dir := t.TempDir()
	source, err := os.ReadFile(filepath.Join(root, sessionMineTool))
	if err != nil {
		t.Fatal(err)
	}
	installed := filepath.Join(dir, "session-mine.go")
	if err := os.WriteFile(installed, source, 0600); err != nil {
		t.Fatal(err)
	}
	output, code := run(t, dir, nil, "go", "run", installed, "corrections", "--harness", "both", "--all-workspaces", "--claude-root", dir, "--codex-root", dir, "--format", "json")
	var report sessionMineCorrections
	if code != 0 || json.Unmarshal([]byte(output), &report) != nil || report.SessionsScanned != 0 {
		t.Fatalf("standalone installed copy (%d): %s", code, output)
	}
}

func TestShippedGoNoAgentCLI(t *testing.T) {
	root := repoRoot(t)
	err := filepath.WalkDir(filepath.Join(root, "extras/scripts"), func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if err != nil {
			return err
		}
		for _, imp := range file.Imports {
			if strings.HasSuffix(path, "session-mine.go") && imp.Path.Value == `"os/exec"` {
				t.Errorf("local transcript reader imports os/exec")
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "exec" {
				return true
			}
			index := 0
			if selector.Sel.Name == "CommandContext" {
				index = 1
			} else if selector.Sel.Name != "Command" {
				return true
			}
			if len(call.Args) <= index {
				return true
			}
			name := ""
			switch arg := call.Args[index].(type) {
			case *ast.BasicLit:
				name, _ = strconv.Unquote(arg.Value)
			case *ast.SelectorExpr:
				name = arg.Sel.Name
			case *ast.Ident:
				name = arg.Name
			}
			for _, agent := range []string{"opencode", "codex", "claude", "agy"} {
				if strings.Contains(strings.ToLower(filepath.Base(name)), agent) {
					t.Errorf("%s invokes agent CLI %q", path, name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
