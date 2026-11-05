package maintain

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHookCommandsUseWrapper(t *testing.T) {
	const wrapper = `"\"${CLAUDE_PLUGIN_ROOT}\"/hooks/run-hook.cmd `
	for _, test := range []struct {
		name, manifest string
		want           bool
	}{
		{"four wrapped commands", `{"hooks":{"SessionStart":[{"hooks":[{"command":` + wrapper + `session-start"}]}],"SubagentStart":[{"hooks":[{"command":` + wrapper + `subagent-start"}]}],"PreToolUse":[{"matcher":"Bash","hooks":[{"command":` + wrapper + `deny-destructive"}]},{"matcher":"mcp__.*","hooks":[{"command":` + wrapper + `deny-destructive"}]}]}}`, true},
		{"one unwrapped command", `{"hooks":{"SessionStart":[{"hooks":[{"command":` + wrapper + `session-start"}]}],"PreToolUse":[{"hooks":[{"command":"bash ./x.sh"}]}]}}`, false},
		{"no commands", `{"hooks":{}}`, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "hooks.json")
			if err := os.WriteFile(path, []byte(test.manifest), 0o600); err != nil {
				t.Fatal(err)
			}
			if got := hookCommandsUseWrapper(path); got != test.want {
				t.Fatalf("hookCommandsUseWrapper = %v, want %v", got, test.want)
			}
		})
	}
}
