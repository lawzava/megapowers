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

func TestObservedTestAttempts(t *testing.T) {
	for _, tc := range []struct {
		command, output string
		rc, wantRC      int
	}{
		{`go test ./...`, "", 7, 7},
		{`cd pkg && go test ./...`, "", 0, 0},
		{`cd missing && go test ./...`, "", 1, 125},
		{`go test ./... && go vet ./...`, "", 0, 0},
		{`go test ./... && go vet ./...`, "", 1, 125},
		{`cat go.mod; go test ./...; echo "exit=$?"; go vet ./...; echo "vet exit=$?"`, "ok\nexit=0\nvet exit=0\n", 0, 0},
		{`go test ./...; echo EXIT=$?`, "FAIL\nEXIT=1\n", 0, 1},
		{`go test ./...; echo EXIT=$?`, "EXIT=0\nEXIT=1\n", 0, 125},
		{`gofmt -l .; go test ./... 2>&1`, "ok\n", 0, 0},
		{`gofmt -l .; go test ./... 2>&1`, "FAIL\n", 1, 1},
		{`cat go.mod; go vet ./... && go test ./...`, "ok\n", 0, 0},
		{`cat go.mod; false && go test ./...`, "", 1, 125},
		{`go test ./...; echo "exit=$?"; go run ./cmd/greet --name Ada; echo "exit=$?"`, "ok\nexit=0\nHello, world!\nexit=0\n", 0, 0},
		{`go test ./...; echo "exit=$?"; go run ./cmd/greet --name Ada; echo "exit=$?"`, "Hello, world!\nexit=0\n", 0, 125},
		{`go test ./...; echo EXIT=$?`, "EXIT=1\nEXIT=1\n", 0, 1},
		{`go test ./...; echo EXIT=$?`, "", 0, 125},
		{`go test ./... || true`, "", 0, 125},
		{`go test ./... | tail -5`, "", 0, 125},
		{`false || go test ./...`, "", 0, 125},
		{`true || go test ./...; echo rc=$?`, "rc=0\n", 0, 125},
		{`false && go test ./...; echo rc=$?`, "rc=1\n", 0, 125},
		{`make build && go test ./...; echo rc=$?`, "rc=1\n", 0, 125},
		{`cd missing && go test ./...; echo rc=$?`, "rc=1\n", 0, 125},
		{`cd pkg && go test ./...; echo rc=$?`, "rc=0\n", 0, 0},
		{`go test ./... && go vet ./... &`, "", 0, 125},
		{`go test ./... &`, "", 0, 125},
		{`sleep 1 & go test ./...`, "ok\n", 0, 125},
		{`exit 0; go test ./...`, "", 0, 125},
		{`true && exit 0 && go test ./...`, "", 0, 125},
		{`exec true && go test ./...`, "", 0, 125},
		{`return 0; go test ./...; echo rc=$?`, "rc=0\n", 0, 125},
		{`echo 'go test ./...'; go test ./...; echo rc=$?`, "rc=0\n", 0, 0},
		{`go test ./...; echo 'rc=$?'`, "rc=0\n", 0, 125},
	} {
		t.Run(tc.command+tc.output, func(t *testing.T) {
			got := Observed(tc.command, tc.rc, tc.output)
			if len(got) != 1 || got[0].Kind != "test" || got[0].RC != tc.wantRC {
				t.Fatalf("Observed() = %+v, want test rc=%d", got, tc.wantRC)
			}
		})
	}
	if got := Observed(`echo 'go test ./...'`, 0, ""); len(got) != 0 {
		t.Fatalf("quoted text classified: %+v", got)
	}
}

func TestObservedSequentialAttributionDoesNotChangeBrokerEvents(t *testing.T) {
	const command = "gofmt -l .; go test ./..."
	events := Events(command, 0)
	if len(events) != 1 || events[0].Kind != UnattributedTestKind {
		t.Fatalf("broker attribution changed: %+v", events)
	}
	if events := Classify(command, 0); len(events) != 0 {
		t.Fatalf("broker accepted untrusted diagnostic: %+v", events)
	}
}
