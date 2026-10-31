// session-mine reads local Claude Code and Codex transcripts, read-only, and
// reports where the human corrected the agent (corrections), which earlier
// sessions discussed a topic (recall), or per-project outcome signals
// (scorecard). Every excerpt is redacted.
package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const usageText = `usage: go run session-mine.go [corrections|recall|scorecard] (--workspace DIR | --all-workspaces)
  [--since YYYY-MM-DD] [--harness claude|codex|both] [--claude-root DIR]
  [--codex-root DIR] [--format text|json] [--limit N] [--topic "WORDS" (recall only)]
  [--sanitize (scorecard only)]`

const (
	correctionExcerpt = 160
	recallExcerpt     = 240
)

type options struct {
	mode       string
	workspace  string
	all        bool
	since      time.Time
	hasSince   bool
	harness    string
	claudeRoot string
	codexRoots []string
	format     string
	limit      int
	topic      []string
	sanitize   bool
}

// A message is one kept text record. Role is "human" (a person typed it),
// "machine" (a harness or scheduler wrote the user turn), "prompt" (a lead
// agent prompted a subagent), or "assistant".
type message struct {
	role string
	text string
	ts   time.Time
	cwd  string
}

type session struct {
	harness string
	base    string
	cwd     string
	msgs    []message
	events  []event
}

// An event is a tool action the scorecard counts: a skill load, a megapowers
// gate block, or a foreground sleep. after is the number of kept messages
// that precede it, so a correction can be tied to the skills loaded since
// the previous human turn.
type event struct {
	kind  string
	name  string
	ts    time.Time
	cwd   string
	after int
}

type scanStats struct {
	skippedLines    int
	skippedFiles    int
	unreadableFiles int
}

type example struct {
	Harness   string `json:"harness"`
	Session   string `json:"session"`
	Timestamp string `json:"timestamp"`
	User      string `json:"user"`
	Agent     string `json:"agent"`
}

type classReport struct {
	Name     string    `json:"name"`
	Count    int       `json:"count"`
	Examples []example `json:"examples"`
}

type correctionsReport struct {
	Note            string        `json:"note"`
	SessionsScanned int           `json:"sessions_scanned"`
	HumanTurns      int           `json:"human_turns"`
	MachineTurns    int           `json:"machine_turns"`
	ClassifiedTurns int           `json:"classified_turns"`
	OtherTurns      int           `json:"other_turns"`
	SkippedLines    int           `json:"skipped_lines"`
	SkippedFiles    int           `json:"skipped_files"`
	UnreadableFiles int           `json:"unreadable_files"`
	Classes         []classReport `json:"classes"`
	OneOffs         []classReport `json:"one_offs"`
}

type recallItem struct {
	Harness        string `json:"harness"`
	Session        string `json:"session"`
	FirstTimestamp string `json:"first_timestamp"`
	LastTimestamp  string `json:"last_timestamp"`
	FirstPrompt    string `json:"first_prompt"`
	LastReply      string `json:"last_reply"`
	last           time.Time
}

type recallReport struct {
	Topic           string       `json:"topic"`
	SessionsScanned int          `json:"sessions_scanned"`
	SkippedLines    int          `json:"skipped_lines"`
	SkippedFiles    int          `json:"skipped_files"`
	UnreadableFiles int          `json:"unreadable_files"`
	Sessions        []recallItem `json:"sessions"`
}

type classRule struct {
	name    string
	pattern *regexp.Regexp
}

var classRules = []classRule{
	rule("stopped-unnecessarily", "why did you stop", "don't ask", "do not ask", "just do it", "go ahead", "you can do", "keep going", "continue", "don't stop"),
	rule("verification", "did you test", "not working", "still broken", "you didn't verify", "doesn't work", "does not work", "did you verify"),
	rule("shallow", "superficial", "try harder", "dig deeper", "lazy", "not thorough"),
	rule("verbosity", "too long", "shorter", "slop", "verbose", "wall of text"),
	rule("overreach", "i didn't ask", "i did not ask", "revert", "don't change", "why did you change", "undo"),
	rule("frustration", "wtf", "stop lying", "again?", "how many times"),
}

func rule(name string, phrases ...string) classRule {
	parts := make([]string, len(phrases))
	for i, phrase := range phrases {
		part := `\b` + regexp.QuoteMeta(phrase)
		if last := rune(phrase[len(phrase)-1]); unicode.IsLetter(last) || unicode.IsDigit(last) {
			part += `\b`
		}
		parts[i] = part
	}
	return classRule{name: name, pattern: regexp.MustCompile(`(?i)` + strings.Join(parts, "|"))}
}

var (
	// Wrapper-tagged blocks are harness injections: slash-command echoes,
	// system reminders, task notifications, Codex environment and skill blocks.
	injectedTag      = regexp.MustCompile(`^<[a-z][a-z0-9_-]*[ >]`)
	openTag          = regexp.MustCompile(`<([a-z][a-z0-9_-]*)(?:\s[^<>]*)?>`)
	injectedPrefixes = []string{"Base directory for this skill:", "# AGENTS.md", "[Request interrupted by user", "Caveat: The messages below were generated"}

	reUserinfo = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^/\s@]+@`)
	reHomePath = regexp.MustCompile(`(?i:\b[a-z]:[\\/]users[\\/][^\\/]+)|(?:/home|/Users)/[A-Za-z0-9._-]+|/root\b`)
	// The literal $HOME and USERPROFILE values, when set, become "~" as well.
	reHomeValues = homeValuePatterns()
	reBearer     = regexp.MustCompile(`(?i)\bBearer\s+[A-Za-z0-9._~+/=-]+`)
	reTokens     = []*regexp.Regexp{
		regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
		regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}`),
		regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
		regexp.MustCompile(`\bxox[abp]-[A-Za-z0-9-]{10,}`),
		regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	}
	reEmail   = regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`)
	reLongB64 = regexp.MustCompile(`[A-Za-z0-9+/=_-]{32,}`)
)

func homeValuePatterns() []*regexp.Regexp {
	var patterns []*regexp.Regexp
	for _, name := range []string{"HOME", "USERPROFILE"} {
		if value := strings.TrimRight(os.Getenv(name), `/\`); len(value) > 1 {
			patterns = append(patterns, regexp.MustCompile(regexp.QuoteMeta(value)+`([^A-Za-z0-9._-]|$)`))
		}
	}
	return patterns
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	opt, err := parseOptions(args)
	if err != nil {
		fmt.Fprintf(stderr, "session-mine: %v\n%s\n", err, usageText)
		return 2
	}
	var stats scanStats
	sessions := loadSessions(opt, &stats)
	var report any
	switch opt.mode {
	case "recall":
		report = recall(opt, sessions, stats)
	case "scorecard":
		report = scorecard(opt, sessions, stats)
	default:
		report = corrections(opt, sessions, stats)
	}
	if opt.format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetEscapeHTML(false)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			fmt.Fprintf(stderr, "session-mine: %v\n", err)
			return 1
		}
		return 0
	}
	switch typed := report.(type) {
	case correctionsReport:
		printCorrections(stdout, typed)
	case recallReport:
		printRecall(stdout, typed)
	case scorecardReport:
		printScorecard(stdout, typed)
	}
	return 0
}

func parseOptions(args []string) (options, error) {
	opt := options{mode: "corrections", harness: "both", format: "text", limit: -1}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		opt.mode = args[0]
		args = args[1:]
	}
	if opt.mode != "corrections" && opt.mode != "recall" && opt.mode != "scorecard" {
		return opt, fmt.Errorf("unknown mode %q", opt.mode)
	}
	seen := map[string]bool{}
	var topic string
	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		if seen[name] {
			return opt, fmt.Errorf("%s may be provided only once", name)
		}
		seen[name] = true
		if name == "--all-workspaces" || name == "--sanitize" {
			if hasValue {
				return opt, fmt.Errorf("%s takes no value", name)
			}
			opt.all = opt.all || name == "--all-workspaces"
			opt.sanitize = opt.sanitize || name == "--sanitize"
			continue
		}
		switch name {
		case "--workspace", "--since", "--harness", "--claude-root", "--codex-root", "--format", "--limit", "--topic":
		default:
			return opt, fmt.Errorf("unknown argument %q", args[i])
		}
		if !hasValue {
			if i+1 >= len(args) {
				return opt, fmt.Errorf("%s requires a value", name)
			}
			i++
			value = args[i]
		}
		switch name {
		case "--workspace":
			opt.workspace = value
		case "--since":
			since, err := time.Parse("2006-01-02", value)
			if err != nil {
				return opt, fmt.Errorf("--since must use YYYY-MM-DD")
			}
			opt.since, opt.hasSince = since, true
		case "--harness":
			opt.harness = value
		case "--claude-root":
			opt.claudeRoot = value
		case "--codex-root":
			if value != "" {
				opt.codexRoots = []string{value}
			}
		case "--format":
			opt.format = value
		case "--limit":
			limit, err := strconv.Atoi(value)
			if err != nil || limit < 1 {
				return opt, fmt.Errorf("--limit must be a positive integer")
			}
			opt.limit = limit
		case "--topic":
			topic = value
		}
	}
	if opt.harness != "claude" && opt.harness != "codex" && opt.harness != "both" {
		return opt, fmt.Errorf("--harness must be claude, codex, or both")
	}
	if opt.format != "text" && opt.format != "json" {
		return opt, fmt.Errorf("--format must be text or json")
	}
	if opt.all && opt.workspace != "" {
		return opt, fmt.Errorf("use --workspace or --all-workspaces, not both")
	}
	if !opt.all {
		if strings.TrimSpace(opt.workspace) == "" {
			return opt, fmt.Errorf("--workspace is required unless --all-workspaces is set")
		}
		abs, err := filepath.Abs(opt.workspace)
		if err != nil {
			return opt, fmt.Errorf("cannot resolve --workspace")
		}
		opt.workspace = filepath.Clean(abs)
	}
	opt.topic = strings.Fields(strings.ToLower(topic))
	if opt.mode == "recall" && len(opt.topic) == 0 {
		return opt, fmt.Errorf("recall requires --topic")
	}
	if opt.mode != "recall" && topic != "" {
		return opt, fmt.Errorf("--topic applies only to recall")
	}
	if opt.sanitize && opt.mode != "scorecard" {
		return opt, fmt.Errorf("--sanitize applies only to scorecard")
	}
	if opt.limit < 0 {
		opt.limit = 5
		if opt.mode == "recall" {
			opt.limit = 10
		}
	}
	home, _ := os.UserHomeDir()
	if opt.claudeRoot == "" {
		if dir := os.Getenv("CLAUDE_CONFIG_DIR"); dir != "" {
			opt.claudeRoot = filepath.Join(dir, "projects")
		} else if home != "" {
			opt.claudeRoot = filepath.Join(home, ".claude", "projects")
		}
	}
	if len(opt.codexRoots) == 0 {
		dir := os.Getenv("CODEX_HOME")
		if dir == "" && home != "" {
			dir = filepath.Join(home, ".codex")
		}
		if dir != "" {
			opt.codexRoots = []string{filepath.Join(dir, "sessions"), filepath.Join(dir, "archived_sessions")}
		}
	}
	return opt, nil
}

// claudeSlug mirrors Claude Code's project directory name: every character
// other than an ASCII letter or digit becomes "-".
func claudeSlug(path string) string {
	var b strings.Builder
	for _, r := range path {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	return b.String()
}

// within reports whether path is dir or below it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func loadSessions(opt options, stats *scanStats) []session {
	var sessions []session
	if opt.harness != "codex" && opt.claudeRoot != "" {
		if root, err := filepath.EvalSymlinks(opt.claudeRoot); err == nil {
			var dirs []string
			if opt.all {
				entries, _ := os.ReadDir(root)
				for _, entry := range entries {
					dirs = append(dirs, filepath.Join(root, entry.Name()))
				}
			} else {
				dirs = []string{filepath.Join(root, claudeSlug(opt.workspace))}
			}
			for _, dir := range dirs {
				top, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
				sub, _ := filepath.Glob(filepath.Join(dir, "*", "subagents", "*.jsonl"))
				for _, path := range append(top, sub...) {
					if s, ok := readFile(path, root, "claude", opt, stats); ok {
						sessions = append(sessions, s)
					}
				}
			}
		}
	}
	if opt.harness != "claude" {
		seen := map[string]bool{}
		for _, dir := range opt.codexRoots {
			root, err := filepath.EvalSymlinks(dir)
			if err != nil {
				continue
			}
			paths, _ := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9][0-9]", "[0-9][0-9]", "[0-9][0-9]", "rollout-*.jsonl"))
			flat, _ := filepath.Glob(filepath.Join(root, "rollout-*.jsonl"))
			for _, path := range append(paths, flat...) {
				name := filepath.Base(path)
				if seen[name] {
					continue
				}
				seen[name] = true
				if s, ok := readFile(path, root, "codex", opt, stats); ok {
					sessions = append(sessions, s)
				}
			}
		}
	}
	return sessions
}

// readFile returns a session with at least one in-range message. A file whose
// real path leaves the resolved store root, or is not a regular file, is
// skipped and counted.
func readFile(path, root, harness string, opt options, stats *scanStats) (session, bool) {
	s := session{harness: harness, base: filepath.Base(path)}
	real, err := filepath.EvalSymlinks(path)
	if err != nil || !within(real, root) {
		stats.skippedFiles++
		return s, false
	}
	info, err := os.Stat(real)
	if err != nil || !info.Mode().IsRegular() {
		stats.skippedFiles++
		return s, false
	}
	if opt.hasSince && info.ModTime().Before(opt.since) {
		return s, false
	}
	file, err := os.Open(real)
	if err != nil {
		stats.unreadableFiles++
		return s, false
	}
	defer file.Close()
	reader := bufio.NewReaderSize(file, 1<<20)
	codexScoped, codexSubagent, codexExec := false, false, false
	origin := ""
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			var msgs []message
			var events []event
			var ok bool
			if harness == "claude" {
				var cwd string
				msgs, events, cwd, ok = parseClaude(line)
				// Workspaces can share a slug (/work/a-b and /work/a/b), so the
				// first recorded cwd decides which workspace owns the file.
				if origin == "" && cwd != "" {
					origin = cwd
					if !opt.all && !within(origin, opt.workspace) {
						return s, false
					}
				}
			} else {
				var meta *codexMeta
				msgs, events, meta, ok = parseCodex(line, codexSubagent, codexExec)
				if meta != nil {
					if !opt.all && filepath.Clean(meta.cwd) != opt.workspace {
						return s, false
					}
					codexScoped, codexSubagent, codexExec = true, meta.subagent, meta.exec
					s.cwd = filepath.Clean(meta.cwd)
				}
			}
			if !ok {
				stats.skippedLines++
			}
			for _, msg := range msgs {
				if opt.hasSince && (msg.ts.IsZero() || msg.ts.Before(opt.since)) {
					continue
				}
				s.msgs = append(s.msgs, msg)
			}
			for _, ev := range events {
				if opt.hasSince && (ev.ts.IsZero() || ev.ts.Before(opt.since)) {
					continue
				}
				ev.after = len(s.msgs)
				s.events = append(s.events, ev)
			}
		}
		if readErr != nil {
			if !errors.Is(readErr, io.EOF) {
				stats.unreadableFiles++
			}
			break
		}
	}
	if harness == "codex" && !codexScoped && !opt.all {
		return s, false
	}
	if origin != "" {
		s.cwd = origin
	} else if harness == "claude" {
		// Without a recorded cwd, the project directory is the only scope, so
		// files from different projects never share a scorecard workspace.
		rel, _ := filepath.Rel(root, real)
		s.cwd = "claude-project:" + strings.Split(filepath.ToSlash(rel), "/")[0]
	}
	if origin != "" && !opt.all {
		// In a file that records cwd, a record without one cannot be scoped.
		// prefix[i] counts kept messages among the first i, so event
		// positions stay attached to the same neighbours.
		prefix := make([]int, len(s.msgs)+1)
		kept := s.msgs[:0]
		for i, msg := range s.msgs {
			prefix[i+1] = prefix[i]
			if msg.cwd != "" {
				kept = append(kept, msg)
				prefix[i+1]++
			}
		}
		s.msgs = kept
		events := s.events[:0]
		for _, ev := range s.events {
			if ev.cwd != "" {
				ev.after = prefix[ev.after]
				events = append(events, ev)
			}
		}
		s.events = events
	}
	return s, len(s.msgs) > 0
}

type contentBlock struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Name    string          `json:"name"`
	Input   json.RawMessage `json:"input"`
	Content json.RawMessage `json:"content"`
	IsError bool            `json:"is_error"`
}

func injected(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || injectedTag.MatchString(trimmed) {
		return true
	}
	for _, prefix := range injectedPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return false
}

func parseTime(raw string) time.Time {
	ts, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}
	}
	return ts.UTC()
}

type claudeRecord struct {
	Type         string `json:"type"`
	Cwd          string `json:"cwd"`
	Timestamp    string `json:"timestamp"`
	IsMeta       bool   `json:"isMeta"`
	IsSidechain  bool   `json:"isSidechain"`
	PromptSource string `json:"promptSource"`
	// A compaction summary is written by the harness, not typed by the user.
	IsCompactSummary bool `json:"isCompactSummary"`
	// Origin names the producer of a non-human turn, such as a task
	// notification or a coordinator. Human turns carry none.
	Origin  json.RawMessage `json:"origin"`
	Message *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// originIsMachine reports whether the record names a non-human producer.
func (r claudeRecord) originIsMachine() bool {
	var origin struct {
		Kind string `json:"kind"`
	}
	return json.Unmarshal(r.Origin, &origin) == nil && origin.Kind != "" && origin.Kind != "human"
}

var (
	// Anchored fallbacks for machine-written turns that carry no flag: a
	// compaction summary, an autonomous-run prompt or its continuation, and
	// a schedule prompt's unattended-run paragraph. A line start keeps a human
	// sentence that merely quotes the phrase from matching.
	machinePrefixes = []string{
		"This session is being continued from a previous conversation",
		"This is an approved autonomous run",
		"Continue the approved run and finish it",
	}
	reUnattended = regexp.MustCompile(`(?mi)^unattended run:`)
)

// machineText reports whether a user turn's text was written by a harness,
// an autonomous run, or a scheduler rather than typed by a person.
func machineText(text string) bool {
	trimmed := strings.TrimSpace(text)
	for _, prefix := range machinePrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			return true
		}
	}
	return reUnattended.MatchString(trimmed)
}

// humanText strips embedded tag blocks and reports whether human text remains.
func humanText(raw string) (string, bool) {
	text := stripTagged(raw)
	return text, !injected(text)
}

// parseClaude returns the record's kept messages, its scorecard events, and
// its cwd, if any.
func parseClaude(line []byte) ([]message, []event, string, bool) {
	var record claudeRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return nil, nil, "", false
	}
	cwd := ""
	if record.Cwd != "" {
		cwd = filepath.Clean(record.Cwd)
	}
	msgs, ok := claudeMessages(record)
	for i := range msgs {
		msgs[i].cwd = cwd
	}
	events := claudeEvents(record)
	for i := range events {
		events[i].cwd = cwd
	}
	return msgs, events, cwd, ok
}

func claudeEvents(record claudeRecord) []event {
	if (record.Type != "user" && record.Type != "assistant") || record.Message == nil {
		return nil
	}
	var blocks []contentBlock
	if json.Unmarshal(record.Message.Content, &blocks) != nil {
		return nil
	}
	ts := parseTime(record.Timestamp)
	var events []event
	for _, block := range blocks {
		switch {
		case record.Type == "assistant" && block.Type == "tool_use":
			var input struct {
				Skill    string `json:"skill"`
				Command  string `json:"command"`
				FilePath string `json:"file_path"`
				// A backgrounded call waits on its own; only a foreground sleep counts.
				Background bool `json:"run_in_background"`
			}
			_ = json.Unmarshal(block.Input, &input)
			switch block.Name {
			case "Skill":
				if input.Skill != "" {
					events = append(events, event{kind: "skill", name: input.Skill, ts: ts})
				}
			case "Bash", "PowerShell":
				var commands []string
				if !input.Background {
					commands = []string{input.Command}
				}
				events = append(events, callEvents(input.Command, commands, ts)...)
			case "Read":
				events = append(events, skillReads(input.FilePath, ts)...)
			}
		case record.Type == "user" && block.Type == "tool_result" && block.IsError:
			events = append(events, outputEvents(resultText(block.Content), ts)...)
		}
	}
	return events
}

// resultText flattens a tool_result content, which is a string or a list of
// text blocks.
func resultText(raw json.RawMessage) string {
	var plain string
	if json.Unmarshal(raw, &plain) == nil {
		return plain
	}
	var blocks []contentBlock
	_ = json.Unmarshal(raw, &blocks)
	var texts []string
	for _, block := range blocks {
		texts = append(texts, block.Text)
	}
	return strings.Join(texts, "\n")
}

var (
	// A SKILL.md path names its plugin either as the directory before a
	// version (plugin caches) or after plugins/ (a source checkout).
	reSkillPath = regexp.MustCompile(`(?:([a-z0-9-]+)/[0-9]+\.[0-9]+\.[0-9]+[^/\s"]*|plugins/([a-z0-9-]+))?/skills/([a-z0-9-]+)/SKILL\.md`)
	// A gate denial opens the tool result, after an optional hook prefix;
	// the same phrase quoted from source code or a search is not a block.
	reGate    = regexp.MustCompile(`^\s*(?:Error: )?(?:[A-Za-z]+(?::[A-Za-z]+)? hook error: )?Load and follow the megapowers ([a-z0-9-]+) skill`)
	reVerdict = regexp.MustCompile(`(?m)^[\s*_>#-]{0,6}(NOT VERIFIED|VERIFIED)\b`)
)

func skillReads(text string, ts time.Time) []event {
	var events []event
	for _, m := range reSkillPath.FindAllStringSubmatch(text, -1) {
		name := m[3]
		if plugin := m[1] + m[2]; plugin != "" {
			name = plugin + ":" + name
		}
		events = append(events, event{kind: "skill", name: name, ts: ts})
	}
	return events
}

// callEvents reads a tool call. text is the whole call body, searched for
// skill paths. commands are the shell commands it runs in the foreground; a
// call counts one sleep event when any of them holds a foreground sleep.
func callEvents(text string, commands []string, ts time.Time) []event {
	events := skillReads(text, ts)
	for _, command := range commands {
		if foregroundSleep(command) {
			events = append(events, event{kind: "sleep", ts: ts})
			break
		}
	}
	return events
}

// reScriptCmd finds the command string in a Codex script call such as
// tools.exec_command({cmd:"...", yield_time_ms:1000}).
var reScriptCmd = regexp.MustCompile(`\bcmd\s*:\s*("(?:[^"\\]|\\.)*")`)

// codexCommands extracts the shell commands from a Codex tool call: JSON
// arguments with a cmd string or a command string or argv, or a script body.
func codexCommands(arguments, input string) []string {
	var commands []string
	var args struct {
		Cmd     string          `json:"cmd"`
		Command json.RawMessage `json:"command"`
	}
	if json.Unmarshal([]byte(arguments), &args) == nil {
		if args.Cmd != "" {
			commands = append(commands, args.Cmd)
		}
		var line string
		var argv []string
		switch {
		case json.Unmarshal(args.Command, &line) == nil && line != "":
			commands = append(commands, line)
		case json.Unmarshal(args.Command, &argv) == nil && len(argv) > 0:
			// ["bash", "-lc", "<script>"] runs the last element.
			last := argv[len(argv)-1]
			if len(argv) >= 3 && strings.HasPrefix(argv[len(argv)-2], "-") && strings.Contains(argv[len(argv)-2], "c") {
				commands = append(commands, last)
			} else {
				commands = append(commands, strings.Join(argv, " "))
			}
		}
	}
	for _, m := range reScriptCmd.FindAllStringSubmatch(input, -1) {
		var command string
		if json.Unmarshal([]byte(m[1]), &command) == nil {
			commands = append(commands, command)
		}
	}
	return commands
}

func outputEvents(text string, ts time.Time) []event {
	if m := reGate.FindStringSubmatch(text); m != nil {
		return []event{{kind: "gate", name: m[1], ts: ts}}
	}
	return nil
}

func claudeMessages(record claudeRecord) ([]message, bool) {
	if (record.Type != "user" && record.Type != "assistant") || record.Message == nil || len(record.Message.Content) == 0 {
		return nil, true
	}
	var blocks []contentBlock
	var plain string
	if err := json.Unmarshal(record.Message.Content, &plain); err == nil {
		blocks = []contentBlock{{Type: "text", Text: plain}}
	} else if err := json.Unmarshal(record.Message.Content, &blocks); err != nil {
		return nil, false
	}
	ts := parseTime(record.Timestamp)
	if record.Type == "assistant" {
		var texts []string
		for _, block := range blocks {
			if block.Type == "text" && strings.TrimSpace(block.Text) != "" {
				texts = append(texts, block.Text)
			}
		}
		if len(texts) == 0 {
			return nil, true
		}
		return []message{{role: "assistant", text: strings.Join(texts, "\n"), ts: ts}}, true
	}
	// A structural signal marks a machine turn even when the harness also
	// flags it meta or system. Anchored text is the fallback for producers,
	// such as an SDK caller, that leave no flag.
	structural := record.IsCompactSummary || record.originIsMachine()
	anchored := false
	var texts []string
	for _, block := range blocks {
		if block.Type == "tool_result" {
			return nil, true
		}
		if block.Type != "text" {
			continue
		}
		raw := strings.TrimSpace(block.Text)
		if strings.HasPrefix(raw, "<task-notification") {
			structural = true
			texts = append(texts, raw)
			continue
		}
		if text, ok := humanText(block.Text); ok {
			anchored = anchored || machineText(text)
			texts = append(texts, text)
		}
	}
	if (record.IsMeta || record.PromptSource == "system") && !structural {
		return nil, true
	}
	if len(texts) == 0 {
		return nil, true
	}
	role := "human"
	switch {
	case record.IsSidechain:
		role = "prompt"
	case structural || anchored:
		role = "machine"
	}
	return []message{{role: role, text: strings.Join(texts, "\n"), ts: ts}}, true
}

type codexMeta struct {
	cwd      string
	subagent bool
	// exec marks a non-interactive "codex exec" run, whose user turns a
	// script wrote.
	exec bool
}

func parseCodex(line []byte, subagent, exec bool) ([]message, []event, *codexMeta, bool) {
	var record struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(line, &record); err != nil {
		return nil, nil, nil, false
	}
	if record.Type == "response_item" {
		var call struct {
			Type      string          `json:"type"`
			Name      string          `json:"name"`
			Arguments string          `json:"arguments"`
			Input     string          `json:"input"`
			Output    json.RawMessage `json:"output"`
		}
		if json.Unmarshal(record.Payload, &call) == nil {
			ts := parseTime(record.Timestamp)
			switch call.Type {
			case "function_call", "custom_tool_call", "local_shell_call":
				// A patch that edits a SKILL.md is authoring, not a load.
				if call.Name != "apply_patch" {
					return nil, callEvents(call.Arguments+"\n"+call.Input, codexCommands(call.Arguments, call.Input), ts), nil, true
				}
				return nil, nil, nil, true
			case "function_call_output", "custom_tool_call_output":
				var text string
				if json.Unmarshal(call.Output, &text) != nil {
					text = string(call.Output)
				}
				return nil, outputEvents(text, ts), nil, true
			}
		}
	}
	switch record.Type {
	case "session_meta":
		var payload struct {
			Cwd    string          `json:"cwd"`
			Source json.RawMessage `json:"source"`
		}
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			return nil, nil, nil, false
		}
		var source map[string]json.RawMessage
		_ = json.Unmarshal(payload.Source, &source)
		_, isSubagent := source["subagent"]
		var kind string
		_ = json.Unmarshal(payload.Source, &kind)
		return nil, nil, &codexMeta{cwd: payload.Cwd, subagent: isSubagent, exec: kind == "exec"}, true
	case "response_item":
		var payload struct {
			Type    string         `json:"type"`
			Role    string         `json:"role"`
			Content []contentBlock `json:"content"`
		}
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			return nil, nil, nil, false
		}
		if payload.Type != "message" || (payload.Role != "user" && payload.Role != "assistant") {
			return nil, nil, nil, true
		}
		var texts []string
		anchored := false
		for _, block := range payload.Content {
			switch {
			case payload.Role == "assistant" && block.Type == "output_text" && strings.TrimSpace(block.Text) != "":
				texts = append(texts, block.Text)
			case payload.Role == "user" && block.Type == "input_text":
				if text, ok := humanText(block.Text); ok {
					anchored = anchored || machineText(text)
					texts = append(texts, text)
				}
			}
		}
		if len(texts) == 0 {
			return nil, nil, nil, true
		}
		role := payload.Role
		if role == "user" {
			role = "human"
			switch {
			case subagent:
				role = "prompt"
			case exec || anchored:
				role = "machine"
			}
		}
		return []message{{role: role, text: strings.Join(texts, "\n"), ts: parseTime(record.Timestamp)}}, nil, nil, true
	}
	return nil, nil, nil, true
}

func classify(text string) string {
	normalized := strings.ReplaceAll(text, "’", "'")
	for _, rule := range classRules {
		if rule.pattern.MatchString(normalized) {
			return rule.name
		}
	}
	return "other"
}

func redact(text string) string {
	text = reUserinfo.ReplaceAllString(text, "${1}")
	for _, pattern := range reHomeValues {
		text = pattern.ReplaceAllString(text, "~${1}")
	}
	text = reHomePath.ReplaceAllString(text, "~")
	text = reBearer.ReplaceAllString(text, "Bearer <secret>")
	for _, pattern := range reTokens {
		text = pattern.ReplaceAllString(text, "<secret>")
	}
	text = reEmail.ReplaceAllString(text, "<email>")
	// Any 32+ run of base64 or hex characters is a secret candidate. A run is
	// kept as a path only when it has several "/" and every segment looks like
	// a lowercase path name; a token such as AAAA/BBBB/CCCC is redacted whole.
	return reLongB64.ReplaceAllStringFunc(text, func(run string) string {
		if strings.Count(run, "/") < 2 {
			return "<secret>"
		}
		for _, segment := range strings.Split(run, "/") {
			if segment != "" && !rePathSegment.MatchString(segment) {
				return "<secret>"
			}
		}
		return run
	})
}

var rePathSegment = regexp.MustCompile(`^[a-z0-9._-]{1,31}$`)

// stripTagged removes every lowercase-tag block that has a matching close
// tag, such as an embedded system reminder or slash-command echo.
func stripTagged(text string) string {
	var b strings.Builder
	for {
		loc := openTag.FindStringSubmatchIndex(text)
		if loc == nil {
			b.WriteString(text)
			return strings.TrimSpace(b.String())
		}
		end := matchingClose(text[loc[1]:], text[loc[2]:loc[3]])
		if end < 0 {
			b.WriteString(text[:loc[1]])
			text = text[loc[1]:]
			continue
		}
		b.WriteString(text[:loc[0]])
		text = text[loc[1]+end:]
	}
}

// matchingClose returns the offset just past the close tag that balances an
// already-open tag of this name, counting nested tags of the same name, or -1.
func matchingClose(text, name string) int {
	opening := regexp.MustCompile(`<` + regexp.QuoteMeta(name) + `(?:\s[^<>]*)?>`)
	closing := "</" + name + ">"
	depth, offset := 1, 0
	for {
		next := strings.Index(text[offset:], closing)
		if next < 0 {
			return -1
		}
		depth += len(opening.FindAllStringIndex(text[offset:offset+next], -1)) - 1
		offset += next + len(closing)
		if depth == 0 {
			return offset
		}
	}
}

// excerpt returns a single-line, redacted prefix of at most max runes.
func excerpt(text string, max int) string {
	runes := []rune(text)
	if len(runes) > 4096 {
		runes = runes[:4096]
	}
	single := []rune(redact(strings.Join(strings.Fields(string(runes)), " ")))
	if len(single) <= max {
		return string(single)
	}
	return string(single[:max-1]) + "…"
}

func formatTime(ts time.Time) string {
	if ts.IsZero() {
		return "unknown"
	}
	return ts.Format(time.RFC3339)
}

type turn struct {
	example
	class string
	ts    time.Time
}

func corrections(opt options, sessions []session, stats scanStats) correctionsReport {
	report := correctionsReport{
		Note:            "keyword classes are candidates; read each turn before treating it as a finding",
		SessionsScanned: len(sessions),
		SkippedLines:    stats.skippedLines,
		SkippedFiles:    stats.skippedFiles,
		UnreadableFiles: stats.unreadableFiles,
		Classes:         []classReport{},
		OneOffs:         []classReport{},
	}
	var turns []turn
	for _, s := range sessions {
		lastAgent := ""
		for _, msg := range s.msgs {
			switch msg.role {
			case "assistant":
				lastAgent = msg.text
			case "human":
				turns = append(turns, turn{
					example: example{Harness: s.harness, Session: s.base, Timestamp: formatTime(msg.ts), User: excerpt(msg.text, correctionExcerpt), Agent: excerpt(lastAgent, correctionExcerpt)},
					class:   classify(msg.text),
					ts:      msg.ts,
				})
				lastAgent = ""
			case "machine":
				report.MachineTurns++
			}
		}
	}
	sort.SliceStable(turns, func(i, j int) bool { return turns[i].ts.Before(turns[j].ts) })
	report.HumanTurns = len(turns)
	byClass := map[string]*classReport{}
	for _, t := range turns {
		if t.class == "other" {
			report.OtherTurns++
			continue
		}
		report.ClassifiedTurns++
		c := byClass[t.class]
		if c == nil {
			c = &classReport{Name: t.class, Examples: []example{}}
			byClass[t.class] = c
		}
		c.Count++
		if len(c.Examples) < opt.limit {
			c.Examples = append(c.Examples, t.example)
		}
	}
	for _, rule := range classRules {
		c := byClass[rule.name]
		switch {
		case c == nil:
		case c.Count >= 2:
			report.Classes = append(report.Classes, *c)
		default:
			report.OneOffs = append(report.OneOffs, *c)
		}
	}
	sort.SliceStable(report.Classes, func(i, j int) bool { return report.Classes[i].Count > report.Classes[j].Count })
	return report
}

func recall(opt options, sessions []session, stats scanStats) recallReport {
	report := recallReport{Topic: redact(strings.Join(opt.topic, " ")), SessionsScanned: len(sessions), SkippedLines: stats.skippedLines, SkippedFiles: stats.skippedFiles, UnreadableFiles: stats.unreadableFiles, Sessions: []recallItem{}}
	for _, s := range sessions {
		matched := false
		for _, msg := range s.msgs {
			if containsAll(strings.ToLower(msg.text), opt.topic) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		item := recallItem{Harness: s.harness, Session: s.base}
		first, last := s.msgs[0].ts, s.msgs[0].ts
		firstHuman, firstPrompt, firstMachine, lastReply := "", "", "", ""
		for _, msg := range s.msgs {
			if first.IsZero() || (!msg.ts.IsZero() && msg.ts.Before(first)) {
				first = msg.ts
			}
			if msg.ts.After(last) {
				last = msg.ts
			}
			switch {
			case msg.role == "human" && firstHuman == "":
				firstHuman = msg.text
			case msg.role == "prompt" && firstPrompt == "":
				firstPrompt = msg.text
			case msg.role == "machine" && firstMachine == "":
				firstMachine = msg.text
			case msg.role == "assistant":
				lastReply = msg.text
			}
		}
		// A scripted run has no typed turn; its first machine prompt still
		// says what the session was for.
		if firstHuman == "" {
			firstHuman = firstPrompt
		}
		if firstHuman == "" {
			firstHuman = firstMachine
		}
		item.FirstTimestamp, item.LastTimestamp, item.last = formatTime(first), formatTime(last), last
		item.FirstPrompt, item.LastReply = excerpt(firstHuman, recallExcerpt), excerpt(lastReply, recallExcerpt)
		report.Sessions = append(report.Sessions, item)
	}
	sort.SliceStable(report.Sessions, func(i, j int) bool {
		if !report.Sessions[i].last.Equal(report.Sessions[j].last) {
			return report.Sessions[i].last.After(report.Sessions[j].last)
		}
		return report.Sessions[i].Session < report.Sessions[j].Session
	})
	if len(report.Sessions) > opt.limit {
		report.Sessions = report.Sessions[:opt.limit]
	}
	return report
}

func containsAll(text string, words []string) bool {
	for _, word := range words {
		if !strings.Contains(text, word) {
			return false
		}
	}
	return true
}

func printClass(w io.Writer, c classReport) {
	fmt.Fprintf(w, "%s (%d)\n", c.Name, c.Count)
	for _, e := range c.Examples {
		fmt.Fprintf(w, "  - %s %s %s\n    user:  %s\n    agent: %s\n", e.Harness, e.Session, e.Timestamp, e.User, e.Agent)
	}
}

func printCorrections(w io.Writer, r correctionsReport) {
	fmt.Fprintf(w, "session-mine corrections: %s\n\n", r.Note)
	if len(r.Classes) == 0 {
		fmt.Fprintln(w, "no class occurred twice")
	}
	for _, c := range r.Classes {
		printClass(w, c)
	}
	if len(r.OneOffs) > 0 {
		fmt.Fprintln(w, "\none-offs (seen once; a class counts after two occurrences):")
		for _, c := range r.OneOffs {
			printClass(w, c)
		}
	}
	fmt.Fprintf(w, "\nsummary: sessions scanned %d, human turns %d, classified turns %d, machine turns %d, skipped lines %d", r.SessionsScanned, r.HumanTurns, r.ClassifiedTurns, r.MachineTurns, r.SkippedLines)
	if r.SkippedFiles > 0 {
		fmt.Fprintf(w, ", skipped files %d", r.SkippedFiles)
	}
	if r.UnreadableFiles > 0 {
		fmt.Fprintf(w, ", unreadable files %d", r.UnreadableFiles)
	}
	fmt.Fprintln(w)
}

func printRecall(w io.Writer, r recallReport) {
	fmt.Fprintf(w, "session-mine recall %q: %d matching session(s), most recent first\n", r.Topic, len(r.Sessions))
	for _, item := range r.Sessions {
		fmt.Fprintf(w, "\n%s %s %s .. %s\n  first prompt: %s\n  last reply:   %s\n", item.Harness, item.Session, item.FirstTimestamp, item.LastTimestamp, item.FirstPrompt, item.LastReply)
	}
	fmt.Fprintf(w, "\nsummary: sessions scanned %d, skipped lines %d", r.SessionsScanned, r.SkippedLines)
	if r.SkippedFiles > 0 {
		fmt.Fprintf(w, ", skipped files %d", r.SkippedFiles)
	}
	if r.UnreadableFiles > 0 {
		fmt.Fprintf(w, ", unreadable files %d", r.UnreadableFiles)
	}
	fmt.Fprintln(w)
}

type skillScore struct {
	Name                 string `json:"name"`
	Loads                int    `json:"loads"`
	Sessions             int    `json:"sessions"`
	FollowedByCorrection int    `json:"followed_by_correction"`
}

type runScore struct {
	ID                   string `json:"id"`
	Records              int    `json:"records"`
	Oracle               int    `json:"oracle"`
	Continuations        int    `json:"continuations"`
	Blocked              int    `json:"blocked"`
	HoldoutRegistrations int    `json:"holdout_registrations"`
	Rebaselines          int    `json:"rebaselines"`
	HighRiskDecisions    int    `json:"high_risk_decisions"`
}

type workspaceScore struct {
	Workspace   string `json:"workspace"`
	Sessions    int    `json:"sessions"`
	HumanTurns  int    `json:"human_turns"`
	Corrections int    `json:"corrections"`
	SkillLoads  int    `json:"skill_loads"`
	GateBlocks  int    `json:"gate_blocks"`
	Verified    int    `json:"verified"`
	NotVerified int    `json:"not_verified"`
	SleepCalls  int    `json:"sleep_calls"`
}

type scorecardReport struct {
	Note            string         `json:"note"`
	Scope           string         `json:"scope"`
	Since           string         `json:"since,omitempty"`
	SessionsScanned int            `json:"sessions_scanned"`
	HumanTurns      int            `json:"human_turns"`
	MachineTurns    int            `json:"machine_turns"`
	Corrections     map[string]int `json:"corrections"`
	Skills          []skillScore   `json:"skills"`
	Gates           map[string]int `json:"gates"`
	Reports         struct {
		Verified    int `json:"verified"`
		NotVerified int `json:"not_verified"`
	} `json:"reports"`
	SleepCalls      int              `json:"sleep_calls"`
	Runs            []runScore       `json:"runs"`
	ByWorkspace     []workspaceScore `json:"by_workspace,omitempty"`
	SkippedLines    int              `json:"skipped_lines"`
	SkippedFiles    int              `json:"skipped_files"`
	UnreadableFiles int              `json:"unreadable_files"`
}

// scorecard summarizes outcome signals. Counts are candidates for a
// retrospective, not findings: a correction after a skill load does not show
// the skill caused it.
func scorecard(opt options, sessions []session, stats scanStats) scorecardReport {
	report := scorecardReport{
		Note:            "counts are candidates; read the sessions behind a number before acting on it",
		Scope:           opt.workspace,
		SessionsScanned: len(sessions),
		Corrections:     map[string]int{},
		Skills:          []skillScore{},
		Gates:           map[string]int{},
		Runs:            []runScore{},
		SkippedLines:    stats.skippedLines,
		SkippedFiles:    stats.skippedFiles,
		UnreadableFiles: stats.unreadableFiles,
	}
	if opt.hasSince {
		report.Since = opt.since.Format("2006-01-02")
	}
	if opt.all {
		report.Scope = "all workspaces"
	}
	if opt.sanitize {
		report.Scope = sanitizedScope(opt)
	}
	skills := map[string]*skillScore{}
	workspaces := map[string]*workspaceScore{}
	for _, s := range sessions {
		key := s.cwd
		if key == "" {
			key = "unknown"
		}
		if opt.sanitize {
			key = hashKey("ws-", key)
		}
		ws := workspaces[key]
		if ws == nil {
			ws = &workspaceScore{Workspace: key}
			workspaces[key] = ws
		}
		ws.Sessions++
		inSession := map[string]bool{}
		for _, ev := range s.events {
			switch ev.kind {
			case "skill":
				name := skillName(ev.name, opt.sanitize)
				score := skills[name]
				if score == nil {
					score = &skillScore{Name: name}
					skills[name] = score
				}
				score.Loads++
				ws.SkillLoads++
				if !inSession[name] {
					inSession[name] = true
					score.Sessions++
				}
			case "gate":
				report.Gates[ev.name]++
				ws.GateBlocks++
			case "sleep":
				report.SleepCalls++
				ws.SleepCalls++
			}
		}
		previousHuman := -1
		for i, msg := range s.msgs {
			switch msg.role {
			case "assistant":
				if m := reVerdict.FindStringSubmatch(msg.text); m != nil {
					if m[1] == "NOT VERIFIED" {
						report.Reports.NotVerified++
						ws.NotVerified++
					} else {
						report.Reports.Verified++
						ws.Verified++
					}
				}
			case "machine":
				report.MachineTurns++
			case "human":
				report.HumanTurns++
				ws.HumanTurns++
				if class := classify(msg.text); class != "other" {
					report.Corrections[class]++
					ws.Corrections++
					// Tie the correction to each skill loaded since the
					// previous human turn, once per skill.
					tied := map[string]bool{}
					for _, ev := range s.events {
						if ev.kind == "skill" && ev.after > previousHuman && ev.after <= i {
							name := skillName(ev.name, opt.sanitize)
							if !tied[name] {
								tied[name] = true
								skills[name].FollowedByCorrection++
							}
						}
					}
				}
				previousHuman = i
			}
		}
	}
	for _, score := range skills {
		report.Skills = append(report.Skills, *score)
	}
	sort.Slice(report.Skills, func(i, j int) bool {
		if report.Skills[i].Loads != report.Skills[j].Loads {
			return report.Skills[i].Loads > report.Skills[j].Loads
		}
		return report.Skills[i].Name < report.Skills[j].Name
	})
	if opt.all {
		for _, ws := range workspaces {
			report.ByWorkspace = append(report.ByWorkspace, *ws)
		}
		sort.Slice(report.ByWorkspace, func(i, j int) bool { return report.ByWorkspace[i].Workspace < report.ByWorkspace[j].Workspace })
	} else {
		report.Runs = runScores(opt.workspace, opt.sanitize)
	}
	return report
}

func hashKey(prefix, value string) string {
	sum := sha256.Sum256([]byte(value))
	return prefix + hex.EncodeToString(sum[:4])
}

func sanitizedScope(opt options) string {
	if opt.all {
		return "all workspaces (sanitized)"
	}
	return hashKey("ws-", opt.workspace) + " (sanitized)"
}

// reMegapowersSkill matches a well-formed megapowers skill name. A failed or
// malformed request can carry arguments after the name, so a prefix check is
// not enough.
var reMegapowersSkill = regexp.MustCompile(`^megapowers:[a-z0-9-]+$`)

// skillName keeps well-formed megapowers skill names, which are public, and
// folds every other name into "other" when sanitizing, since a private
// plugin's skill names or a malformed request's arguments can identify a
// client or project.
func skillName(name string, sanitize bool) string {
	if sanitize && !reMegapowersSkill.MatchString(name) {
		return "other"
	}
	return name
}

// runScores reads autonomous-run journals in the workspace checkout.
func runScores(workspace string, sanitize bool) []runScore {
	paths, _ := filepath.Glob(filepath.Join(workspace, ".megapowers", "run", "*", "journal.jsonl"))
	sort.Strings(paths)
	runs := []runScore{}
	for i, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		score := runScore{ID: filepath.Base(filepath.Dir(path))}
		if sanitize {
			score.ID = fmt.Sprintf("run-%d", i+1)
		}
		for _, line := range strings.Split(string(data), "\n") {
			var rec struct {
				Event    string `json:"event"`
				Decision string `json:"decision"`
				Risk     string `json:"risk"`
			}
			if strings.TrimSpace(line) == "" || json.Unmarshal([]byte(line), &rec) != nil {
				continue
			}
			score.Records++
			switch rec.Event {
			case "oracle":
				score.Oracle++
			case "continuation":
				score.Continuations++
			case "blocked":
				score.Blocked++
			case "holdout":
				score.HoldoutRegistrations++
				if rec.Decision != "" {
					score.Rebaselines++
				}
			}
			if rec.Risk == "high" {
				score.HighRiskDecisions++
			}
		}
		runs = append(runs, score)
	}
	return runs
}

func joinCounts(counts map[string]int) string {
	if len(counts) == 0 {
		return "none"
	}
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = fmt.Sprintf("%s %d", key, counts[key])
	}
	return strings.Join(parts, ", ")
}

func printScorecard(w io.Writer, r scorecardReport) {
	fmt.Fprintf(w, "session-mine scorecard: %s; %s\n", r.Scope, r.Note)
	if r.Since != "" {
		fmt.Fprintf(w, "since: %s\n", r.Since)
	}
	fmt.Fprintf(w, "sessions: %d, human turns: %d, machine turns: %d\n", r.SessionsScanned, r.HumanTurns, r.MachineTurns)
	fmt.Fprintf(w, "corrections: %s\n", joinCounts(r.Corrections))
	fmt.Fprintln(w, "skills (loads / sessions / followed by correction):")
	if len(r.Skills) == 0 {
		fmt.Fprintln(w, "- none")
	}
	for _, s := range r.Skills {
		fmt.Fprintf(w, "- %s: %d / %d / %d\n", s.Name, s.Loads, s.Sessions, s.FollowedByCorrection)
	}
	fmt.Fprintf(w, "gates: %s\n", joinCounts(r.Gates))
	fmt.Fprintf(w, "reports: %d verified, %d not verified\n", r.Reports.Verified, r.Reports.NotVerified)
	fmt.Fprintf(w, "sleep calls: %d\n", r.SleepCalls)
	for _, run := range r.Runs {
		fmt.Fprintf(w, "run %s: %d records, %d oracle, %d continuations, %d blocked, %d holdout registrations, %d rebaselines, %d high-risk decisions\n",
			run.ID, run.Records, run.Oracle, run.Continuations, run.Blocked, run.HoldoutRegistrations, run.Rebaselines, run.HighRiskDecisions)
	}
	if len(r.ByWorkspace) > 0 {
		fmt.Fprintln(w, "by workspace (sessions / human turns / corrections / skill loads / gate blocks / verified / not verified / sleep calls):")
		for _, ws := range r.ByWorkspace {
			fmt.Fprintf(w, "- %s: %d / %d / %d / %d / %d / %d / %d / %d\n", ws.Workspace, ws.Sessions, ws.HumanTurns, ws.Corrections, ws.SkillLoads, ws.GateBlocks, ws.Verified, ws.NotVerified, ws.SleepCalls)
		}
	}
	fmt.Fprintf(w, "summary: skipped lines %d, skipped files %d, unreadable files %d\n", r.SkippedLines, r.SkippedFiles, r.UnreadableFiles)
}

// ---------------------------------------------------------------------------
// Foreground sleep detection. A small shell reader, adapted from a
// PreToolUse hook that blocks foreground sleeps, tells a sleep that blocks the call (outside
// every loop, not backgrounded, longer than 5 s) from the word "sleep" in a
// string, heredoc, comment, or argument. It is not a full shell parser, and
// anything it cannot read is not counted.

const (
	maxDepth        = 8
	maxShortSeconds = 5.0 // a sleep this short is a pause, not a wait
)

type token struct {
	op     string   // operator token: ; \n && || | & ( ) ;;  (empty for words)
	text   string   // decoded word text
	quoted bool     // any part of the word was quoted or escaped
	subs   []string // command substitution bodies found in the word
}

type heredoc struct {
	delim string
	strip bool
}

func lex(s string) []token {
	var toks []token
	var pending []heredoc
	n := len(s)
	i := 0
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '\\' && i+1 < n && s[i+1] == '\n':
			i += 2
		case c == '\n':
			toks = append(toks, token{op: "\n"})
			i++
			for _, h := range pending {
				i = skipHeredoc(s, i, h)
			}
			pending = nil
		case c == '#':
			for i < n && s[i] != '\n' {
				i++
			}
		case c == ';':
			if i+1 < n && s[i+1] == ';' {
				toks = append(toks, token{op: ";;"})
				i += 2
			} else {
				toks = append(toks, token{op: ";"})
				i++
			}
		case c == '|':
			switch {
			case i+1 < n && s[i+1] == '|':
				toks = append(toks, token{op: "||"})
				i += 2
			case i+1 < n && s[i+1] == '&':
				toks = append(toks, token{op: "|"})
				i += 2
			default:
				toks = append(toks, token{op: "|"})
				i++
			}
		case c == '&' && !(i+1 < n && s[i+1] == '>'):
			if i+1 < n && s[i+1] == '&' {
				toks = append(toks, token{op: "&&"})
				i += 2
			} else {
				toks = append(toks, token{op: "&"})
				i++
			}
		case c == '(' || c == ')':
			toks = append(toks, token{op: string(c)})
			i++
		default:
			var w token
			i = lexWord(s, i, &w, &pending)
			toks = append(toks, w)
		}
	}
	return toks
}

// lexWord reads one word starting at i and returns the index after it.
func lexWord(s string, i int, w *token, pending *[]heredoc) int {
	n := len(s)
	var b strings.Builder
	for i < n {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == ';' || c == '(' || c == ')':
			// `<(` and `>(` are process substitutions, handled below.
			w.text = b.String()
			return i
		case c == '|':
			if i > 0 && s[i-1] == '>' { // >| clobber redirect
				b.WriteByte(c)
				i++
				continue
			}
			w.text = b.String()
			return i
		case c == '&':
			if i > 0 && (s[i-1] == '>' || s[i-1] == '<') || (i+1 < n && s[i+1] == '>' && b.Len() == 0) {
				b.WriteByte(c)
				i++
				continue
			}
			w.text = b.String()
			return i
		case c == '\\':
			w.quoted = true
			if i+1 < n && s[i+1] != '\n' {
				b.WriteByte(s[i+1])
			}
			i += 2
		case c == '\'':
			w.quoted = true
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				b.WriteString(s[i+1:])
				i = n
			} else {
				b.WriteString(s[i+1 : i+1+j])
				i += j + 2
			}
		case c == '"':
			w.quoted = true
			i = lexDouble(s, i+1, &b, w)
		case c == '`':
			j := i + 1
			for j < n && s[j] != '`' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			if j > n {
				j = n
			}
			w.subs = append(w.subs, s[i+1:min(j, n)])
			b.WriteByte(0)
			i = j + 1
		case c == '$' && i+1 < n && s[i+1] == '(':
			i = lexSubst(s, i+2, &b, w, i+2 < n && s[i+2] == '(')
		case c == '$' && i+1 < n && s[i+1] == '{':
			j := i + 2
			depth := 1
			for j < n && depth > 0 {
				switch s[j] {
				case '{':
					depth++
				case '}':
					depth--
				}
				j++
			}
			b.WriteString(s[i:min(j, n)])
			i = j
		case (c == '<' || c == '>') && i+1 < n && s[i+1] == '(':
			i = lexSubst(s, i+2, &b, w, false)
		case c == '<' && i+1 < n && s[i+1] == '<' && !(i+2 < n && s[i+2] == '<'):
			b.WriteString("<<")
			i += 2
			strip := false
			if i < n && s[i] == '-' {
				strip = true
				i++
			}
			for i < n && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			var d token
			i = lexWord(s, i, &d, pending)
			*pending = append(*pending, heredoc{delim: d.text, strip: strip})
		default:
			b.WriteByte(c)
			i++
		}
	}
	w.text = b.String()
	return i
}

// lexDouble reads a double-quoted string body starting after the opening
// quote and returns the index after the closing quote.
func lexDouble(s string, i int, b *strings.Builder, w *token) int {
	n := len(s)
	for i < n {
		c := s[i]
		switch {
		case c == '"':
			return i + 1
		case c == '\\' && i+1 < n:
			if strings.IndexByte("\"\\$`\n", s[i+1]) >= 0 {
				if s[i+1] != '\n' {
					b.WriteByte(s[i+1])
				}
			} else {
				b.WriteByte(c)
				b.WriteByte(s[i+1])
			}
			i += 2
		case c == '$' && i+1 < n && s[i+1] == '(':
			i = lexSubst(s, i+2, b, w, i+2 < n && s[i+2] == '(')
		case c == '`':
			j := i + 1
			for j < n && s[j] != '`' {
				if s[j] == '\\' {
					j++
				}
				j++
			}
			w.subs = append(w.subs, s[i+1:min(j, n)])
			b.WriteByte(0)
			i = j + 1
		default:
			b.WriteByte(c)
			i++
		}
	}
	return n
}

// lexSubst reads a $( ... ) body starting after "$(" and records it. An
// arithmetic $(( ... )) is consumed and ignored.
func lexSubst(s string, i int, b *strings.Builder, w *token, arith bool) int {
	end := matchParen(s, i)
	body := s[i:min(end, len(s))]
	if !arith {
		w.subs = append(w.subs, body)
	}
	b.WriteByte(0)
	return end + 1
}

// matchParen returns the index of the ")" closing a "(" whose body starts at
// i, or len(s) when unbalanced.
func matchParen(s string, i int) int {
	depth := 1
	for i < len(s) {
		switch s[i] {
		case '\\':
			i++
		case '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return len(s)
			}
			i += j + 1
		case '"':
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' {
					i++
				}
				i++
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		}
		i++
	}
	return len(s)
}

func skipHeredoc(s string, i int, h heredoc) int {
	n := len(s)
	for i < n {
		end := strings.IndexByte(s[i:], '\n')
		line := s[i:]
		next := n
		if end >= 0 {
			line = s[i : i+end]
			next = i + end + 1
		}
		if h.strip {
			line = strings.TrimLeft(line, "\t")
		}
		i = next
		if line == h.delim {
			return i
		}
	}
	return n
}

// ---------------------------------------------------------------------------

type analyzer struct {
	toks    []token
	loops   int
	base    bool // started inside a loop (recursion into a payload)
	depth   int
	frames  []*frame
	flagged []float64
}

type frame struct {
	pending   []float64 // current and-or list
	committed []float64 // finished lists of this group
}

// analyze returns the durations (-1 = unknown) of every foreground sleep that
// runs outside a loop and is not backgrounded.
func analyze(command string, inLoop bool, depth int) []float64 {
	if depth > maxDepth {
		return nil
	}
	a := &analyzer{toks: lex(command), base: inLoop, depth: depth}
	a.frames = []*frame{{}}
	var words []token
	for i := 0; i < len(a.toks); i++ {
		t := a.toks[i]
		if t.op == "" {
			words = append(words, t)
			continue
		}
		next := ""
		if i+1 < len(a.toks) {
			next = a.toks[i+1].op
		}
		switch t.op {
		case "(":
			a.simple(words, "")
			words = nil
			a.frames = append(a.frames, &frame{})
		case ")":
			a.simple(words, "")
			words = nil
			a.closeGroup(next)
		default:
			a.simple(words, t.op)
			words = nil
			switch t.op {
			case ";", "\n", ";;":
				a.commit()
			case "&":
				a.top().pending = nil
			}
		}
	}
	a.simple(words, "")
	for len(a.frames) > 1 {
		a.closeGroup("")
	}
	a.commit()
	return a.top().committed
}

func (a *analyzer) top() *frame { return a.frames[len(a.frames)-1] }

func (a *analyzer) commit() {
	f := a.top()
	f.committed = append(f.committed, f.pending...)
	f.pending = nil
}

// closeGroup ends a ( ) or { } group. A group followed by "&" runs in the
// background, so its sleeps do not block the tool call.
func (a *analyzer) closeGroup(nextOp string) {
	if len(a.frames) < 2 {
		return
	}
	a.commit()
	g := a.top()
	a.frames = a.frames[:len(a.frames)-1]
	if nextOp != "&" {
		p := a.top()
		p.pending = append(p.pending, g.committed...)
	}
}

func (a *analyzer) inLoop() bool { return a.base || a.loops > 0 }

var assignRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\[[^\]]*\])?\+?=`)

var wrapperValueOpts = map[string]map[string]bool{
	"sudo":    {"-u": true, "-g": true, "-h": true, "-p": true, "-C": true, "-D": true, "-r": true, "-t": true, "-T": true, "-U": true},
	"doas":    {"-u": true, "-C": true},
	"timeout": {"-k": true, "-s": true, "--kill-after": true, "--signal": true},
	"nice":    {"-n": true},
	"ionice":  {"-c": true, "-n": true, "-p": true},
	"env":     {"-u": true, "-C": true, "-S": true},
	"stdbuf":  {"-i": true, "-o": true, "-e": true},
}

var wrappers = map[string]bool{
	"sudo": true, "doas": true, "nohup": true, "command": true, "exec": true, "nice": true,
	"ionice": true, "stdbuf": true, "setsid": true, "time": true, "env": true, "timeout": true,
	"builtin": true,
}

var shells = map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true}

// simple analyses one simple command; nextOp is the operator that ended it.
func (a *analyzer) simple(words []token, nextOp string) {
	i := 0
strip:
	for i < len(words) {
		w := words[i]
		if w.quoted {
			break
		}
		switch w.text {
		case "while", "until":
			a.loops++
		case "for", "select":
			a.loops++
			a.subs(words)
			return
		case "case":
			a.subs(words)
			return
		case "do", "then", "else", "elif", "if", "!", "fi", "esac":
		case "done":
			if a.loops > 0 {
				a.loops--
			}
		case "{":
			a.frames = append(a.frames, &frame{})
		case "}":
			a.closeGroup(nextOp)
		default:
			break strip
		}
		i++
	}
	a.subs(words[i:])
	for i < len(words) && !words[i].quoted && assignRE.MatchString(words[i].text) {
		i++
	}
	for i < len(words) {
		w := words[i]
		name := path.Base(w.text)
		switch {
		case wrappers[name]:
			i = skipWrapper(words, i+1, name)
		case name == "sleep":
			a.sleep(words[i+1:])
			return
		case shells[name]:
			a.shellPayload(words[i+1:])
			return
		case name == "eval":
			var parts []string
			for _, x := range words[i+1:] {
				parts = append(parts, x.text)
			}
			a.recurse(strings.Join(parts, " "))
			return
		default:
			return
		}
	}
}

func (a *analyzer) subs(words []token) {
	for _, w := range words {
		for _, s := range w.subs {
			a.recurse(s)
		}
	}
}

func (a *analyzer) recurse(payload string) {
	got := analyze(payload, a.inLoop(), a.depth+1)
	f := a.top()
	f.pending = append(f.pending, got...)
}

// skipWrapper returns the index of the command after a wrapper's options.
func skipWrapper(words []token, i int, name string) int {
	needDuration := name == "timeout"
	for i < len(words) {
		t := words[i].text
		switch {
		case !words[i].quoted && assignRE.MatchString(t) && name == "env":
			i++
		case strings.HasPrefix(t, "-") && t != "-" && !words[i].quoted:
			if wrapperValueOpts[name][t] {
				i++
			}
			i++
		case needDuration:
			needDuration = false
			i++
		default:
			return i
		}
	}
	return i
}

func (a *analyzer) shellPayload(args []token) {
	for i := 0; i < len(args); i++ {
		t := args[i].text
		if strings.HasPrefix(t, "--") || !strings.HasPrefix(t, "-") {
			if !strings.HasPrefix(t, "-") {
				return // script path: not analysable
			}
			continue
		}
		if strings.ContainsRune(t[1:], 'c') && i+1 < len(args) {
			a.recurse(args[i+1].text)
			return
		}
	}
}

var durationRE = regexp.MustCompile(`^([0-9]+(?:\.[0-9]*)?|\.[0-9]+)([smhd]?)$`)

// sleep records a foreground sleep unless it is short or inside a loop.
func (a *analyzer) sleep(args []token) {
	if len(args) == 0 || a.inLoop() {
		return
	}
	total := 0.0
	for _, w := range args {
		m := durationRE.FindStringSubmatch(w.text)
		if m == nil {
			if strings.HasPrefix(w.text, "-") || w.text == "infinity" || w.text == "inf" {
				return // an option or an endless sleep: not a "sleep then check"
			}
			total = -1 // $VAR or computed: length unknown, treat as a wait
			break
		}
		v, _ := strconv.ParseFloat(m[1], 64)
		switch m[2] {
		case "m":
			v *= 60
		case "h":
			v *= 3600
		case "d":
			v *= 86400
		}
		total += v
	}
	if total >= 0 && total <= maxShortSeconds {
		return
	}
	f := a.top()
	f.pending = append(f.pending, total)
}

// foregroundSleep reports whether a shell command holds a sleep that blocks
// the call: outside every loop, not backgrounded, and longer than 5 s.
func foregroundSleep(command string) bool {
	return len(analyze(command, false, 0)) > 0
}
