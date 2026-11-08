package commandtrace

import (
	"reflect"
	"strings"
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

func TestObservedGoTestPipelineOutput(t *testing.T) {
	for _, tc := range []struct {
		name, command, output string
		wantRC                int
	}{
		{"tail", `go test ./... 2>&1 | tail -20`, "ok  \texample/rate\t0.003s\n", 0},
		{"grep and masked echo", `go test -v ./... | grep -E '^(---|ok|FAIL)'; echo exit=$?`, "--- PASS: TestAllow (0.00s)\nok  \texample/rate\t(cached)\nexit=0\n", 0},
		{"prerequisite", `gofmt -l .; go vet ./... && go test -v ./... 2>&1 | tail -10`, "rate.go\nPASS\nok  \texample/rate\t0.003s\n", 0},
		{"multiple packages", `go test ./... | tail -20`, "ok  \texample/a\t0.003s\nok  \texample/b\t(cached)\n", 0},
		{"standalone PASS", `go test -v ./... | grep '^PASS$'`, "PASS\n", 0},
		{"no output", `go test ./... | tail -20`, "", 125},
		{"masked status only", `go test ./... | grep ok; echo exit=$?`, "exit=0\n", 125},
		{"individual test only", `go test -v ./... | head -10`, "=== RUN   TestAllow\n--- PASS: TestAllow (0.00s)\n", 125},
		{"malformed ok", `go test ./... | tail -20`, "ok\n", 125},
		{"failed package", `go test ./... | tail -20`, "ok  \texample/a\t0.003s\nFAIL\texample/b\t0.002s\nFAIL\n", 125},
		{"failed test", `go test -v ./... | tail -20`, "--- FAIL: TestAllow (0.00s)\nPASS\nok  \texample/rate\t0.003s\n", 125},
		{"no test package", `go test ./... | tail -20`, "?   \texample/cmd\t[no test files]\nok  \texample/rate\t0.003s\n", 125},
		{"conditional fallback", `true || go test ./... | tail -20`, "PASS\n", 125},
		{"terminated", `exit 0; go test ./... | tail -20`, "PASS\n", 125},
		{"background", `go test ./... | tail -20 &`, "PASS\n", 125},
		{"substitution", `echo $(go version); go test ./... | tail -20`, "PASS\n", 125},
		{"not Go", `npm test | tail -20`, "PASS\n", 125},
		{"sequential output", `go test ./...; go vet ./...`, "PASS\n", 0},
		{"sequential failure", `go test ./...; cat health.go`, "FAIL\n", 125},
		{"sequential empty", `go test ./...; cat health.go`, "", 125},
		{"sequential no tests", `go test ./...; cat health.go`, "?   \texample/hard\t[no test files]\n", 125},
		{"sequential fallback", `go test ./... || true; cat health.go`, "PASS\n", 125},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := Observed(tc.command, 0, tc.output)
			if len(got) != 1 || got[0].Kind != "test" || got[0].RC != tc.wantRC {
				t.Fatalf("Observed() = %+v, want test rc=%d", got, tc.wantRC)
			}
			if got := Classify(tc.command, 0); len(got) != 0 {
				t.Fatalf("output inference changed broker classification: %+v", got)
			}
		})
	}
	for _, rc := range []int{1, 124, 125, 143} {
		got := Observed(`go test -v ./... | grep -E '^(ok|FAIL)'`, rc, "ok  \texample/rate\t0.003s\n")
		if len(got) != 1 || got[0].Kind != "test" || got[0].RC != 125 {
			t.Fatalf("partial or failed pipeline rc=%d inferred success: %+v", rc, got)
		}
	}
	// One invocation's success cannot attest a second, possibly skipped test.
	events := Observed(`go test ./... | tail -5; false && go test ./... | tail -5`, 0, "ok  \texample/rate\t0.003s\n")
	if len(events) != 2 {
		t.Fatalf("missing test attempts: %+v", events)
	}
	for _, event := range events {
		if event.Kind != "test" || event.RC != 125 {
			t.Fatalf("ambiguous output attributed across invocations: %+v", event)
		}
	}
}

func TestObservedGoTestMutationSequence(t *testing.T) {
	const command = `cp store.go /tmp/f; sed -i 's/fixed/broken/' store.go; go test -count=1 ./... 2>&1 | grep -E -- '--- FAIL|ok'; cp /tmp/f store.go; go test -count=1 ./...; go vet ./...`
	const green = "PASS\nok  \texample/hard\t0.003s\n"
	const red = "--- FAIL: TestOrder (0.00s)\n--- FAIL: TestAlias (0.00s)\nFAIL\nFAIL\texample/hard\t0.003s\nFAIL\n"
	for _, tc := range []struct {
		name, command, output string
		want                  []int
	}{
		{"red then green", command, red + green, []int{1, 0}},
		{"CRLF red then green", command, strings.ReplaceAll(red+green, "\n", "\r\n"), []int{1, 0}},
		{"filtered red block", command, "--- FAIL: TestOrder (0.00s)\n--- FAIL: TestAlias (0.00s)\n" + green, []int{1, 0}},
		{"bare FAIL", command, "FAIL\n" + green, []int{1, 0}},
		{"green then red", command, green + red, []int{0, 1}},
		{"green red green", `go test -v ./... | tail -12; git stash -q; go test ./... | grep -E 'FAIL|ok'; git stash pop -q; go test ./...; go vet ./...`, green + red + green, []int{0, 1, 0}},
		{"both green", command, green + green, []int{0, 0}},
		{"missing summary", command, green, []int{125, 125}},
		{"extra summary", command, green + red + green, []int{125, 125}},
		{"multiple packages", command, green + "ok  \texample/other\t0.003s\n", []int{125, 125}},
		{"no test package", command, green + "?   \texample/hard\t[no test files]\n", []int{125, 125}},
		{"PASS alone ambiguous", command, "PASS\nPASS\n", []int{125, 125}},
		{"two bare failures ambiguous", command, "FAIL\nFAIL\n", []int{125, 125}},
		{"mixed test runners", `go test ./... | tail -5; npm test; go vet ./...`, green + green, []int{125, 125}},
		{"conditional fallback", `true || go test ./... | tail -5; go test ./... | tail -5; go vet ./...`, green + green, []int{125, 125}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			events := Observed(tc.command, 0, tc.output)
			var got []int
			for _, event := range events {
				if event.Kind == "test" {
					got = append(got, event.RC)
				}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("test statuses = %v, want %v", got, tc.want)
			}
			if got := Classify(tc.command, 0); len(got) != 0 {
				t.Fatalf("broker attribution changed: %+v", got)
			}
		})
	}
	// A plain successful && chain already proves both test statuses.
	for _, event := range Observed(`go test ./... && go test ./...`, 0, red+green) {
		if event.Kind != "test" || event.RC != 0 {
			t.Fatalf("output overrode attributable shell success: %+v", event)
		}
	}
}
