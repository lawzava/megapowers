// session-mine reads local Claude Code and Codex transcripts, read-only, and
// reports where the human corrected the agent (corrections) or which earlier
// sessions discussed a topic (recall). Every excerpt is redacted.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const usageText = `usage: go run session-mine.go [corrections|recall] (--workspace DIR | --all-workspaces)
  [--since YYYY-MM-DD] [--harness claude|codex|both] [--claude-root DIR]
  [--codex-root DIR] [--format text|json] [--limit N] [--topic "WORDS" (recall only)]`

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
	codexRoot  string
	format     string
	limit      int
	topic      []string
}

// A message is one kept text record. Role is "human" (a person typed it),
// "prompt" (a lead agent prompted a subagent), or "assistant".
type message struct {
	role string
	text string
	ts   time.Time
	cwd  string
}

type session struct {
	harness string
	base    string
	msgs    []message
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
	if opt.mode == "recall" {
		report = recall(opt, sessions, stats)
	} else {
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
	}
	return 0
}

func parseOptions(args []string) (options, error) {
	opt := options{mode: "corrections", harness: "both", format: "text", limit: -1}
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		opt.mode = args[0]
		args = args[1:]
	}
	if opt.mode != "corrections" && opt.mode != "recall" {
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
		if name == "--all-workspaces" {
			if hasValue {
				return opt, fmt.Errorf("--all-workspaces takes no value")
			}
			opt.all = true
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
			opt.codexRoot = value
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
	if opt.mode == "corrections" && topic != "" {
		return opt, fmt.Errorf("--topic applies only to recall")
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
	if opt.codexRoot == "" {
		if dir := os.Getenv("CODEX_HOME"); dir != "" {
			opt.codexRoot = filepath.Join(dir, "sessions")
		} else if home != "" {
			opt.codexRoot = filepath.Join(home, ".codex", "sessions")
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
	if opt.harness != "claude" && opt.codexRoot != "" {
		if root, err := filepath.EvalSymlinks(opt.codexRoot); err == nil {
			paths, _ := filepath.Glob(filepath.Join(root, "[0-9][0-9][0-9][0-9]", "[0-9][0-9]", "[0-9][0-9]", "rollout-*.jsonl"))
			for _, path := range paths {
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
	codexScoped, codexSubagent := false, false
	origin := ""
	for {
		line, readErr := reader.ReadBytes('\n')
		line = bytes.TrimSpace(line)
		if len(line) > 0 {
			var msgs []message
			var ok bool
			if harness == "claude" {
				var cwd string
				msgs, cwd, ok = parseClaude(line)
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
				msgs, meta, ok = parseCodex(line, codexSubagent)
				if meta != nil {
					if !opt.all && filepath.Clean(meta.cwd) != opt.workspace {
						return s, false
					}
					codexScoped, codexSubagent = true, meta.subagent
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
	if origin != "" && !opt.all {
		// In a file that records cwd, a record without one cannot be scoped.
		kept := s.msgs[:0]
		for _, msg := range s.msgs {
			if msg.cwd != "" {
				kept = append(kept, msg)
			}
		}
		s.msgs = kept
	}
	return s, len(s.msgs) > 0
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
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
	Message      *struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// humanText strips embedded tag blocks and reports whether human text remains.
func humanText(raw string) (string, bool) {
	text := stripTagged(raw)
	return text, !injected(text)
}

// parseClaude returns the record's kept messages and its cwd, if any.
func parseClaude(line []byte) ([]message, string, bool) {
	var record claudeRecord
	if err := json.Unmarshal(line, &record); err != nil {
		return nil, "", false
	}
	cwd := ""
	if record.Cwd != "" {
		cwd = filepath.Clean(record.Cwd)
	}
	msgs, ok := claudeMessages(record)
	for i := range msgs {
		msgs[i].cwd = cwd
	}
	return msgs, cwd, ok
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
	if record.IsMeta || record.PromptSource == "system" {
		return nil, true
	}
	var texts []string
	for _, block := range blocks {
		if block.Type == "tool_result" {
			return nil, true
		}
		if block.Type != "text" {
			continue
		}
		if text, ok := humanText(block.Text); ok {
			texts = append(texts, text)
		}
	}
	if len(texts) == 0 {
		return nil, true
	}
	role := "human"
	if record.IsSidechain {
		role = "prompt"
	}
	return []message{{role: role, text: strings.Join(texts, "\n"), ts: ts}}, true
}

type codexMeta struct {
	cwd      string
	subagent bool
}

func parseCodex(line []byte, subagent bool) ([]message, *codexMeta, bool) {
	var record struct {
		Type      string          `json:"type"`
		Timestamp string          `json:"timestamp"`
		Payload   json.RawMessage `json:"payload"`
	}
	if err := json.Unmarshal(line, &record); err != nil {
		return nil, nil, false
	}
	switch record.Type {
	case "session_meta":
		var payload struct {
			Cwd    string          `json:"cwd"`
			Source json.RawMessage `json:"source"`
		}
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			return nil, nil, false
		}
		var source map[string]json.RawMessage
		_ = json.Unmarshal(payload.Source, &source)
		_, isSubagent := source["subagent"]
		return nil, &codexMeta{cwd: payload.Cwd, subagent: isSubagent}, true
	case "response_item":
		var payload struct {
			Type    string         `json:"type"`
			Role    string         `json:"role"`
			Content []contentBlock `json:"content"`
		}
		if err := json.Unmarshal(record.Payload, &payload); err != nil {
			return nil, nil, false
		}
		if payload.Type != "message" || (payload.Role != "user" && payload.Role != "assistant") {
			return nil, nil, true
		}
		var texts []string
		for _, block := range payload.Content {
			switch {
			case payload.Role == "assistant" && block.Type == "output_text" && strings.TrimSpace(block.Text) != "":
				texts = append(texts, block.Text)
			case payload.Role == "user" && block.Type == "input_text":
				if text, ok := humanText(block.Text); ok {
					texts = append(texts, text)
				}
			}
		}
		if len(texts) == 0 {
			return nil, nil, true
		}
		role := payload.Role
		if role == "user" {
			role = "human"
			if subagent {
				role = "prompt"
			}
		}
		return []message{{role: role, text: strings.Join(texts, "\n"), ts: parseTime(record.Timestamp)}}, nil, true
	}
	return nil, nil, true
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
		firstHuman, firstPrompt, lastReply := "", "", ""
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
			case msg.role == "assistant":
				lastReply = msg.text
			}
		}
		if firstHuman == "" {
			firstHuman = firstPrompt
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
	fmt.Fprintf(w, "\nsummary: sessions scanned %d, human turns %d, classified turns %d, skipped lines %d", r.SessionsScanned, r.HumanTurns, r.ClassifiedTurns, r.SkippedLines)
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
