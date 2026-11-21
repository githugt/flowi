// SPDX-License-Identifier: Apache-2.0

package postman

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/importers"
	"github.com/githugt/flowi/lib/model"
)

// A Postman test script is JavaScript, which Flowi does not run. The idioms nearly every collection uses are
// translated: saving a value of the response into a variable becomes an extract, and a status, header, body or time
// check in pm.test / pm.expect becomes an assert. What is left is kept, word for word, in the request's docs, so
// nothing is lost and nothing is silently pretended to work.

type scriptResult struct {
	translated int      // statements that became an extract or an assert
	kept       []string // statements (and whole pre-request scripts) that did not
}

type scriptTranslator struct {
	v      *model.RequestVersion
	alias  map[string]bool // variables that hold pm.response.json()
	result scriptResult
}

var (
	reString    = `"(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'`
	reAlias     = regexp.MustCompile(`^(?:var|let|const)\s+(\w+)\s*=\s*(?:pm\.response\.json\(\)|JSON\.parse\((?:responseBody|pm\.response\.text\(\))\))$`)
	reSet       = regexp.MustCompile(`^pm\.(?:environment|collectionVariables|globals|variables)\.set\(\s*(` + reString + `)\s*,\s*(.+)\)$`)
	reStatus    = regexp.MustCompile(`^pm\.response\.to\.have\.status\(\s*(\d{3})\s*\)$`)
	reHeaderHas = regexp.MustCompile(`^pm\.response\.to\.have\.header\(\s*(` + reString + `)\s*\)$`)
	reExpect    = regexp.MustCompile(`^pm\.expect\((.+?)\)\.((?:to|not)\..+)$`)
	reTestOne   = regexp.MustCompile(`^pm\.test\(.*?,\s*(?:function\s*\(\s*\)|\(\s*\)\s*=>)\s*\{(.*)\}\s*\)\s*$`)
	reTestOpen  = regexp.MustCompile(`^pm\.test\(.*(?:function\s*\(\s*\)|\(\s*\)\s*=>)\s*\{\s*$`)
	reCloser    = regexp.MustCompile(`^\}\s*\)\s*$`)
	reHeaderGet = regexp.MustCompile(`^pm\.response\.headers\.get\(\s*(` + reString + `)\s*\)$`)
	reJSPath    = regexp.MustCompile(`^(?:\.[A-Za-z_]\w*|\[\d+\]|\[(?:` + reString + `)\])*$`)
	reLiteral   = regexp.MustCompile(`^(?:-?\d+(?:\.\d+)?|true|false|` + reString + `)$`)
	reNumber    = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)
	reName      = regexp.MustCompile(`^[A-Za-z_]\w*$`)
	reWord      = regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	reStrLit    = regexp.MustCompile(reString)
)

// translateScripts turns the test script of a request into extracts and asserts on v, and keeps what it cannot
// translate in v.Docs. A pre-request script is kept whole: it runs before the request and has no equivalent.
func translateScripts(rep *importers.Item, v *model.RequestVersion, events []event) {
	var total scriptResult
	for _, e := range events {
		text := strings.Join(e.Script.Exec, "\n")
		if strings.TrimSpace(text) == "" {
			continue
		}
		tr := &scriptTranslator{v: v, alias: map[string]bool{}}
		switch e.Listen {
		case "test":
			for _, stmt := range statements(e.Script.Exec) {
				tr.statement(stmt)
			}
		default:
			label := cmpOr(map[string]string{"prerequest": "pre-request script"}[e.Listen], e.Listen+" script")
			tr.result.kept = append(tr.result.kept, "// "+label+"\n"+strings.TrimSpace(text))
		}
		total.translated += tr.result.translated
		total.kept = append(total.kept, tr.result.kept...)
	}
	if len(total.kept) == 0 {
		return
	}
	var kept []string
	for _, k := range total.kept {
		kept = append(kept, redactScript(rep, k))
	}
	note := "Imported from Postman: script lines Flowi does not run, kept as they were.\n\n```js\n" + strings.Join(kept, "\n") + "\n```"
	if v.Docs != "" {
		v.Docs += "\n\n"
	}
	v.Docs += note
	how := "kept in the request's docs"
	if total.translated > 0 {
		how = fmt.Sprintf("partly translated: %d %s became extract or assert, the rest kept in the request's docs", total.translated, plural(total.translated, "statement", "statements"))
	}
	rep.Unmapped("request script", how, "scripts are JavaScript; Flowi has no scripts, use hooks and extract or assert nodes")
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// redactScript replaces the string literals of a line that mentions a credential, except the names of credentials: the script is kept in a file
// that is committed, and a password or token written in it must not go along.
func redactScript(rep *importers.Item, text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		for _, w := range reWord.FindAllString(line, -1) {
			if importers.SensitiveHeader(w) && reStrLit.MatchString(line) {
				lines[i] = reStrLit.ReplaceAllStringFunc(line, func(lit string) string {
					if name := unquote(lit); reName.MatchString(name) && importers.SensitiveHeader(name) {
						return lit // the name of the credential, not its value
					}
					return `"…"`
				})
				rep.Redact("a string in a script line", w)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

// statements splits a script into statements: one per line, several per line when ';' separates them, and the
// bodies of pm.test(...) callbacks without their wrapper.
func statements(exec []string) []string {
	var out []string
	for _, line := range exec {
		line = strings.TrimSpace(line)
		switch {
		case line == "" || strings.HasPrefix(line, "//"):
		case reTestOpen.MatchString(line) || reCloser.MatchString(strings.TrimSuffix(line, ";")):
		default:
			if m := reTestOne.FindStringSubmatch(strings.TrimSuffix(line, ";")); m != nil {
				line = m[1]
			}
			for _, s := range splitStatements(line) {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// splitStatements cuts on ';' outside string literals.
func splitStatements(line string) []string {
	var out []string
	var quote rune
	start := 0
	escaped := false
	for i, r := range line {
		switch {
		case escaped:
			escaped = false
		case r == '\\' && quote != 0:
			escaped = true
		case quote != 0:
			if r == quote {
				quote = 0
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ';':
			out = append(out, line[start:i])
			start = i + 1
		}
	}
	return append(out, line[start:])
}

func (t *scriptTranslator) keep(stmt string) { t.result.kept = append(t.result.kept, stmt) }

func (t *scriptTranslator) statement(stmt string) {
	stmt = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(stmt), ";"))
	if m := reAlias.FindStringSubmatch(stmt); m != nil {
		t.alias[m[1]] = true
		return // naming the parsed body is no behaviour of its own
	}
	if t.translate(stmt) {
		t.result.translated++
		return
	}
	t.keep(stmt)
}

func (t *scriptTranslator) translate(stmt string) bool {
	switch {
	case reSet.MatchString(stmt):
		m := reSet.FindStringSubmatch(stmt)
		return t.extract(unquote(m[1]), strings.TrimSpace(m[2]))
	case reStatus.MatchString(stmt):
		return t.assert("status == " + reStatus.FindStringSubmatch(stmt)[1])
	case stmt == "pm.response.to.be.ok":
		return t.assert("status >= 200") && t.assert("status < 300")
	case reHeaderHas.MatchString(stmt):
		return t.assert(fmt.Sprintf("response.headers[%s] != empty", quote(strings.ToLower(unquote(reHeaderHas.FindStringSubmatch(stmt)[1])))))
	case reExpect.MatchString(stmt):
		m := reExpect.FindStringSubmatch(stmt)
		return t.expect(strings.TrimSpace(m[1]), m[2])
	}
	return false
}

func (t *scriptTranslator) assert(check string) bool {
	for _, have := range t.v.Assert {
		if have == check {
			return true
		}
	}
	t.v.Assert = append(t.v.Assert, check)
	return true
}

// operand is the Flowi expression for a pm.expect subject.
func (t *scriptTranslator) operand(subject string) (expr string, isTime, ok bool) {
	switch {
	case subject == "pm.response.code":
		return "status", false, true
	case subject == "pm.response.responseTime":
		return "response.time", true, true
	}
	if m := reHeaderGet.FindStringSubmatch(subject); m != nil {
		return fmt.Sprintf("response.headers[%s]", quote(strings.ToLower(unquote(m[1])))), false, true
	}
	if path, ok := t.bodyPath(subject); ok {
		return "response.body" + exprPath(path), false, true
	}
	return "", false, false
}

// bodyPath is the JavaScript path after the parsed body ("" for the body itself), when subject reads one.
func (t *scriptTranslator) bodyPath(subject string) (string, bool) {
	rest, ok := strings.CutPrefix(subject, "pm.response.json()")
	if !ok {
		name := reWord.FindString(subject)
		if !t.alias[name] || !strings.HasPrefix(subject, name) {
			return "", false
		}
		rest = subject[len(name):]
	}
	return rest, reJSPath.MatchString(rest)
}

func (t *scriptTranslator) expect(subject, matcher string) bool {
	left, isTime, ok := t.operand(subject)
	if !ok {
		return false
	}
	call := regexp.MustCompile(`^(.*)\(\s*(.+?)\s*\)$`).FindStringSubmatch(matcher)
	name, arg := matcher, ""
	if call != nil {
		name, arg = call[1], call[2]
	}
	op := map[string]string{
		"to.eql": "==", "to.equal": "==", "to.eq": "==", "to.be.equal": "==", "to.deep.equal": "==", "to.be.eql": "==",
		"to.not.eql": "!=", "to.not.equal": "!=", "to.not.eq": "!=", "to.not.be.equal": "!=",
		"to.be.above": ">", "to.be.greaterThan": ">", "to.be.below": "<", "to.be.lessThan": "<",
		"to.be.at.least": ">=", "to.be.at.most": "<=",
		"to.include": "contains", "to.contain": "contains", "to.includes": "contains", "to.have.string": "contains",
	}[name]
	switch {
	case call == nil && (matcher == "to.exist" || matcher == "to.not.be.null" || matcher == "to.not.be.undefined"):
		return !isTime && t.assert(left+" != empty")
	case op == "":
		return false
	case !reLiteral.MatchString(arg):
		return false
	}
	switch {
	case isTime:
		if !reNumber.MatchString(arg) || (op != "<" && op != ">" && op != "<=" && op != ">=") {
			return false
		}
		return t.assert(fmt.Sprintf("%s %s %sms", left, op, arg))
	case op == ">" || op == "<" || op == ">=" || op == "<=":
		if !reNumber.MatchString(arg) {
			return false
		}
	case op == "contains":
		if reNumber.MatchString(arg) || arg == "true" || arg == "false" {
			return false // contains looks in text; a number or true is not a text to look for
		}
	}
	return t.assert(fmt.Sprintf("%s %s %s", left, op, literal(arg)))
}

// extract turns pm.<scope>.set(name, value) into an extraction when value reads the response.
func (t *scriptTranslator) extract(name, value string) bool {
	if !reName.MatchString(name) {
		return false
	}
	for _, e := range t.v.Extract {
		if e.Name == name {
			return false // a second value for the same name is not what an extract can say
		}
	}
	e := model.Extraction{Name: name}
	switch {
	case value == "pm.response.code":
		e.From = "status"
	case reHeaderGet.MatchString(value):
		e.From, e.Header = "header", unquote(reHeaderGet.FindStringSubmatch(value)[1])
	default:
		path, ok := t.bodyPath(value)
		if !ok {
			return false
		}
		e.From, e.Path = "body", "$"+jsonPath(path)
	}
	t.v.Extract = append(t.v.Extract, e)
	return true
}

var reSegment = regexp.MustCompile(`\.[A-Za-z_]\w*|\[\d+\]|\[(?:` + reString + `)\]`)

// jsonPath writes a JavaScript property path as a JSONPath: .a.b[0]['c'].
func jsonPath(js string) string {
	var b strings.Builder
	for _, seg := range reSegment.FindAllString(js, -1) {
		if strings.HasPrefix(seg, "[") && (seg[1] == '"' || seg[1] == '\'') {
			b.WriteString("['" + strings.ReplaceAll(unquote(seg[1:len(seg)-1]), "'", `\'`) + "']")
			continue
		}
		b.WriteString(seg)
	}
	return b.String()
}

// exprPath writes the same path for an assert expression: .a.b[0]["c"].
func exprPath(js string) string {
	var b strings.Builder
	for _, seg := range reSegment.FindAllString(js, -1) {
		if strings.HasPrefix(seg, "[") && (seg[1] == '"' || seg[1] == '\'') {
			b.WriteString("[" + quote(unquote(seg[1:len(seg)-1])) + "]")
			continue
		}
		b.WriteString(seg)
	}
	return b.String()
}

// unquote reads a JavaScript string literal written with either quote.
func unquote(s string) string {
	if len(s) < 2 {
		return s
	}
	body := s[1 : len(s)-1]
	if s[0] == '"' {
		if u, err := strconv.Unquote(s); err == nil {
			return u
		}
	}
	return strings.NewReplacer(`\'`, `'`, `\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t").Replace(body)
}

// quote writes text as a double-quoted expression string.
func quote(s string) string { return strconv.Quote(s) }

// literal is a JavaScript literal written for an expression: numbers and true/false as they are, text double-quoted.
func literal(js string) string {
	if reNumber.MatchString(js) || js == "true" || js == "false" {
		return js
	}
	return quote(unquote(js))
}
