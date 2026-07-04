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

// logFactorial returns log(n!) via lgamma(n+1); it never overflows.
func logFactorial(n int) float64 {
	v, _ := math.Lgamma(float64(n + 1))
	return v
}

// logChoose returns log(C(n, k)).
func logChoose(n, k int) float64 {
	return logFactorial(n) - logFactorial(k) - logFactorial(n-k)
}

// logHyper returns the log of the exact hypergeometric point probability of the
// 2x2 table [[a, b], [c, d]] with all four margins held fixed.
func logHyper(a, b, c, d int) float64 {
	return logChoose(a+b, a) + logChoose(c+d, c) - logChoose(a+b+c+d, a+c)
}

// fisherTwoSided returns the two-sided Fisher exact p-value for the 2x2 table
// [[a, b], [c, d]] (row 1 = skill pass/fail, row 2 = control pass/fail).
//
// Convention: hold all four margins fixed and sum the exact hypergeometric
// point probability of every table whose point probability is <= the observed
// table's (the standard "sum of tables at least as extreme" two-sided rule that
// R's fisher.test uses). Log-factorials keep the factorials from overflowing;
// at the n used here (<=36) the float64 accumulation is exact to far under the
// 1e-9 the self-test asserts. Exact ties are counted via a small relative
// tolerance (1e-7, matching R), so the two boundary tables of a 12/12-vs-0/12
// split both contribute.
func fisherTwoSided(a, b, c, d int) float64 {
	n1 := a + b // skill row total
	n2 := c + d // control row total
	k := a + c  // total passes (column-1 total)
	logP0 := logHyper(a, b, c, d)
	lo := 0
	if k-n2 > lo {
		lo = k - n2
	}
	hi := n1
	if k < hi {
		hi = k
	}
	const tol = 1e-7 // relative tie tolerance in log space, per R's fisher.test
	sum := 0.0
	for aa := lo; aa <= hi; aa++ {
		bb := n1 - aa
		cc := k - aa
		dd := n2 - cc
		lp := logHyper(aa, bb, cc, dd)
		if lp <= logP0+tol {
			sum += math.Exp(lp)
		}
	}
	if sum > 1 {
		sum = 1
	}
	return sum
}

// selftest verifies the Fisher exact test against three known 2x2 tables and
// returns the number of failed assertions (0 = pass). Wired into run-all.sh so
// a statistics regression fails the suite.
func selftest() int {
	fails := 0
	check := func(name string, got, want, tol float64) {
		if math.Abs(got-want) > tol {
			fmt.Printf("FAIL %s: got %.12g want %.12g (tol %g)\n", name, got, want, tol)
			fails++
		} else {
			fmt.Printf("ok   %s: %.12g (want %.12g)\n", name, got, want)
		}
	}
	// 12/12 vs 0/12: the two most extreme tables of a 24-run split.
	// one-tail = 1/C(24,12) = 3.698e-07; two-sided = 2/C(24,12) = 7.396e-07.
	check("12/12 vs 0/12 one-tail", math.Exp(logHyper(12, 0, 0, 12)), 3.698e-07, 1e-9)
	check("12/12 vs 0/12 fisher_p", fisherTwoSided(12, 0, 0, 12), 7.396e-07, 1e-9)
	// 3/10 vs 2/10: every table is at least as probable as the observed one,
	// so the two-sided p-value is exactly 1.
	check("3/10 vs 2/10 fisher_p", fisherTwoSided(3, 7, 2, 8), 1.0, 1e-9)
	if fails == 0 {
		fmt.Println("score.go selftest: PASS")
	} else {
		fmt.Printf("score.go selftest: FAIL (%d assertion(s))\n", fails)
	}
	return fails
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--selftest" {
		if selftest() != 0 {
			os.Exit(1)
		}
		return
	}
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
		fisher           float64
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
		// Fisher exact is valid at these n (and 0%/100% cells) where the pooled
		// z's normal approximation is not; both are reported so the reader can
		// prefer the exact p-value.
		fisher := fisherTwoSided(sk.pass, sk.fail, ct.pass, ct.fail)
		effs = append(effs, eff{s, p1, p2, p1 - p2, z, fisher, n1, n2})
	}
	if len(effs) > 0 {
		fmt.Println("## Skill effect size (skill vs control)")
		fmt.Println()
		fmt.Println("| scenario | skill pass% (n) | control pass% (n) | Δ | z | fisher_p |")
		fmt.Println("|---|---|---|---|---|---|")
		for _, e := range effs {
			zs := "n/a"
			if !math.IsNaN(e.z) {
				zs = fmt.Sprintf("%.2f", e.z)
			}
			fmt.Printf("| %s | %.0f%% (%d) | %.0f%% (%d) | %+.0f%% | %s | %.3g |\n",
				e.scenario, e.p1*100, e.n1, e.p2*100, e.n2, e.delta*100, zs, e.fisher)
		}
		fmt.Println()
		fmt.Println("_z is a two-proportion z-score; |z|>1.96 ≈ p<0.05. fisher_p is the two-sided Fisher exact p-value, valid at small n and boundary (0%/100%) cells where z is not. Small n → treat as directional, and grow the run count before claiming significance._")
	}
}
