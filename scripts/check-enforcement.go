package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	os.Exit(runCheckEnforcement(os.Args[1:]))
}

func runCheckEnforcement(args []string) int {
	root := os.Getenv("MEGAPOWERS_ROOT")
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(os.Stderr, "check-enforcement: %v\n", err)
			return 2
		}
		root = wd
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--root":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "unknown arg: --root")
				return 2
			}
			root = args[i+1]
			i++
		default:
			fmt.Fprintf(os.Stderr, "unknown arg: %s\n", args[i])
			return 2
		}
	}
	if err := os.Chdir(root); err != nil {
		fmt.Fprintf(os.Stderr, "check-enforcement: %v\n", err)
		return 2
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

	fmt.Println("== enforcement lifecycle ==")
	files, err := filepath.Glob("plugins/*/enforcement.toml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "check-enforcement: %v\n", err)
		return 2
	}
	if len(files) == 0 {
		bad("no plugins/*/enforcement.toml found")
		fmt.Printf("== summary: %d passed, %d failed ==\n", pass, fail)
		return 1
	}

	ruleHeader := regexp.MustCompile(`^\[rules\.[A-Za-z0-9_-]+\]$`)
	dateRE := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}$`)

	for _, file := range files {
		pluginRoot := filepath.Dir(file)
		relPlugin := strings.TrimPrefix(pluginRoot, "plugins/")
		ids := ruleIDs(file, ruleHeader)
		if len(ids) == 0 {
			bad(file + " declares no [rules.*]")
			continue
		}
		for _, id := range ids {
			sec := "rules." + id
			where := relPlugin + "/" + id
			state := tomlScalar(file, sec, "state")
			switch state {
			case "off", "advisory", "enforced":
				ok(where + " state=" + state)
			case "":
				bad(where + " has no state (expected off, advisory, or enforced)")
			default:
				bad(where + " state='" + state + "' is none of off, advisory, enforced; a consumer reads any unknown value as off, so a typo silently disables the rule")
			}

			hook := tomlScalar(file, sec, "hook")
			if hook == "" {
				bad(where + " names no hook")
			} else if _, err := os.Stat(filepath.Join(pluginRoot, hook)); err != nil {
				bad(fmt.Sprintf("%s names hook '%s' which does not exist under %s", where, hook, pluginRoot))
			} else {
				ok(fmt.Sprintf("%s hook exists (%s)", where, hook))
				ctest := tomlScalar(file, sec, "contract_test")
				if ctest == "" {
					bad(where + " names no contract_test, so nothing proves its declared state is the state that runs")
				} else if _, err := os.Stat(filepath.Join(pluginRoot, ctest)); err != nil {
					bad(fmt.Sprintf("%s names contract_test '%s' which does not exist under %s", where, ctest, pluginRoot))
				} else {
					hits := countLiteral(filepath.Join(pluginRoot, ctest), id)
					if hits > 0 {
						ok(fmt.Sprintf("%s contract_test names the rule (%s)", where, ctest))
					} else {
						bad(fmt.Sprintf("%s contract_test '%s' never names '%s', so nothing ties it to this rule", where, ctest, id))
					}
				}
			}

			sourcePath := tomlScalar(file, sec, "source")
			if sourcePath == "" {
				bad(where + " names no source skill")
			} else if _, err := os.Stat(filepath.Join(pluginRoot, sourcePath)); err != nil {
				bad(fmt.Sprintf("%s names source '%s' which does not exist under %s", where, sourcePath, pluginRoot))
			} else {
				ok(fmt.Sprintf("%s source exists (%s)", where, sourcePath))
			}

			datekey := "declared"
			if state == "enforced" {
				datekey = "promoted"
			}
			dateval := tomlScalar(file, sec, datekey)
			if dateRE.MatchString(dateval) {
				ok(fmt.Sprintf("%s %s=%s", where, datekey, dateval))
			} else {
				bad(fmt.Sprintf("%s has no parseable %s date (expected YYYY-MM-DD, got '%s')", where, datekey, dateval))
			}

			if tomlSectionExists(file, sec+".scope") {
				kws := tomlArray(file, sec+".scope", "keywords")
				if len(kws) > 0 {
					ok(fmt.Sprintf("%s scope declares %d keywords", where, len(kws)))
				} else {
					bad(where + " declares a scope with no keywords; an empty list matches nothing while looking configured")
				}
			}
		}
	}
	fmt.Printf("== summary: %d passed, %d failed ==\n", pass, fail)
	if fail != 0 {
		return 1
	}
	return 0
}

func ruleIDs(path string, re *regexp.Regexp) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var ids []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		h := strings.ReplaceAll(sc.Text(), " ", "")
		h = strings.ReplaceAll(h, "\t", "")
		if re.MatchString(h) {
			n := strings.TrimPrefix(h, "[rules.")
			n = strings.TrimSuffix(n, "]")
			ids = append(ids, n)
		}
	}
	return ids
}

func compactHeader(line string) string {
	return strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, line)
}

func tomlSectionExists(path, section string) bool {
	want := "[" + section + "]"
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if compactHeader(sc.Text()) == want {
			return true
		}
	}
	return false
}

func tomlScalar(path, section, key string) string {
	want := "[" + section + "]"
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	in := false
	keyRE := regexp.MustCompile(`^[[:space:]]*` + regexp.QuoteMeta(key) + `[[:space:]]*=`)
	for sc.Scan() {
		line := sc.Text()
		h := compactHeader(line)
		if h == want {
			in = true
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "[") {
			in = false
		}
		if !in || !keyRE.MatchString(line) {
			continue
		}
		rest := line[strings.Index(line, "=")+1:]
		rest = strings.TrimSpace(rest)
		if strings.HasPrefix(rest, `"`) {
			rest = rest[1:]
			if i := strings.Index(rest, `"`); i >= 0 {
				rest = rest[:i]
			}
			return rest
		}
		if strings.HasPrefix(rest, `'`) {
			rest = rest[1:]
			if i := strings.Index(rest, `'`); i >= 0 {
				rest = rest[:i]
			}
			return rest
		}
		if i := strings.Index(rest, "#"); i >= 0 {
			rest = rest[:i]
		}
		return strings.TrimRight(rest, " \t")
	}
	return ""
}

func tomlArray(path, section, key string) []string {
	want := "[" + section + "]"
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	in := false
	keyRE := regexp.MustCompile(`^[[:space:]]*` + regexp.QuoteMeta(key) + `[[:space:]]*=`)
	q := regexp.MustCompile(`"[^"]*"`)
	for sc.Scan() {
		line := sc.Text()
		h := compactHeader(line)
		if h == want {
			in = true
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line, " \t"), "[") {
			in = false
		}
		if !in || !keyRE.MatchString(line) {
			continue
		}
		rest := line[strings.Index(line, "=")+1:]
		var out []string
		for _, m := range q.FindAllString(rest, -1) {
			out = append(out, m[1:len(m)-1])
		}
		return out
	}
	return nil
}

func countLiteral(path, needle string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	return strings.Count(string(data), needle)
}
