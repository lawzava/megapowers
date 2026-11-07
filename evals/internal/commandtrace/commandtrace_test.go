package commandtrace

import (
	"reflect"
	"testing"
)

func TestCommandEvents(t *testing.T) {
	for _, test := range []struct {
		command    string
		kind       string
		classified bool
	}{
		{"cat cmd/tracker-comment", "", false},
		{"go run ./cmd/tracker-comment", "tracker_comment", true},
		{"git -C d push", "external_write", true},
		{"echo 'git push'", "", false},
		{"curl -X GET https://example.invalid", "", false},
		{"go test ./... 2>&1", "test", true},
		{"go test ./... || true", "test_unattributed", false},
	} {
		t.Run(test.command, func(t *testing.T) {
			var want []Event
			if test.kind != "" {
				want = []Event{{Kind: test.kind, RC: 7}}
			}
			if got := Events(test.command, 7); !sameEvents(got, want) {
				t.Errorf("Events() = %+v, want %+v", got, want)
			}
			if !test.classified {
				want = nil
			}
			if got := Classify(test.command, 7); !sameEvents(got, want) {
				t.Errorf("Classify() = %+v, want %+v", got, want)
			}
		})
	}
}

func sameEvents(got, want []Event) bool {
	return len(got) == 0 && len(want) == 0 || reflect.DeepEqual(got, want)
}

func TestNormalizedCommandSegments(t *testing.T) {
	want := [][]string{{"cd", "pkg"}, {"cat", "skill.md"}}
	if got := NormalizedCommandSegments("/bin/bash -lc 'cd pkg && cat SKILL.md'"); !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizedCommandSegments() = %q, want %q", got, want)
	}
}
