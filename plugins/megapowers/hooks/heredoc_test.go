package main

import "testing"

func TestHeredocClassification(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, command string
		deny          bool
	}{
		{"literal", "cat <<'EOF'\nrm -rf /\nEOF", false},
		{"unquoted data", "cat <<EOF\nrm -rf /\nEOF", false},
		{"tabs", "cat <<-'EOF'\n\trm -rf /\n\tEOF", false},
		{"two bodies", "cat <<ONE <<'TWO'\nrm -rf /\nONE\nmkfs.ext4 /dev/sda\nTWO", false},
		{"quoted substitution", "cat <<'EOF'\n$(rm -rf /)\nEOF", false},
		{"expanding substitution", "cat <<EOF\n$(rm -rf /)\nEOF", true},
		{"quotes in expanding data", "cat <<EOF\n'$(rm -rf /)'\nEOF", true},
		{"shell input", "sh <<'EOF'\nrm -rf /\nEOF", true},
		{"remote shell input", "ssh host <<'EOF'\nrm -rf /\nEOF", true},
		{"digit-ending host", "ssh web1<<'EOF'\nrm -rf /\nEOF", true},
		{"numeric host", "ssh 10.0.0.1<<'EOF'\nrm -rf /\nEOF", true},
		{"digit-ending argument", "bash -s arg1<<'EOF'\nrm -rf /\nEOF", true},
		{"remote explicit shell", "ssh host bash -s <<'EOF'\nrm -rf /\nEOF", true},
		{"remote data input", "ssh host cat <<'EOF'\nrm -rf /\nEOF", false},
		{"arithmetic shift", "x=$((1<<20))\nrm -rf /", true},
		{"arithmetic shift assignment", "(( x <<= 1 ))\nrm -rf /", true},
		{"unterminated heredoc", "cat <<'EOF'\nrm -rf /", true},
		{"explicit stdin", "bash 0<<'EOF'\nrm -rf /\nEOF", true},
		{"other descriptor", "bash 3<<'EOF'\nrm -rf /\nEOF", false},
		{"syntax check", "bash -n <<'EOF'\nrm -rf /\nEOF", false},
		{"unrelated shell", "cat <<'EOF' ; bash </dev/null\nrm -rf /\nEOF", false},
		{"commented shell", "cat <<'EOF' # ; bash\nrm -rf /\nEOF", false},
		{"commented pipe", "cat <<'EOF' # | sh\nrm -rf /\nEOF", false},
		{"piped shell", "cat <<'EOF' | sh\nrm -rf /\nEOF", true},
		{"stderr piped shell", "cat <<'EOF' |& sh\nrm -rf /\nEOF", true},
		{"substitution shell", "out=$(bash <<'EOF'\nrm -rf /\nEOF\n)", true},
		{"subshell", "(sh <<'EOF'\nrm -rf /\nEOF\n)", true},
		{"backtick shell", "out=`sh <<'EOF'\nrm -rf /\nEOF\n`", true},
		{"substitution data", "out=$(cat <<'EOF'\nrm -rf /\nEOF\n)", false},
		{"shell script input", "sh script.sh <<'EOF'\nrm -rf /\nEOF", false},
		{"shell command input", "sh -c cat <<'EOF'\nrm -rf /\nEOF", false},
		{"after delimiter", "cat <<'EOF'\nsafe\nEOF\nrm -rf /", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := classifyCommand(tc.command, "/home/tester")
			if got.Deny != tc.deny {
				t.Fatalf("Deny=%v, want %v: %s", got.Deny, tc.deny, got.Reason)
			}
		})
	}
}

func TestFindRegexDoesNotExemptWholeHomeDeletion(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		pattern string
		deny    bool
	}{
		{".+", true}, {"^.*$", true}, {".*", true}, {"..*", true}, {".*.*", true},
		{`.*\.tmp`, false}, {`/home/tester/cache/.*`, false},
	} {
		t.Run(tc.pattern, func(t *testing.T) {
			got := classifyCommand("find /home/tester -regex '"+tc.pattern+"' -delete", "/home/tester")
			if got.Deny != tc.deny {
				t.Fatalf("Deny=%v, want %v: %s", got.Deny, tc.deny, got.Reason)
			}
		})
	}
}

func TestHeredocPreservesExecutableGitDiscard(t *testing.T) {
	for _, tc := range []struct {
		command string
		discard bool
	}{
		{"cat <<'EOF'\ngit reset --hard\nEOF", false},
		{"sh <<'EOF'\ngit reset --hard\nEOF", true},
		{"cat <<EOF | sh\ngit clean -fd\nEOF", true},
		{"x=$((1<<20))\ngit reset --hard", true},
		{"ssh host <<'EOF'\ngit reset --hard\nEOF", true},
		{"ssh db1<<'EOF'\ngit reset --hard\nEOF", true},
		{"out=$(sh <<'EOF'\ngit reset --hard\nEOF\n)", true},
		{"cat <<'EOF' |& sh\ngit reset --hard\nEOF", true},
	} {
		if got := gitDiscardKey(tc.command, "/repo") != ""; got != tc.discard {
			t.Errorf("discard=%v, want %v: %s", got, tc.discard, tc.command)
		}
	}
}
