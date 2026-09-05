package main

import (
	"strings"
	"testing"
)

func TestClassifyCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command string
		deny    bool
	}{
		{name: "scoped cleanup", command: `rm -rf ./dist`},
		{name: "quoted data", command: `echo "rm -rf /"`},
		{name: "root wipe", command: `rm -rf /`, deny: true},
		{name: "wrapped root wipe", command: `sudo timeout 10 rm -rf /etc`, deny: true},
		{name: "raw disk overwrite", command: `dd if=/dev/zero of=/dev/sda`, deny: true},
		{name: "nested find payload", command: `find . -exec sh -c "rm -rf /" ;`, deny: true},
		{name: "PowerShell drive wipe", command: `Remove-Item -Recurse -Force C:\`, deny: true},
		{name: "PowerShell mixed lower item", command: `Remove-item -Recurse /`, deny: true},
		{name: "PowerShell mixed upper item", command: `remove-Item -Recurse /`, deny: true},
		{name: "root dotfile wipe", command: `rm -rf /.*`, deny: true},
		{name: "find root dotfile wipe", command: `find /.* -delete`, deny: true},
		{name: "spaced fork bomb", command: `bomb()    { bomb | bomb & }`, deny: true},
		{name: "reversible risk stays with harness", command: `git reset --hard HEAD~1`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := classifyCommand(tt.command, "/home/tester")
			if got.Deny != tt.deny {
				t.Fatalf("classifyCommand(%q).Deny = %v, want %v (reason %q)", tt.command, got.Deny, tt.deny, got.Reason)
			}
		})
	}
}

func TestCommandLengthBoundary(t *testing.T) {
	t.Parallel()

	padding := strings.Repeat("echo 'git find rm chunk of ordinary work';", 150)
	if len(padding) < 6_000 {
		t.Fatalf("test padding is only %d bytes", len(padding))
	}
	for _, tt := range []struct {
		name    string
		command string
		deny    bool
	}{
		{name: "ordinary long command", command: padding},
		{name: "catastrophe within parsing budget", command: padding + " rm -rf /", deny: true},
		{name: "reversible long command", command: padding + " git reset --hard"},
		{name: "over budget delegates to harness", command: strings.Repeat("echo 'git ok';", 1_200) + " rm -rf /"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyCommand(tt.command, "/home/tester"); got.Deny != tt.deny {
				t.Fatalf("Deny = %v, want %v (command bytes %d, reason %q)", got.Deny, tt.deny, len(tt.command), got.Reason)
			}
		})
	}
}

func TestMultibyteQuotedData(t *testing.T) {
	t.Parallel()
	for _, command := range []string{
		`echo "日本語 → rm -rf / ✓"`,
		`git commit -m "fix: don't run rm -rf / — ünicode näme"`,
	} {
		if got := classifyCommand(command, "/home/tester"); got.Deny {
			t.Fatalf("classifyCommand(%q) denied quoted data: %s", command, got.Reason)
		}
	}
}
