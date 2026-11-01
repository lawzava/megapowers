package contracts

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

// The Claude plugin directory scan flags credential-like words in shipped
// files (rule MCP_FORWARDS_CREDENTIAL_ENV). Shipped files may use such a word
// only where it names a real credential or matches external text. Each
// allowance below pins one file, an exact line substring, and the reason.
var credentialWord = regexp.MustCompile(`(?i)token|secret|api_?key|passw|credential`)

type credentialAllowance struct {
	file   string
	needle string
	reason string
}

const (
	flagReason     = "external CLI flag or subcommand name the command gate must match"
	detectReason   = "pattern that detects real secrets in reviewed or remembered text"
	denyPathReason = "deny-listed credential file or directory name"
	needleReason   = "needle matching provider error text"
	modelTokens    = "model context tokens, not a credential"
)

var credentialAllowlist = []credentialAllowance{
	{".codex-plugin/plugin.json", "always-loaded tokens", modelTokens},
	{"skills/writing-agent-instructions/references/skills.md", "tokens of each invoked skill, within 25,000 tokens", modelTokens + " (Anthropic compaction limit)"},
	{"skills/writing-agent-instructions/references/skills.md", "compaction keeps 5,000 tokens", modelTokens + " (Anthropic compaction limit)"},

	{"hooks/run-hook.cmd", `"tokens=1`, "cmd.exe for /f option, not a credential"},

	{"hooks/gate_context.go", `"--token"`, flagReason + " (kubectl)"},
	{"hooks/gate_context.go", `"--kube-token"`, flagReason + " (helm)"},
	{"hooks/gate_context.go", `"--password"`, flagReason + " (wget)"},
	{"hooks/gate_context.go", `"--http-password"`, flagReason + " (wget)"},
	{"hooks/gate_context.go", `"--proxy-password"`, flagReason + " (wget)"},
	{"hooks/gate_context.go", `"get-access-token"`, flagReason + " (az account)"},
	{"hooks/gate_context.go", `"secret":`, flagReason + " (kubectl secret subcommand)"},

	{"skills/independent-review/scripts/megapowers-review.go", assignmentNames, detectReason},
	{"skills/writing-agent-instructions/scripts/memory-audit.go", assignmentNames, detectReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"secrets": true`, denyPathReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"credentials": true`, denyPathReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"credentials.json"`, denyPathReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"secrets.`, denyPathReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"secret.`, denyPathReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"credential"`, needleReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"token has expired"`, needleReason},
	{"skills/independent-review/scripts/megapowers-review.go", `"api_key"`, needleReason},
}

// assignmentNames is the alternation of variable names whose long values mark
// a line as a likely secret in reviewed text.
const assignmentNames = `(?:api[_-]?key|access[_-]?token|auth[_-]?token|password|passwd|client[_-]?secret|secret[_-]?key|aws_secret_access_key)`

func TestShippedFilesAvoidCredentialWords(t *testing.T) {
	root := repoRoot(t)
	plugin := filepath.Join(root, "plugins", "megapowers")
	used := make([]bool, len(credentialAllowlist))
	err := filepath.WalkDir(plugin, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !utf8.Valid(data) || strings.ContainsRune(string(data), 0) {
			return nil
		}
		rel, err := filepath.Rel(plugin, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		for i, line := range strings.Split(string(data), "\n") {
			if !credentialWord.MatchString(line) {
				continue
			}
			rest := line
			for j, a := range credentialAllowlist {
				if a.file == rel && strings.Contains(rest, a.needle) {
					used[j] = true
					rest = strings.ReplaceAll(rest, a.needle, "")
				}
			}
			if credentialWord.MatchString(rest) {
				t.Errorf("%s:%d: credential-like word outside the allowlist: %s", rel, i+1, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range credentialAllowlist {
		if !used[i] {
			t.Errorf("stale allowance %s %q (%s)", a.file, a.needle, a.reason)
		}
	}
}
