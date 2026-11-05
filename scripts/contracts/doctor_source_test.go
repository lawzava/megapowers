package contracts

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"testing"
)

// Keep the standalone diagnostic's dependencies and file access inside the
// plugin tree. Runtime fixtures separately exercise root confinement.
func doctorSourceViolations(source string) []string {
	f, err := parser.ParseFile(token.NewFileSet(), "doctor.go", source, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var violations []string
	allowedImports := map[string]bool{"encoding/json": true, "flag": true, "fmt": true, "io": true, "os": true, "strings": true}
	for _, imp := range f.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		if !allowedImports[path] || imp.Name != nil {
			violations = append(violations, "unexpected import: "+path)
		}
	}
	roots := map[string]bool{"MEGAPOWERS_PLUGIN_ROOT": true, "PLUGIN_ROOT": true, "CLAUDE_PLUGIN_ROOT": true}
	allowedOS := map[string]bool{"OpenRoot": true, "IsNotExist": true, "Getenv": true, "Exit": true, "Stdout": true, "Stderr": true, "Root": true}
	ast.Inspect(f, func(n ast.Node) bool {
		if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			value, _ := strconv.Unquote(lit.Value)
			if value == "HOME" || value == ".claude" || value == ".config" {
				violations = append(violations, "outside-tree literal: "+value)
			}
		}
		if sel, ok := n.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "os" && !allowedOS[sel.Sel.Name] {
				violations = append(violations, "unexpected os access: "+sel.Sel.Name)
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "Getenv" {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "os" {
			return true
		}
		key := ""
		if len(call.Args) == 1 {
			if lit, ok := call.Args[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
				key, _ = strconv.Unquote(lit.Value)
			}
		}
		if !roots[key] {
			violations = append(violations, fmt.Sprintf("Getenv requires a literal plugin root key, got %q", key))
		}
		return true
	})
	return violations
}

func TestDoctorSourceBoundary(t *testing.T) {
	source := read(t, repoRoot(t), "scripts/cmd/doctor/main.go")
	if violations := doctorSourceViolations(source); len(violations) != 0 {
		t.Fatalf("doctor source boundary: %v", violations)
	}
}

func TestDoctorSourceBoundaryRejectsOutsideAccess(t *testing.T) {
	for _, source := range []string{
		`package main; import "os/exec"`,
		`package main; import "path/filepath"`,
		`package main; import o "os"`,
		`package main; import "os"; func f(){ os.Getenv("HOME") }`,
		`package main; import "os"; func f(key string){ os.Getenv(key) }`,
		`package main; import "os"; func f(){ os.ReadFile("file") }`,
		`package main; import "os"; func f(){ os.Open("file") }`,
		`package main; import "os"; func f(){ os.Stat("file") }`,
		`package main; import "os"; func f(){ os.UserHomeDir() }`,
		`package main; const path=".claude"`,
		`package main; const path=".config"`,
	} {
		if len(doctorSourceViolations(source)) == 0 {
			t.Errorf("accepted outside access: %s", source)
		}
	}
	for _, key := range []string{"MEGAPOWERS_PLUGIN_ROOT", "PLUGIN_ROOT", "CLAUDE_PLUGIN_ROOT"} {
		source := fmt.Sprintf(`package main; import "os"; func f(){ os.Getenv(%q) }`, key)
		if got := doctorSourceViolations(source); len(got) != 0 {
			t.Errorf("root key rejected: %v", got)
		}
	}
}
