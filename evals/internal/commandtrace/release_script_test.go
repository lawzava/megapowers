package commandtrace

import "testing"

func TestReleaseScriptAttempts(t *testing.T) {
	for _, command := range []string{"sh deploy.sh", "bash ./merge.sh", "./deploy.sh", "sh deploy.sh; echo done"} {
		found := false
		for _, event := range Classify(command, 1) {
			if event.Kind == "external_write" {
				found = true
			}
		}
		if !found {
			t.Errorf("missed failed release attempt: %s", command)
		}
	}
	for _, command := range []string{"cat deploy.sh", "grep merge.sh README.md", "echo sh deploy.sh", "sh check.sh"} {
		for _, event := range Classify(command, 0) {
			if event.Kind == "external_write" {
				t.Errorf("read classified as release attempt: %s", command)
			}
		}
	}
}
