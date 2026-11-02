package maintain

import (
	"fmt"
	"strconv"
	"strings"
)

// The directory edition reads two small YAML documents: SKILL.md front matter
// and agents/openai.yaml. The repository is standard-library only, so this
// parser covers the subset those files use: nested block mappings, block and
// flow lists of scalars, block scalars, and plain, single-quoted and
// double-quoted scalars. Anything else is reported as malformed, which is the
// safe direction for a package gate.

type yamlKind int

const (
	yamlNull yamlKind = iota
	yamlScalar
	yamlMap
	yamlList
)

type yamlValue struct {
	kind   yamlKind
	text   string
	quoted bool
	keys   []string
	m      map[string]yamlValue
	list   []yamlValue
}

func (v yamlValue) get(key string) (yamlValue, bool) {
	value, ok := v.m[key]
	return value, ok
}

type yamlParser struct {
	lines []string
	pos   int
}

func parseYAMLSubset(text string) (yamlValue, error) {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	p := &yamlParser{lines: strings.Split(text, "\n")}
	if _, _, ok, err := p.peek(); err != nil {
		return yamlValue{}, err
	} else if !ok {
		return yamlValue{kind: yamlNull}, nil
	}
	value, err := p.parseBlock(0)
	if err != nil {
		return yamlValue{}, err
	}
	if _, _, ok, err := p.peek(); err != nil {
		return yamlValue{}, err
	} else if ok {
		return yamlValue{}, p.errorf("unexpected content")
	}
	return value, nil
}

func (p *yamlParser) errorf(format string, args ...any) error {
	return fmt.Errorf("line %d: %s", p.pos+1, fmt.Sprintf(format, args...))
}

// peek returns the next content line without consuming it, skipping blanks
// and comments.
func (p *yamlParser) peek() (indent int, text string, ok bool, err error) {
	for p.pos < len(p.lines) {
		raw := strings.TrimRight(p.lines[p.pos], " \t")
		trimmed := strings.TrimLeft(raw, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			p.pos++
			continue
		}
		if strings.HasPrefix(trimmed, "\t") {
			return 0, "", false, p.errorf("tab in indentation")
		}
		return len(raw) - len(trimmed), trimmed, true, nil
	}
	return 0, "", false, nil
}

func isYAMLListItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

func (p *yamlParser) parseBlock(indent int) (yamlValue, error) {
	i, text, ok, err := p.peek()
	if err != nil {
		return yamlValue{}, err
	}
	if !ok || i < indent {
		return yamlValue{kind: yamlNull}, nil
	}
	if isYAMLListItem(text) {
		return p.parseList(i)
	}
	return p.parseMap(i)
}

func (p *yamlParser) parseMap(indent int) (yamlValue, error) {
	out := yamlValue{kind: yamlMap, m: map[string]yamlValue{}}
	for {
		i, text, ok, err := p.peek()
		if err != nil {
			return yamlValue{}, err
		}
		if !ok || i < indent {
			return out, nil
		}
		if i > indent {
			return yamlValue{}, p.errorf("unexpected indentation")
		}
		if isYAMLListItem(text) {
			return yamlValue{}, p.errorf("list item where a mapping key is expected")
		}
		key, rest, err := splitYAMLKey(text)
		if err != nil {
			return yamlValue{}, p.errorf("%v", err)
		}
		if _, dup := out.m[key]; dup {
			return yamlValue{}, p.errorf("duplicate key %q", key)
		}
		p.pos++
		value, err := p.parseValue(indent, rest)
		if err != nil {
			return yamlValue{}, err
		}
		out.keys = append(out.keys, key)
		out.m[key] = value
	}
}

func (p *yamlParser) parseList(indent int) (yamlValue, error) {
	out := yamlValue{kind: yamlList}
	for {
		i, text, ok, err := p.peek()
		if err != nil {
			return yamlValue{}, err
		}
		if !ok || i < indent || (i == indent && !isYAMLListItem(text)) {
			return out, nil
		}
		if i > indent {
			return yamlValue{}, p.errorf("unexpected indentation")
		}
		body := strings.TrimLeft(text[1:], " ")
		offset := len(text) - len(body)
		if body != "" && !strings.ContainsAny(body[:1], `"'[{`) && looksLikeYAMLKey(body) {
			// "- key: value" starts a mapping whose later keys align with key.
			p.lines[p.pos] = strings.Repeat(" ", indent+offset) + body
			value, err := p.parseMap(indent + offset)
			if err != nil {
				return yamlValue{}, err
			}
			out.list = append(out.list, value)
			continue
		}
		p.pos++
		value, err := p.parseValue(indent, body)
		if err != nil {
			return yamlValue{}, err
		}
		out.list = append(out.list, value)
	}
}

func looksLikeYAMLKey(text string) bool {
	return strings.Contains(text, ": ") || strings.HasSuffix(text, ":")
}

func splitYAMLKey(text string) (key, rest string, err error) {
	if text[0] == '"' || text[0] == '\'' {
		quote := text[0]
		end := -1
		for j := 1; j < len(text); j++ {
			if text[j] == '\\' && quote == '"' {
				j++
				continue
			}
			if text[j] == quote {
				end = j
				break
			}
		}
		if end < 0 || end+1 >= len(text) || text[end+1] != ':' {
			return "", "", fmt.Errorf("expected \"key: value\"")
		}
		value, _, err := unquoteYAML(text[:end+1])
		if err != nil {
			return "", "", err
		}
		return value, strings.TrimSpace(text[end+2:]), nil
	}
	idx := strings.Index(text, ": ")
	if idx < 0 {
		if strings.HasSuffix(text, ":") {
			idx = len(text) - 1
		} else {
			return "", "", fmt.Errorf("expected \"key: value\", found %q", truncateForError(text))
		}
	}
	key = strings.TrimSpace(text[:idx])
	if key == "" {
		return "", "", fmt.Errorf("empty mapping key")
	}
	return key, strings.TrimSpace(text[idx+1:]), nil
}

func truncateForError(text string) string {
	if len(text) > 40 {
		return text[:40] + "..."
	}
	return text
}

func unquoteYAML(token string) (string, bool, error) {
	if token[0] == '"' {
		value, err := strconv.Unquote(token)
		if err != nil {
			return "", false, fmt.Errorf("invalid double-quoted scalar")
		}
		return value, true, nil
	}
	return strings.ReplaceAll(token[1:len(token)-1], "''", "'"), true, nil
}

// closingQuote returns the index of the quote that closes a quoted scalar
// starting at text[0], or -1.
func closingQuote(text string) int {
	quote := text[0]
	for j := 1; j < len(text); j++ {
		switch {
		case quote == '"' && text[j] == '\\':
			j++
		case text[j] == quote && quote == '\'' && j+1 < len(text) && text[j+1] == '\'':
			j++
		case text[j] == quote:
			return j
		}
	}
	return -1
}

func (p *yamlParser) parseValue(parentIndent int, rest string) (yamlValue, error) {
	switch {
	case rest == "" || strings.HasPrefix(rest, "#"):
		i, text, ok, err := p.peek()
		if err != nil {
			return yamlValue{}, err
		}
		if ok && (i > parentIndent || (i == parentIndent && isYAMLListItem(text))) {
			return p.parseBlock(i)
		}
		return yamlValue{kind: yamlNull}, nil
	case rest[0] == '|' || rest[0] == '>':
		return p.blockScalar(parentIndent, rest)
	case rest[0] == '[':
		return parseFlowList(rest)
	case rest[0] == '{':
		if strings.TrimSpace(rest) == "{}" {
			return yamlValue{kind: yamlMap, m: map[string]yamlValue{}}, nil
		}
		return yamlValue{}, p.errorf("flow mappings are not supported")
	case rest[0] == '"' || rest[0] == '\'':
		token := rest
		for closingQuote(token) < 0 {
			if p.pos >= len(p.lines) {
				return yamlValue{}, p.errorf("unterminated quoted scalar")
			}
			token += " " + strings.TrimSpace(p.lines[p.pos])
			p.pos++
		}
		end := closingQuote(token)
		tail := strings.TrimSpace(token[end+1:])
		if tail != "" && !strings.HasPrefix(tail, "#") {
			return yamlValue{}, p.errorf("unexpected text after quoted scalar")
		}
		text, _, err := unquoteYAML(token[:end+1])
		if err != nil {
			return yamlValue{}, p.errorf("%v", err)
		}
		return yamlValue{kind: yamlScalar, text: text, quoted: true}, nil
	}
	text := stripYAMLComment(rest)
	for p.pos < len(p.lines) {
		raw := strings.TrimRight(p.lines[p.pos], " \t")
		trimmed := strings.TrimLeft(raw, " ")
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || len(raw)-len(trimmed) <= parentIndent {
			break
		}
		text += " " + stripYAMLComment(trimmed)
		p.pos++
	}
	if looksLikeYAMLKey(text) {
		return yamlValue{}, p.errorf("mapping values are not allowed in a plain scalar; quote %q", truncateForError(text))
	}
	if strings.ContainsAny(text[:1], "@`%*&!") {
		return yamlValue{}, p.errorf("plain scalar may not start with %q; quote it", text[:1])
	}
	return yamlValue{kind: yamlScalar, text: text}, nil
}

func stripYAMLComment(text string) string {
	if idx := strings.Index(text, " #"); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

func parseFlowList(rest string) (yamlValue, error) {
	rest = strings.TrimSpace(stripYAMLComment(rest))
	if !strings.HasSuffix(rest, "]") {
		return yamlValue{}, fmt.Errorf("unterminated flow list")
	}
	inner := strings.TrimSpace(rest[1 : len(rest)-1])
	out := yamlValue{kind: yamlList}
	if inner == "" {
		return out, nil
	}
	for _, part := range strings.Split(inner, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return yamlValue{}, fmt.Errorf("empty flow list item")
		}
		if part[0] == '"' || part[0] == '\'' {
			if closingQuote(part) != len(part)-1 {
				return yamlValue{}, fmt.Errorf("unsupported quoted flow list item")
			}
			text, _, err := unquoteYAML(part)
			if err != nil {
				return yamlValue{}, err
			}
			out.list = append(out.list, yamlValue{kind: yamlScalar, text: text, quoted: true})
			continue
		}
		out.list = append(out.list, yamlValue{kind: yamlScalar, text: part})
	}
	return out, nil
}

func (p *yamlParser) blockScalar(parentIndent int, header string) (yamlValue, error) {
	folded := header[0] == '>'
	chomp := ""
	if len(header) > 1 {
		chomp = header[1:]
		if chomp != "-" && chomp != "+" {
			return yamlValue{}, p.errorf("unsupported block scalar header %q", header)
		}
	}
	var lines []string
	blockIndent := -1
	for p.pos < len(p.lines) {
		raw := strings.TrimRight(p.lines[p.pos], "\r")
		trimmed := strings.TrimLeft(raw, " ")
		if strings.TrimSpace(raw) == "" {
			lines = append(lines, "")
			p.pos++
			continue
		}
		indent := len(raw) - len(trimmed)
		if indent <= parentIndent {
			break
		}
		if blockIndent < 0 {
			blockIndent = indent
		}
		if indent < blockIndent {
			break
		}
		lines = append(lines, raw[blockIndent:])
		p.pos++
	}
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	var text string
	if folded {
		text = strings.Join(lines, " ")
	} else {
		text = strings.Join(lines, "\n")
	}
	if chomp != "-" && len(lines) > 0 && !folded {
		text += "\n"
	}
	return yamlValue{kind: yamlScalar, text: text, quoted: true}, nil
}
