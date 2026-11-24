// SPDX-License-Identifier: Apache-2.0

package speclint

import (
	"regexp"
	"strings"
)

var (
	rulesBlockLine = regexp.MustCompile(`^rules:[ \t]*(#.*)?\r?$`)
	rulesEmptyLine = regexp.MustCompile(`^rules:[ \t]*\{[ \t]*\}[ \t]*(#.*)?\r?$`)
	rulesAnyLine   = regexp.MustCompile(`^rules[ \t]*:`)
	entryLine      = regexp.MustCompile(`^([ \t]+)(?:'([^']*)'|"([^"]*)"|([^\s:#'"]+))[ \t]*:[ \t]*`)
)

// Edit sets one rule in the text of a lint file, leaving every other line as it was, so the comments a team wrote in
// it stay. level "" (or the rule's own default) takes the rule's line out. A file whose rules are written some other
// way than one `name: level` per line (a flow mapping with entries, say) cannot be edited in place: it is written
// again from what it says, keeping the comment lines above the first key.
func Edit(old []byte, rule, level string) []byte {
	if level == DefaultLevel(rule) {
		level = ""
	}
	if len(strings.TrimSpace(string(old))) > 0 {
		if out, ok := editInPlace(string(old), rule, level); ok {
			return []byte(out)
		}
	}
	cfg, _ := ParseConfig(old, "")
	if cfg == nil {
		cfg = Config{}
	}
	if level == "" {
		delete(cfg, rule)
	} else {
		cfg[rule] = level
	}
	return append(leadingComments(string(old)), withoutHeader(Render(cfg))...)
}

// leadingComments are the comment and blank lines before the first key.
func leadingComments(text string) []byte {
	var b strings.Builder
	for _, line := range strings.SplitAfter(text, "\n") {
		t := strings.TrimSpace(line)
		if t != "" && !strings.HasPrefix(t, "#") {
			break
		}
		b.WriteString(line)
	}
	out := b.String()
	if out == "" {
		return []byte(strings.SplitAfter(string(Render(Config{})), "\n")[0])
	}
	if !strings.HasSuffix(out, "\n") {
		out += "\n"
	}
	return []byte(out)
}

// withoutHeader drops Render's own first comment line, which leadingComments stands in for.
func withoutHeader(rendered []byte) []byte {
	_, rest, _ := strings.Cut(string(rendered), "\n")
	return []byte(rest)
}

func editInPlace(text, rule, level string) (string, bool) {
	lines := strings.Split(text, "\n")
	at := -1
	for i, l := range lines {
		if rulesAnyLine.MatchString(l) {
			if at >= 0 {
				return "", false // two rules: keys
			}
			at = i
		}
	}
	if at < 0 {
		return appendRules(text, rule, level), true
	}
	switch {
	case rulesEmptyLine.MatchString(lines[at]):
		if level == "" {
			return text, true
		}
		comment := rulesEmptyLine.FindStringSubmatch(lines[at])[1]
		lines[at] = "rules:"
		if comment != "" {
			lines[at] += " " + comment
		}
		return strings.Join(slicesInsert(lines, at+1, "  "+rule+": "+level), "\n"), true
	case !rulesBlockLine.MatchString(lines[at]):
		return "", false
	}
	// The block: the lines after rules: that are indented, with the blank and comment lines among them.
	end, last, indent, entries := at+1, at, "  ", 0
	for end < len(lines) {
		l := lines[end]
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") && (l[0] == ' ' || l[0] == '\t') {
			end++
			continue
		}
		if l[0] != ' ' && l[0] != '\t' {
			break
		}
		m := entryLine.FindStringSubmatch(l)
		if m == nil {
			return "", false // a line of some other shape (a nested value, a list)
		}
		if entries == 0 {
			indent = m[1]
		}
		entries++
		last = end
		if firstOf(m[2], m[3], m[4]) == rule {
			return setEntry(lines, end, at, entries == 1 && !otherEntries(lines, end+1, len(lines)), level), true
		}
		end++
	}
	if level == "" {
		return text, true
	}
	return strings.Join(slicesInsert(lines, last+1, indent+rule+": "+level), "\n"), true
}

// otherEntries says whether an entry follows line from on, inside the block.
func otherEntries(lines []string, from, to int) bool {
	for _, l := range lines[from:to] {
		if l != "" && (l[0] == ' ' || l[0] == '\t') && entryLine.MatchString(l) {
			return true
		}
		if l != "" && l[0] != ' ' && l[0] != '\t' && !strings.HasPrefix(strings.TrimSpace(l), "#") {
			return false
		}
	}
	return false
}

// setEntry rewrites the rule's line with a new level, keeping the comment after it, or takes the line out. Taking out
// the only entry leaves `rules: {}`.
func setEntry(lines []string, i, rulesAt int, only bool, level string) string {
	if level == "" {
		if only {
			lines[rulesAt] = rulesBlockLine.ReplaceAllString(lines[rulesAt], "rules: {}$1")
			lines[rulesAt] = strings.Replace(lines[rulesAt], "{}#", "{} #", 1)
		}
		lines = append(lines[:i], lines[i+1:]...)
		return strings.Join(lines, "\n")
	}
	m := entryLine.FindString(lines[i])
	rest := lines[i][len(m):]
	// the old level runs to a space, a comment or the end of the line
	end := 0
	for end < len(rest) && rest[end] != ' ' && rest[end] != '\t' && rest[end] != '#' && rest[end] != '\r' {
		end++
	}
	lines[i] = m + level + rest[end:]
	return strings.Join(lines, "\n")
}

func appendRules(text, rule, level string) string {
	if level == "" {
		return text
	}
	if text != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	return text + "rules:\n  " + rule + ": " + level + "\n"
}

func firstOf(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func slicesInsert(lines []string, at int, line string) []string {
	out := make([]string, 0, len(lines)+1)
	out = append(out, lines[:at]...)
	out = append(out, line)
	return append(out, lines[at:]...)
}
