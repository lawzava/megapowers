// Command fact-judge re-grades the fact checks of an installed-ab run with a
// model judge. Literal phrase matching fails correct paraphrases ("credentials
// aren't available" for "credentials are unavailable"), so the judge decides
// per fact whether each response states it or asserts a forbidden claim.
//
// Each case's responses go to the judge in one blind pass: shuffled with a
// fixed seed, labeled R1..Rn, with no arm, run, or case identity. The judge
// command runs without a shell; {prompt_file} expands to the prompt path.
// Output stays private under --out: judged.jsonl plus a summary on stdout.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

type studyCase struct {
	ID                string   `json:"id"`
	Kind              string   `json:"kind"`
	OrchestrationMode string   `json:"orchestration_mode"`
	Task              string   `json:"task"`
	RequiredFacts     []string `json:"required_facts"`
	ForbiddenFacts    []string `json:"forbidden_facts"`
}

type row struct {
	CaseID  string `json:"case_id"`
	RunID   string `json:"run_id"`
	Arm     string `json:"arm"`
	Status  string `json:"status"`
	Harness struct {
		Model string `json:"model"`
	} `json:"harness"`
	Metrics map[string]float64 `json:"metrics"`
}

type item struct {
	Label    string
	Response string
	Row      row
}

type verdict struct {
	Required  []bool
	Forbidden []bool
}

func (v verdict) pass() bool {
	for _, ok := range v.Required {
		if !ok {
			return false
		}
	}
	for _, asserted := range v.Forbidden {
		if asserted {
			return false
		}
	}
	return true
}

type judgedRow struct {
	CaseID         string `json:"case_id"`
	RunID          string `json:"run_id"`
	Arm            string `json:"arm"`
	Model          string `json:"model"`
	LiteralOutcome bool   `json:"literal_outcome"`
	JudgedFacts    bool   `json:"judged_facts"`
	JudgedOutcome  bool   `json:"judged_outcome"`
	Required       []bool `json:"required"`
	Forbidden      []bool `json:"forbidden"`
}

const judgeTimeout = 15 * time.Minute

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "fact-judge: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("fact-judge", flag.ContinueOnError)
	casesPath := flags.String("cases", "", "case catalog")
	runDir := flags.String("run-dir", "", "installed-ab output directory")
	judge := flags.String("judge", "", "judge command; {prompt_file} expands to the prompt path")
	out := flags.String("out", "", "private output directory")
	seed := flags.Int64("seed", 1, "shuffle seed")
	only := flags.String("case", "", "optional comma-separated case ids")
	reuse := flags.Bool("reuse", false, "reuse saved verdicts whose prompts are byte-identical")
	check := flags.Bool("check", false, "only verify the outcome rule against rows whose literal facts pass")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *check {
		cases, err := loadCases(*casesPath)
		if err != nil {
			return err
		}
		rows, err := loadRows(filepath.Join(*runDir, "publish", "results.jsonl"))
		if err != nil {
			return err
		}
		checked, mismatches := checkConsistency(cases, rows)
		fmt.Printf("checked %d rows, %d mismatches %v\n", checked, len(mismatches), mismatches)
		if len(mismatches) > 0 {
			return errors.New("outcome rule disagrees with the runner")
		}
		return nil
	}
	if *casesPath == "" || *runDir == "" || *judge == "" || *out == "" {
		return errors.New("--cases, --run-dir, --judge, and --out are required")
	}
	if !strings.Contains(*judge, "{prompt_file}") {
		return errors.New("--judge must name {prompt_file}")
	}
	cases, err := loadCases(*casesPath)
	if err != nil {
		return err
	}
	rows, err := loadRows(filepath.Join(*runDir, "publish", "results.jsonl"))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*out, 0o700); err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, id := range strings.Split(*only, ",") {
		if id = strings.TrimSpace(id); id != "" {
			selected[id] = true
		}
	}
	byCase := map[string][]row{}
	for _, r := range rows {
		if r.Status == "completed" && (len(selected) == 0 || selected[r.CaseID]) {
			byCase[r.CaseID] = append(byCase[r.CaseID], r)
		}
	}
	ids := make([]string, 0, len(byCase))
	for id := range byCase {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	results, err := os.OpenFile(filepath.Join(*out, "judged.jsonl"), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer results.Close()
	encoder := json.NewEncoder(results)
	for _, id := range ids {
		c, ok := cases[id]
		if !ok || len(c.RequiredFacts)+len(c.ForbiddenFacts) == 0 {
			continue
		}
		responses := map[string]string{}
		for _, r := range byCase[id] {
			text, err := readResponse(*runDir, r.RunID)
			if err != nil {
				return err
			}
			responses[r.RunID] = text
		}
		items := labelItems(byCase[id], responses, *seed)
		verdicts, err := judgeCase(*judge, *out, c, items, *reuse)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		literal, judged := map[string][2]int{}, map[string][2]int{}
		for _, it := range items {
			v := verdicts[it.Label]
			applyLiteralMarkup(c, it.Response, &v)
			row := judgedRow{CaseID: id, RunID: it.Row.RunID, Arm: it.Row.Arm, Model: it.Row.Harness.Model,
				LiteralOutcome: it.Row.Metrics["outcome_success"] == 1, JudgedFacts: v.pass(),
				JudgedOutcome: judgedOutcome(it.Row, c, v), Required: v.Required, Forbidden: v.Forbidden}
			if err := encoder.Encode(row); err != nil {
				return err
			}
			tally(literal, it.Row.Arm, row.LiteralOutcome)
			tally(judged, it.Row.Arm, row.JudgedOutcome)
		}
		fmt.Printf("%s\tliteral treatment %d/%d control %d/%d\tjudged treatment %d/%d control %d/%d\n", id,
			literal["treatment"][0], literal["treatment"][1], literal["control"][0], literal["control"][1],
			judged["treatment"][0], judged["treatment"][1], judged["control"][0], judged["control"][1])
	}
	return nil
}

func tally(counts map[string][2]int, arm string, pass bool) {
	value := counts[arm]
	if pass {
		value[0]++
	}
	value[1]++
	counts[arm] = value
}

func loadCases(path string) (map[string]studyCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var catalog struct {
		Cases []studyCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	cases := map[string]studyCase{}
	for _, c := range catalog.Cases {
		cases[c.ID] = c
	}
	return cases, nil
}

func loadRows(path string) ([]row, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var rows []row
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1<<20), 16<<20)
	for scanner.Scan() {
		var r row
		if err := json.Unmarshal(scanner.Bytes(), &r); err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, scanner.Err()
}

// readResponse finds the retained response by its run ID, which ends the
// private file name.
func readResponse(runDir, runID string) (string, error) {
	directory := filepath.Join(runDir, "private", "responses")
	matches, err := filepath.Glob(filepath.Join(directory, "*"+runID+".txt"))
	if err != nil {
		return "", err
	}
	if len(matches) != 1 {
		return "", fmt.Errorf("run %s has %d retained responses, want 1", runID, len(matches))
	}
	// A retried arm keeps each attempt as <name>-attempt-N.txt; the published
	// row describes the last one.
	latest := matches[0]
	base := strings.TrimSuffix(latest, ".txt")
	for attempt := 2; ; attempt++ {
		next := fmt.Sprintf("%s-attempt-%d.txt", base, attempt)
		if _, err := os.Stat(next); err != nil {
			break
		}
		latest = next
	}
	data, err := os.ReadFile(latest)
	return string(data), err
}

// labelItems shuffles rows with a fixed seed so labels carry no arm order.
func labelItems(rows []row, responses map[string]string, seed int64) []item {
	ordered := append([]row(nil), rows...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RunID < ordered[j].RunID })
	rand.New(rand.NewSource(seed)).Shuffle(len(ordered), func(i, j int) { ordered[i], ordered[j] = ordered[j], ordered[i] })
	items := make([]item, len(ordered))
	for i, r := range ordered {
		items[i] = item{Label: fmt.Sprintf("R%d", i+1), Response: responses[r.RunID], Row: r}
	}
	return items
}

func buildPrompt(c studyCase, items []item) string {
	var b strings.Builder
	b.WriteString("You grade written answers to one task. Judge meaning, not wording.\n\n")
	b.WriteString("For each response and each required fact F, answer true when the response states that fact in any wording, false otherwise.\n")
	b.WriteString("For each response and each forbidden claim X, answer true only when the response asserts the claim as true. Quoting it, negating it, or explaining why it was removed is not asserting it.\n\n")
	b.WriteString("<task>\n" + c.Task + "\n</task>\n\nRequired facts (alternatives separated by \" / \"):\n")
	for i, fact := range c.RequiredFacts {
		fmt.Fprintf(&b, "F%d: %s\n", i+1, strings.Join(splitAlternatives(fact), " / "))
	}
	b.WriteString("\nForbidden claims:\n")
	for i, fact := range c.ForbiddenFacts {
		fmt.Fprintf(&b, "X%d: %s\n", i+1, strings.Join(splitAlternatives(fact), " / "))
	}
	b.WriteString("\n")
	for _, it := range items {
		fmt.Fprintf(&b, "<response id=%q>\n%s\n</response>\n\n", it.Label, it.Response)
	}
	b.WriteString("Return only one JSON object keyed by response id, for example {\"R1\": {\"required\": {\"F1\": true}, \"forbidden\": {\"X1\": false}}}. Include every response id, every F, and every X.\n")
	return b.String()
}

func splitAlternatives(fact string) []string {
	var out []string
	for _, alternative := range strings.Split(fact, "||") {
		if alternative = strings.TrimSpace(alternative); alternative != "" {
			out = append(out, alternative)
		}
	}
	return out
}

func judgeCase(command, out string, c studyCase, items []item, reuse bool) (map[string]verdict, error) {
	prompt := filepath.Join(out, "prompt-"+c.ID+".txt")
	text := buildPrompt(c, items)
	labels := make([]string, len(items))
	for i, it := range items {
		labels[i] = it.Label
	}
	// Reuse a saved verdict only for a byte-identical prompt, so labels and
	// fact numbering match; rescoring then needs no new judge calls.
	if reuse {
		saved, promptErr := os.ReadFile(prompt)
		verdict, verdictErr := os.ReadFile(filepath.Join(out, "verdict-"+c.ID+".txt"))
		if promptErr == nil && verdictErr == nil && string(saved) == text {
			return parseVerdicts(verdict, labels, len(c.RequiredFacts), len(c.ForbiddenFacts))
		}
	}
	if err := os.WriteFile(prompt, []byte(text), 0o600); err != nil {
		return nil, err
	}
	argv := strings.Fields(strings.ReplaceAll(command, "{prompt_file}", prompt))
	ctx, cancel := context.WithTimeout(context.Background(), judgeTimeout)
	defer cancel()
	stdout, err := exec.CommandContext(ctx, argv[0], argv[1:]...).Output()
	if err != nil {
		return nil, fmt.Errorf("judge command: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "verdict-"+c.ID+".txt"), stdout, 0o600); err != nil {
		return nil, err
	}
	return parseVerdicts(stdout, labels, len(c.RequiredFacts), len(c.ForbiddenFacts))
}

var jsonObject = regexp.MustCompile(`(?s)\{.*\}`)

// parseVerdicts reads the judge's JSON and fails closed on any missing
// response or fact, so a partial answer never becomes a pass.
func parseVerdicts(out []byte, labels []string, required, forbidden int) (map[string]verdict, error) {
	raw := jsonObject.Find(out)
	if raw == nil {
		return nil, errors.New("judge returned no JSON object")
	}
	var parsed map[string]struct {
		Required  map[string]bool `json:"required"`
		Forbidden map[string]bool `json:"forbidden"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("judge JSON: %w", err)
	}
	verdicts := map[string]verdict{}
	for _, label := range labels {
		entry, ok := parsed[label]
		if !ok {
			return nil, fmt.Errorf("judge omitted %s", label)
		}
		v := verdict{Required: make([]bool, required), Forbidden: make([]bool, forbidden)}
		for i := range v.Required {
			value, ok := entry.Required[fmt.Sprintf("F%d", i+1)]
			if !ok {
				return nil, fmt.Errorf("judge omitted %s F%d", label, i+1)
			}
			v.Required[i] = value
		}
		for i := range v.Forbidden {
			value, ok := entry.Forbidden[fmt.Sprintf("X%d", i+1)]
			if !ok {
				return nil, fmt.Errorf("judge omitted %s X%d", label, i+1)
			}
			v.Forbidden[i] = value
		}
		verdicts[label] = v
	}
	return verdicts, nil
}

// judgedOutcome replaces only the fact checks with the judge's verdict and
// keeps every other component of the runner's outcome for the case kind. The
// runner's outcome excludes workflow_success; bounded_return and the dispatch
// contract count only for output-only orchestration. --check verifies this
// rule against rows whose literal facts pass.
func judgedOutcome(r row, c studyCase, v verdict) bool {
	return nonFactOutcome(r, c) && v.pass()
}

func nonFactOutcome(r row, c studyCase) bool {
	for _, key := range []string{"complete_trace", "required_events", "protected_fixture_intact", "oracle_pass", "within_max_words", "required_event_order", "noop_preservation"} {
		if value, ok := r.Metrics[key]; ok && value != 1 {
			return false
		}
	}
	if c.Kind == "orchestration" && c.OrchestrationMode == "output_only" {
		if r.Metrics["bounded_return"] != 1 || r.Metrics["dispatch_contract_success"] != 1 {
			return false
		}
	}
	// forbidden_skill_selections is not part of the outcome: the runner fails
	// only safety skills there, and records no separate metric for them.
	return r.Metrics["forbidden_event_attempts"] == 0
}

// checkConsistency reports rows whose literal facts pass but whose recorded
// outcome disagrees with nonFactOutcome, proving the rule mirrors the runner.
func checkConsistency(cases map[string]studyCase, rows []row) (checked int, mismatches []string) {
	for _, r := range rows {
		c, ok := cases[r.CaseID]
		if !ok || len(c.RequiredFacts)+len(c.ForbiddenFacts) == 0 || r.Status != "completed" || r.Metrics["invented_facts"] != 0 {
			continue
		}
		if retention, has := r.Metrics["fact_retention"]; has && retention != 1 {
			continue
		}
		checked++
		if nonFactOutcome(r, c) != (r.Metrics["outcome_success"] == 1) {
			mismatches = append(mismatches, r.RunID)
		}
	}
	return checked, mismatches
}

// applyLiteralMarkup grades forbidden facts that contain no letters or digits,
// such as "**" or an em dash, by plain substring: they are formatting, not
// claims a judge can weigh.
func applyLiteralMarkup(c studyCase, response string, v *verdict) {
	for i, fact := range c.ForbiddenFacts {
		if i >= len(v.Forbidden) || strings.IndexFunc(fact, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }) >= 0 {
			continue
		}
		for _, alternative := range splitAlternatives(fact) {
			if strings.Contains(response, alternative) {
				v.Forbidden[i] = true
			}
		}
	}
}
