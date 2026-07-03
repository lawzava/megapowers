// score.go — aggregate megapowers eval result rows into a scorecard.
//
// Usage: go run evals/score.go results.jsonl   (or: cat results.jsonl | go run evals/score.go)
//
// Reads JSONL rows emitted by run.sh:
//
//	{"scenario":..,"skill":..,"kind":..,"agent":..,"mode":"skill|control","verdict":"pass|fail|indeterminate","ms":N}
//
// Emits a markdown scorecard: overall pass rate, per-scenario verdicts, and — for
// scenarios run in BOTH skill and control mode — the effect size (Δ pass-rate plus a
// two-proportion z, so a skill's benefit is measured rather than vibed).
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
)

type row struct {
	Scenario string `json:"scenario"`
	Skill    string `json:"skill"`
	Kind     string `json:"kind"`
	Agent    string `json:"agent"`
	Mode     string `json:"mode"`
	Verdict  string `json:"verdict"`
	MS       int    `json:"ms"`
}

type tally struct{ pass, fail, indet int }

func (t *tally) add(v string) {
	switch v {
	case "pass":
		t.pass++
	case "fail":
		t.fail++
	default:
		t.indet++
	}
}

// decided = pass+fail (indeterminate excluded from the rate — never counts as pass).
func (t tally) rate() (float64, int) {
	n := t.pass + t.fail
	if n == 0 {
		return math.NaN(), 0
	}
	return float64(t.pass) / float64(n), n
}

func main() {
	var r *bufio.Scanner
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		defer f.Close()
		r = bufio.NewScanner(f)
	} else {
		r = bufio.NewScanner(os.Stdin)
	}
	r.Buffer(make([]byte, 1024*1024), 1024*1024)

	var overall tally
	perScenario := map[string]*tally{}       // scenario -> tally
	byMode := map[string]map[string]*tally{} // scenario -> mode -> tally
	order := []string{}

	for r.Scan() {
		line := r.Bytes()
		if len(line) == 0 {
			continue
		}
		var x row
		if err := json.Unmarshal(line, &x); err != nil {
			fmt.Fprintf(os.Stderr, "skipping unparseable row: %s\n", line)
			continue
		}
		overall.add(x.Verdict)
		if perScenario[x.Scenario] == nil {
			perScenario[x.Scenario] = &tally{}
			byMode[x.Scenario] = map[string]*tally{}
			order = append(order, x.Scenario)
		}
		perScenario[x.Scenario].add(x.Verdict)
		if byMode[x.Scenario][x.Mode] == nil {
			byMode[x.Scenario][x.Mode] = &tally{}
		}
		byMode[x.Scenario][x.Mode].add(x.Verdict)
	}
	if err := r.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	sort.Strings(order)

	rate, n := overall.rate()
	fmt.Println("# megapowers eval scorecard")
	fmt.Println()
	if n == 0 {
		fmt.Println("No decided results.")
		return
	}
	fmt.Printf("**Overall:** %d/%d passed (%.0f%%), %d indeterminate.\n\n",
		overall.pass, n, rate*100, overall.indet)

	fmt.Println("## Per scenario")
	fmt.Println()
	fmt.Println("| scenario | pass | fail | indet |")
	fmt.Println("|---|---|---|---|")
	for _, s := range order {
		t := perScenario[s]
		fmt.Printf("| %s | %d | %d | %d |\n", s, t.pass, t.fail, t.indet)
	}
	fmt.Println()

	// Effect size where a scenario was run in both skill and control mode.
	type eff struct {
		scenario         string
		p1, p2, delta, z float64
		n1, n2           int
	}
	var effs []eff
	for _, s := range order {
		sk, ct := byMode[s]["skill"], byMode[s]["control"]
		if sk == nil || ct == nil {
			continue
		}
		p1, n1 := sk.rate()
		p2, n2 := ct.rate()
		if n1 == 0 || n2 == 0 {
			continue
		}
		pooled := float64(sk.pass+ct.pass) / float64(n1+n2)
		se := math.Sqrt(pooled * (1 - pooled) * (1.0/float64(n1) + 1.0/float64(n2)))
		z := math.NaN()
		if se > 0 {
			z = (p1 - p2) / se
		}
		effs = append(effs, eff{s, p1, p2, p1 - p2, z, n1, n2})
	}
	if len(effs) > 0 {
		fmt.Println("## Skill effect size (skill vs control)")
		fmt.Println()
		fmt.Println("| scenario | skill pass% (n) | control pass% (n) | Δ | z |")
		fmt.Println("|---|---|---|---|---|")
		for _, e := range effs {
			zs := "n/a"
			if !math.IsNaN(e.z) {
				zs = fmt.Sprintf("%.2f", e.z)
			}
			fmt.Printf("| %s | %.0f%% (%d) | %.0f%% (%d) | %+.0f%% | %s |\n",
				e.scenario, e.p1*100, e.n1, e.p2*100, e.n2, e.delta*100, zs)
		}
		fmt.Println()
		fmt.Println("_z is a two-proportion z-score; |z|>1.96 ≈ p<0.05. Small n → treat as directional, and grow the run count before claiming significance._")
	}
}
