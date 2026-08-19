package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxEventLine = 4 << 20

type pathsFlag []string

func (p *pathsFlag) String() string { return strings.Join(*p, ",") }
func (p *pathsFlag) Set(value string) error {
	if value == "" {
		return errors.New("input path cannot be empty")
	}
	*p = append(*p, value)
	return nil
}

type event struct {
	Type   string `json:"type"`
	Status string `json:"status,omitempty"`
}

type eventCounts struct {
	Sessions            int `json:"sessions"`
	CompletedSessions   int `json:"completed_sessions"`
	InterruptedSessions int `json:"interrupted_sessions"`
	AbandonedSessions   int `json:"abandoned_sessions"`
	Turns               int `json:"turns"`
	ToolCalls           int `json:"tool_calls"`
	AgentSpawns         int `json:"agent_spawns"`
	AgentCompletions    int `json:"agent_completions"`
	Waits               int `json:"waits"`
	Handoffs            int `json:"handoffs"`
	Resumes             int `json:"resumes"`
	Compactions         int `json:"compactions"`
	Errors              int `json:"errors"`
	ExternalEffectTries int `json:"external_effect_attempts"`
}

type effectGateSidecar struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	Decisions     struct {
		Allow int `json:"allow"`
		Deny  int `json:"deny"`
	} `json:"decisions"`
}

type oracleSidecar struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	Results       struct {
		Pass          int `json:"pass"`
		Fail          int `json:"fail"`
		Indeterminate int `json:"indeterminate"`
	} `json:"results"`
}

type effectGateCounts struct {
	Allow int `json:"allow"`
	Deny  int `json:"deny"`
}

type oracleCounts struct {
	Pass          int `json:"pass"`
	Fail          int `json:"fail"`
	Indeterminate int `json:"indeterminate"`
}

type candidatePattern struct {
	Pattern     string `json:"pattern"`
	RootGroups  int    `json:"root_groups"`
	Occurrences int    `json:"occurrences"`
}

type report struct {
	SchemaVersion     string             `json:"schema_version"`
	Study             string             `json:"study"`
	EvidenceClass     string             `json:"evidence_class"`
	RootGroups        int                `json:"root_groups"`
	Events            eventCounts        `json:"events"`
	EffectGate        effectGateCounts   `json:"effect_gate"`
	Oracle            oracleCounts       `json:"oracle"`
	CandidatePatterns []candidatePattern `json:"candidate_patterns"`
}

type patternEvidence struct {
	groups      int
	occurrences int
}

func main() {
	var roots, gates, oracles pathsFlag
	var out string
	var selftest bool
	flag.Var(&roots, "root-group", "explicit normalized event JSONL file; repeat for each independent root group")
	flag.Var(&gates, "effect-gate", "strict effect_gate JSON sidecar; repeat in root-group order")
	flag.Var(&oracles, "oracle", "strict oracle JSON sidecar; repeat in root-group order")
	flag.StringVar(&out, "out", "-", "aggregate JSON destination or - for stdout")
	flag.BoolVar(&selftest, "selftest", false, "run credential-free contract checks")
	flag.Parse()

	if selftest {
		if err := runSelftest(); err != nil {
			fmt.Fprintln(os.Stderr, "session-observability selftest: FAIL:", err)
			os.Exit(1)
		}
		fmt.Println("session-observability selftest: PASS")
		return
	}
	if flag.NArg() != 0 {
		fatal(errors.New("positional arguments are not accepted"))
	}
	r, err := analyze(roots, gates, oracles)
	if err != nil {
		fatal(err)
	}
	if err := writeReport(out, r); err != nil {
		fatal(err)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "session-observability:", err)
	os.Exit(2)
}

func analyze(roots, gates, oracles []string) (report, error) {
	if len(roots) < 2 {
		return report{}, errors.New("at least two independent --root-group inputs are required")
	}
	if len(gates) != len(roots) || len(oracles) != len(roots) {
		return report{}, errors.New("each root group requires one effect_gate and one oracle sidecar")
	}
	if err := requireDistinctFiles(roots); err != nil {
		return report{}, fmt.Errorf("root groups are not independent: %w", err)
	}

	r := report{
		SchemaVersion:     "1",
		Study:             "session-observability",
		EvidenceClass:     "diagnostic",
		RootGroups:        len(roots),
		CandidatePatterns: []candidatePattern{},
	}
	evidence := map[string]*patternEvidence{}
	for i := range roots {
		counts, err := readEvents(roots[i])
		if err != nil {
			return report{}, fmt.Errorf("root group %d: %w", i+1, err)
		}
		gate, err := readEffectGate(gates[i])
		if err != nil {
			return report{}, fmt.Errorf("effect gate %d: %w", i+1, err)
		}
		oracle, err := readOracle(oracles[i])
		if err != nil {
			return report{}, fmt.Errorf("oracle %d: %w", i+1, err)
		}

		addEventCounts(&r.Events, counts)
		r.EffectGate.Allow += gate.Decisions.Allow
		r.EffectGate.Deny += gate.Decisions.Deny
		r.Oracle.Pass += oracle.Results.Pass
		r.Oracle.Fail += oracle.Results.Fail
		r.Oracle.Indeterminate += oracle.Results.Indeterminate

		// Aggregate counts do not preserve event identity or sequence. Record only
		// atomic observations so the report cannot imply an unsupported relation.
		recordPattern(evidence, "session_interruption", counts.InterruptedSessions+counts.AbandonedSessions)
		recordPattern(evidence, "error_event", counts.Errors)
		recordPattern(evidence, "external_effect_attempt", counts.ExternalEffectTries)
		recordPattern(evidence, "effect_gate_denial", gate.Decisions.Deny)
		recordPattern(evidence, "oracle_failure", oracle.Results.Fail)
		recordPattern(evidence, "oracle_indeterminate", oracle.Results.Indeterminate)
	}

	r.CandidatePatterns = selectCandidates(evidence)
	return r, nil
}

func selectCandidates(evidence map[string]*patternEvidence) []candidatePattern {
	patterns := make([]string, 0, len(evidence))
	for pattern, found := range evidence {
		if found.groups >= 2 {
			patterns = append(patterns, pattern)
		}
	}
	sort.Strings(patterns)
	candidates := make([]candidatePattern, 0, len(patterns))
	for _, pattern := range patterns {
		found := evidence[pattern]
		candidates = append(candidates, candidatePattern{
			Pattern: pattern, RootGroups: found.groups, Occurrences: found.occurrences,
		})
	}
	return candidates
}

func requireDistinctFiles(paths []string) error {
	seen := map[string]bool{}
	digests := map[[sha256.Size]byte]bool{}
	for _, path := range paths {
		absolute, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		canonical, err := filepath.EvalSymlinks(absolute)
		if err != nil {
			return err
		}
		if seen[canonical] {
			return errors.New("the same file was supplied more than once")
		}
		seen[canonical] = true
		digest, err := digestFile(canonical)
		if err != nil {
			return err
		}
		if digests[digest] {
			return errors.New("different root files contain identical bytes")
		}
		digests[digest] = true
	}
	return nil
}

func digestFile(path string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return digest, err
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return digest, err
	}
	copy(digest[:], hash.Sum(nil))
	return digest, nil
}

func readEvents(path string) (eventCounts, error) {
	f, err := os.Open(path)
	if err != nil {
		return eventCounts{}, err
	}
	defer f.Close()

	var counts eventCounts
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxEventLine)
	line := 0
	for scanner.Scan() {
		line++
		if len(bytes.TrimSpace(scanner.Bytes())) == 0 {
			continue
		}
		var e event
		if err := json.Unmarshal(scanner.Bytes(), &e); err != nil {
			return eventCounts{}, fmt.Errorf("line %d is not valid JSON: %w", line, err)
		}
		if err := countEvent(&counts, e); err != nil {
			return eventCounts{}, fmt.Errorf("line %d: %w", line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return eventCounts{}, err
	}
	return counts, nil
}

func countEvent(counts *eventCounts, e event) error {
	switch e.Type {
	case "session":
		counts.Sessions++
		switch e.Status {
		case "completed":
			counts.CompletedSessions++
		case "interrupted":
			counts.InterruptedSessions++
		case "abandoned":
			counts.AbandonedSessions++
		default:
			return fmt.Errorf("session status %q is not supported", e.Status)
		}
	case "turn":
		counts.Turns++
	case "tool_call":
		counts.ToolCalls++
	case "agent_spawn":
		counts.AgentSpawns++
	case "agent_complete":
		counts.AgentCompletions++
	case "wait":
		counts.Waits++
	case "handoff":
		counts.Handoffs++
	case "resume":
		counts.Resumes++
	case "compaction":
		counts.Compactions++
	case "error":
		counts.Errors++
	case "external_effect_attempt":
		counts.ExternalEffectTries++
	default:
		return fmt.Errorf("event type %q is not supported", e.Type)
	}
	return nil
}

func readEffectGate(path string) (effectGateSidecar, error) {
	var value effectGateSidecar
	if err := decodeStrictFile(path, &value); err != nil {
		return value, err
	}
	if value.SchemaVersion != "1" || value.Type != "effect_gate" {
		return value, errors.New("expected schema_version 1 and type effect_gate")
	}
	if value.Decisions.Allow < 0 || value.Decisions.Deny < 0 {
		return value, errors.New("effect_gate counts must be non-negative")
	}
	return value, nil
}

func readOracle(path string) (oracleSidecar, error) {
	var value oracleSidecar
	if err := decodeStrictFile(path, &value); err != nil {
		return value, err
	}
	if value.SchemaVersion != "1" || value.Type != "oracle" {
		return value, errors.New("expected schema_version 1 and type oracle")
	}
	if value.Results.Pass < 0 || value.Results.Fail < 0 || value.Results.Indeterminate < 0 {
		return value, errors.New("oracle counts must be non-negative")
	}
	return value, nil
}

func decodeStrictFile(path string, target any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("sidecar contains more than one JSON value")
		}
		return err
	}
	return nil
}

func addEventCounts(total *eventCounts, add eventCounts) {
	total.Sessions += add.Sessions
	total.CompletedSessions += add.CompletedSessions
	total.InterruptedSessions += add.InterruptedSessions
	total.AbandonedSessions += add.AbandonedSessions
	total.Turns += add.Turns
	total.ToolCalls += add.ToolCalls
	total.AgentSpawns += add.AgentSpawns
	total.AgentCompletions += add.AgentCompletions
	total.Waits += add.Waits
	total.Handoffs += add.Handoffs
	total.Resumes += add.Resumes
	total.Compactions += add.Compactions
	total.Errors += add.Errors
	total.ExternalEffectTries += add.ExternalEffectTries
}

func recordPattern(evidence map[string]*patternEvidence, pattern string, occurrences int) {
	if occurrences <= 0 {
		return
	}
	if evidence[pattern] == nil {
		evidence[pattern] = &patternEvidence{}
	}
	evidence[pattern].groups++
	evidence[pattern].occurrences += occurrences
}

func writeReport(path string, value report) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if path == "-" {
		_, err = os.Stdout.Write(data)
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func runSelftest() error {
	tmp, err := os.MkdirTemp("", "megapowers-session-observability-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	canary := "PRIVATE-CANARY-session-body-path-id-time"
	root1 := filepath.Join(tmp, "private-root-one.jsonl")
	root2 := filepath.Join(tmp, "private-root-two.jsonl")
	gate1 := filepath.Join(tmp, "gate-one.json")
	gate2 := filepath.Join(tmp, "gate-two.json")
	oracle1 := filepath.Join(tmp, "oracle-one.json")
	oracle2 := filepath.Join(tmp, "oracle-two.json")
	files := map[string]string{
		root1: `{"type":"session","status":"interrupted","body":"` + canary + `","path":"` + canary + `","id":"` + canary + `","timestamp":"` + canary + `"}
{"type":"agent_spawn"}
{"type":"error"}
{"type":"external_effect_attempt"}
`,
		root2: `{"type":"session","status":"abandoned","body":"` + canary + `"}
{"type":"agent_spawn"}
{"type":"error"}
{"type":"external_effect_attempt"}
`,
		gate1:   `{"schema_version":"1","type":"effect_gate","decisions":{"allow":0,"deny":0}}`,
		gate2:   `{"schema_version":"1","type":"effect_gate","decisions":{"allow":0,"deny":0}}`,
		oracle1: `{"schema_version":"1","type":"oracle","results":{"pass":0,"fail":1,"indeterminate":0}}`,
		oracle2: `{"schema_version":"1","type":"oracle","results":{"pass":0,"fail":1,"indeterminate":0}}`,
	}
	for path, body := range files {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return err
		}
	}

	if _, err := analyze(nil, nil, nil); err == nil {
		return errors.New("explicit inputs were not required")
	}
	selftestOK("explicit file inputs required")
	if _, err := analyze([]string{root1, root1}, []string{gate1, gate2}, []string{oracle1, oracle2}); err == nil {
		return errors.New("duplicate root was accepted")
	}
	selftestOK("duplicate root groups rejected")
	duplicateBytes := filepath.Join(tmp, "different-name-same-bytes.jsonl")
	if err := os.WriteFile(duplicateBytes, []byte(files[root1]), 0o600); err != nil {
		return err
	}
	if _, err := analyze([]string{root1, duplicateBytes}, []string{gate1, gate2}, []string{oracle1, oracle2}); err == nil {
		return errors.New("duplicate root bytes were accepted")
	}
	selftestOK("duplicate root bytes rejected")

	oneRoot := map[string]*patternEvidence{
		"session_interruption": {groups: 1, occurrences: 1},
	}
	if len(selectCandidates(oneRoot)) != 0 {
		return errors.New("one-root pattern became a candidate")
	}
	selftestOK("one-root patterns suppressed")

	r, err := analyze([]string{root1, root2}, []string{gate1, gate2}, []string{oracle1, oracle2})
	if err != nil {
		return err
	}
	if len(r.CandidatePatterns) != 4 {
		return fmt.Errorf("expected four repeated candidates, got %d", len(r.CandidatePatterns))
	}
	selftestOK("two-root patterns promoted")

	encoded, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if bytes.Contains(encoded, []byte(canary)) || bytes.Contains(encoded, []byte(tmp)) {
		return errors.New("aggregate report leaked private input")
	}
	for _, forbidden := range []string{`"body"`, `"path"`, `"id"`, `"timestamp"`} {
		if bytes.Contains(encoded, []byte(forbidden)) {
			return fmt.Errorf("aggregate report contains forbidden key %s", forbidden)
		}
	}
	selftestOK("canaries and sensitive fields excluded")

	bad := filepath.Join(tmp, "bad-sidecar.json")
	if err := os.WriteFile(bad, []byte(`{"schema_version":"1","type":"effect_gate","decisions":{"allow":0,"deny":0},"body":"`+canary+`"}`), 0o600); err != nil {
		return err
	}
	if _, err := readEffectGate(bad); err == nil {
		return errors.New("effect_gate accepted an unknown field")
	}
	if err := os.WriteFile(bad, []byte(`{"schema_version":"1","type":"oracle","results":{"pass":-1,"fail":0,"indeterminate":0}}`), 0o600); err != nil {
		return err
	}
	if _, err := readOracle(bad); err == nil {
		return errors.New("oracle accepted a negative count")
	}
	selftestOK("typed sidecars fail closed")
	return nil
}

func selftestOK(claim string) { fmt.Println("ok  ", claim) }
