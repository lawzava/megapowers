package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const defaultMaxAge = 30

// validate.sh calls this with a huge --max-age-days as a format guard.

type freshnessEntry struct {
	file    string
	marker  string
	maxAge  int
	hasAge  bool
}

// Codex config reviewed lives on the shell wrapper (scripts/check-freshness.sh).
var freshnessFiles = []freshnessEntry{
	{file: "docs/harness-support.md", marker: "Last reviewed:"},
	{file: "evals/RESULTS.md", marker: "Last run:"},
	{file: "plugins/mega-orchestration/enforcement.toml", marker: "Last reviewed:"},
	{file: "plugins/megapowers/enforcement.toml", marker: "Last reviewed:"},
	{file: "plugins/mega-orchestration/skills/multi-agent-delegation/delegates.toml", marker: "Last reviewed:"},
	{file: "plugins/mega-orchestration/skills/multi-agent-delegation/references/providers/codex.md", marker: "Last reviewed:", maxAge: 30, hasAge: true},
	{file: "plugins/mega-orchestration/skills/multi-agent-delegation/references/providers/opencode.md", marker: "Last reviewed:", maxAge: 30, hasAge: true},
	{file: "plugins/megapowers/models.toml", marker: "Last reviewed:"},
	{file: "plugins/mega-orchestration/skills/orchestrating/references/harness-primitives.md", marker: "Last reviewed:", maxAge: 30, hasAge: true},
	{file: "plugins/mega-frontend/skills/designing-frontends/SKILL.md", marker: "Calibration reviewed:"},
	{file: "scripts/check-freshness.sh", marker: "Codex config reviewed:", maxAge: 30, hasAge: true},
}

var dateRE = regexp.MustCompile(`[0-9]{4}-[0-9]{2}-[0-9]{2}`)

func main() {
	os.Exit(runCheckFreshness(os.Args[1:]))
}

func runCheckFreshness(args []string) int {
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-freshness: %v\n", err)
			return 2
		}
		root = wd
	}
	maxAge := defaultMaxAge
	flagSet := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--max-age-days":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "unknown arg: --max-age-days")
				return 2
			}
			n, err := strconv.Atoi(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "unknown arg: %s\n", args[i+1])
				return 2
			}
			maxAge = n
			flagSet = true
			i++
		default:
			fmt.Fprintf(os.Stderr, "unknown arg: %s\n", args[i])
			return 2
		}
	}
	pass, fail := 0, 0
	ok := func(msg string) {
		fmt.Printf("  \033[32mPASS\033[0m %s\n", msg)
		pass++
	}
	bad := func(msg string) {
		fmt.Printf("  \033[31mFAIL\033[0m %s\n", msg)
		fail++
	}
	now := time.Now()
	fmt.Printf("== freshness (max age: %d days) ==\n", maxAge)
	for _, e := range freshnessFiles {
		path := e.file
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, e.file)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			bad(e.file + " missing")
			continue
		}
		var d string
		for _, line := range strings.Split(string(data), "\n") {
			if strings.Contains(strings.ToLower(line), strings.ToLower(e.marker)) {
				if m := dateRE.FindString(line); m != "" {
					d = m
					break
				}
			}
		}
		if d == "" {
			bad(fmt.Sprintf("%s: no '%s YYYY-MM-DD' line found", e.file, e.marker))
			continue
		}
		then, err := time.ParseInLocation("2006-01-02", d, time.Local)
		if err != nil {
			bad(fmt.Sprintf("%s: unparseable date '%s'", e.file, d))
			continue
		}
		limit := defaultMaxAge
		if flagSet {
			limit = maxAge
		} else if e.hasAge {
			limit = e.maxAge
		}
		age := int(now.Sub(then).Hours() / 24)
		if age <= limit {
			ok(fmt.Sprintf("%s reviewed %dd ago (%s)", e.file, age, d))
		} else {
			bad(fmt.Sprintf("%s reviewed %dd ago (%s) — re-review its opinions, then bump the date", e.file, age, d))
		}
	}
	fmt.Printf("== summary: %d passed, %d failed ==\n", pass, fail)
	if fail != 0 {
		return 1
	}
	return 0
}
