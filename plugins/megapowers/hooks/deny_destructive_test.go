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
		{name: "double-quoted program", command: `"rm" -rf ~`, deny: true},
		{name: "quoted absolute program", command: `"/bin/rm" -rf /home`, deny: true},
		{name: "single-quoted program", command: `'rm' -rf /`, deny: true},
		{name: "partly quoted program", command: `r"m" -rf /etc`, deny: true},
		{name: "quoted wrapper", command: `"sudo" rm -rf /`, deny: true},
		{name: "quoted program scoped cleanup", command: `"rm" -rf ./dist`},
		{name: "PowerShell profile wipe", command: `Remove-Item -Recurse -Force $env:USERPROFILE`, deny: true},
		{name: "PowerShell profile contents wipe", command: `Remove-Item -Recurse -Force $env:USERPROFILE\*`, deny: true},
		{name: "PowerShell lower-case profile", command: `Remove-Item -Recurse $env:userprofile`, deny: true},
		{name: "PowerShell HOME variable", command: `Remove-Item -Recurse -Force $HOME`, deny: true},
		{name: "PowerShell profile subdirectory", command: `Remove-Item -Recurse -Force $env:USERPROFILE\project\dist`},
		{name: "Windows profile path", command: `Remove-Item -Recurse -Force C:\Users\alice`, deny: true},
		{name: "Windows profile subdirectory", command: `Remove-Item -Recurse -Force C:\Users\alice\project\dist`},
		{name: "nested shell with escaped quotes", command: `bash -c "bash -c \"rm -rf ~\""`, deny: true},
		{name: "command substitution in double quotes", command: `echo "$(rm -rf ~)"`, deny: true},
		{name: "bare command substitution", command: `echo $(rm -rf /etc)`, deny: true},
		{name: "backtick substitution in double quotes", command: "echo \"`rm -rf /`\"", deny: true},
		{name: "substitution in single quotes is data", command: `echo '$(rm -rf ~)'`},
		{name: "escaped substitution is data", command: `echo "\$(rm -rf ~)"`},
		{name: "quoted raw device redirect", command: `cat image.bin > "/dev/sda"`, deny: true},
		{name: "single-quoted raw device redirect", command: `cat image.bin >'/dev/nvme0n1'`, deny: true},
		{name: "quoted device path as data", command: `echo "> /dev/sda"`},
		{name: "find match-all name filter", command: `find ~ -name '*' -delete`, deny: true},
		{name: "find unfiltered or branch", command: `find ~ -name '*.tmp' -o -delete`, deny: true},
		{name: "find filtered home cleanup", command: `find ~ -name '*.tmp' -delete`},
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

func TestFindDenialAdviceMentionsFilter(t *testing.T) {
	t.Parallel()

	got := classifyCommand("find ~ -delete", "/home/tester")
	if !got.Deny {
		t.Fatalf("find ~ -delete was allowed")
	}
	if strings.Contains(got.Reason, "Use a specific relative start path") || !strings.Contains(got.Reason, "-name") {
		t.Fatalf("find denial advice does not explain the filter route: %q", got.Reason)
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
