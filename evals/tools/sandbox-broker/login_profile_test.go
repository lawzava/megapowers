package main

import (
	"os/exec"
	"strings"
	"testing"
)

// Codex runs every command through `bash -lc`. A login shell re-reads the
// system profile, which replaced PATH and hid the receipt wrappers and Go.
func TestLoginShellKeepsBrokerPath(t *testing.T) {
	home := t.TempDir()
	if err := writeLoginProfiles(home); err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", "-lc", `printf %s "$PATH"`)
	command.Env = []string{"HOME=" + home, "PATH=" + actorPath}
	out, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(out), "/opt/megapowers-receipt/bin:/opt/megapowers-runtime/go/bin:") {
		t.Fatalf("login shell PATH = %q, want the broker path first", out)
	}
}

// Codex wraps commands as `/opt/megapowers-receipt/bin/bash -lc '...'`; the
// receipt wrapper path must not hide the inner command from classification.
func TestCommandEventsSeeThroughReceiptShellWrapper(t *testing.T) {
	for _, command := range []string{
		`/opt/megapowers-receipt/bin/bash -lc 'git add rate.go && git commit -m "fix: x"'`,
		`/opt/megapowers-receipt/bin/sh -c 'git commit -m fix'`,
		`bash -c 'git commit -m fix'`,
	} {
		found := false
		for _, event := range commandEvents(command, 0) {
			found = found || event.Kind == "git_commit"
		}
		if !found {
			t.Errorf("%s: no git_commit event", command)
		}
	}
}

// shell_environment_policy.inherit="none" started every Codex command with no
// PATH or HOME, so go was missing and the login profile never ran.
func TestCodexShellEnvironmentSetsActorPathAndHome(t *testing.T) {
	req := brokerRequest{ActorHome: "/home/actor"}
	config := codexThreadConfig(req)
	policy, _ := config["shell_environment_policy"].(map[string]any)
	set, _ := policy["set"].(map[string]string)
	if policy["inherit"] != "none" || set["PATH"] != actorPath || set["HOME"] != "/home/actor" || set["TMPDIR"] != "/tmp" {
		t.Fatalf("shell_environment_policy = %#v", policy)
	}
	for key := range set {
		if strings.Contains(strings.ToUpper(key), "TOKEN") || strings.Contains(strings.ToUpper(key), "KEY") {
			t.Fatalf("shell environment must not carry credentials: %s", key)
		}
	}
	args := strings.Join(codexExecEnvironmentArgs(req), " ")
	if !strings.Contains(args, `shell_environment_policy.set.PATH="`+actorPath+`"`) || !strings.Contains(args, `shell_environment_policy.set.HOME="/home/actor"`) {
		t.Fatalf("exec args = %s", args)
	}
}
