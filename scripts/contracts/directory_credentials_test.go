package contracts

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Keep local harness homes and credential-file references out of every shipped
// file, including passive assets and generated catalogs.
func TestShippedFilesExcludeCredentialLocations(t *testing.T) {
	root := repoRoot(t)
	forbidden := regexp.MustCompile(`(?i)~/\.claude|~/\.codex|CODEX_HOME|\.credentials|auth\.json`)
	err := filepath.WalkDir(filepath.Join(root, "plugins/megapowers"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(read(t, root, rel), "\n") {
			if forbidden.MatchString(line) {
				t.Errorf("%s:%d: credential location: %s", rel, i+1, line)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInstructionExtrasStayOutsidePlugin(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range []string{
		"references/session-mining.md",
		"references/learning-loop.md",
		"references/global-instructions.md",
		"assets/global-AGENTS.md",
		"assets/global-CLAUDE.md",
	} {
		path := "skills/writing-agent-instructions/" + rel
		read(t, root, "extras/"+path)
		if _, err := os.Stat(filepath.Join(root, "plugins/megapowers", path)); !os.IsNotExist(err) {
			t.Errorf("%s must remain repository-only; stat: %v", path, err)
		}
		requireContains(t, read(t, root, "extras/README.md"), "("+path+")", "extras listing")
	}
}
