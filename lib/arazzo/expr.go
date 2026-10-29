// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// Expression mapping (plan Decision 4). Arazzo runtime expressions become
// Flowi references and back:
//
//	$inputs.x                 inputs.x
//	$steps.s.outputs.o        vars.s_o (written by the step's set node)
//	$statusCode               nodes.<step>.status
//	$response.body#/a/0       nodes.<step>.body.a[0]
//	$response.header.X-Rate   nodes.<step>.headers["x-rate"]
//
// Anything else ($message, $outputs, $request, $components, …) has no
// Flowi equivalent and is reported by the caller, never guessed.

var (
	identifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	unsafeVar  = regexp.MustCompile(`[^A-Za-z0-9_]`)
	embedded   = regexp.MustCompile(`\{(\$[^{}]+)\}`)
	// dollarToken is one runtime expression inside a condition.
	dollarToken = regexp.MustCompile(`\$[A-Za-z]+(?:\.[A-Za-z0-9_\-]+)*(?:#/[^\s()'"=!<>&|]*)?`)
	template    = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)
	refStep     = regexp.MustCompile(`^\.([A-Za-z_][A-Za-z0-9_]*)|^\[(\d+)\]|^\[("(?:[^"\\]|\\.)*")\]`)
	refRoot     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
)

// varName is the variable a step output is stored in.
func varName(step, output string) string {
	return unsafeVar.ReplaceAllString(step+"_"+output, "_")
}

// segment renders one reference step: .name, [0] or ["odd-name"].
func segment(s string) string {
	switch {
	case identifier.MatchString(s):
		return "." + s
	case s != "" && strings.Trim(s, "0123456789") == "":
		return "[" + s + "]"
	}
	return "[" + strconv.Quote(s) + "]"
}

func nodeRef(step string) string { return "nodes" + segment(step) }

// mapRef maps one runtime expression; step is the step whose response
// $response and $statusCode mean, or "" where there is none.
func mapRef(expr, step string) (string, bool) {
	switch {
	case strings.HasPrefix(expr, "$inputs."):
		return dotted("inputs", strings.TrimPrefix(expr, "$inputs."))
	case strings.HasPrefix(expr, "$steps."):
		parts := strings.Split(strings.TrimPrefix(expr, "$steps."), ".")
		if len(parts) != 3 || parts[1] != "outputs" {
			return "", false
		}
		return "vars." + varName(parts[0], parts[2]), true
	case step == "":
		return "", false
	case expr == "$statusCode":
		return nodeRef(step) + ".status", true
	case expr == "$response.body":
		return nodeRef(step) + ".body", true
	case strings.HasPrefix(expr, "$response.body#/"):
		out := nodeRef(step) + ".body"
		for _, p := range strings.Split(strings.TrimPrefix(expr, "$response.body#/"), "/") {
			out += segment(strings.ReplaceAll(strings.ReplaceAll(p, "~1", "/"), "~0", "~"))
		}
		return out, true
	case strings.HasPrefix(expr, "$response.header."):
		return nodeRef(step) + ".headers" + segment(strings.ToLower(strings.TrimPrefix(expr, "$response.header."))), true
	}
	return "", false
}

func dotted(root, path string) (string, bool) {
	out := root
	for _, p := range strings.Split(path, ".") {
		if !identifier.MatchString(p) {
			return "", false
		}
		out += "." + p
	}
	return out, true
}

// mapExpression maps a parameter or output value: a whole expression
// ($inputs.x), text with embedded ones ("Bearer {$inputs.t}"), or a
// literal. Literal "{{" would be read as a template, so it is refused.
func mapExpression(expr, step string) (string, bool) {
	if strings.Contains(expr, "{{") {
		return "", false
	}
	if strings.HasPrefix(expr, "$") {
		ref, ok := mapRef(expr, step)
		return "{{" + ref + "}}", ok
	}
	ok := true
	out := embedded.ReplaceAllStringFunc(expr, func(m string) string {
		ref, mapped := mapRef(m[1:len(m)-1], step)
		ok = ok && mapped
		return "{{" + ref + "}}"
	})
	return out, ok
}

// stringLiteral matches a quoted string in a condition.
var stringLiteral = regexp.MustCompile(`'(?:[^'\\]|\\.)*'|"(?:[^"\\]|\\.)*"`)

// mapCondition maps a simple success criterion to an assert check. Text
// inside string literals is copied as is.
func mapCondition(cond, step string) (string, bool) {
	ok := true
	mapCode := func(code string) string {
		return dollarToken.ReplaceAllStringFunc(code, func(m string) string {
			ref, mapped := mapRef(m, step)
			ok = ok && mapped
			return ref
		})
	}
	var b strings.Builder
	last := 0
	for _, loc := range stringLiteral.FindAllStringIndex(cond, -1) {
		b.WriteString(mapCode(cond[last:loc[0]]))
		b.WriteString(cond[loc[0]:loc[1]])
		last = loc[1]
	}
	b.WriteString(mapCode(cond[last:]))
	return b.String(), ok
}

// unmapTemplate is the inverse of mapExpression. vars maps variable names
// written by exported step outputs to their $steps expressions.
func unmapTemplate(tmpl, step string, vars map[string]string) (string, bool) {
	// Literal text an Arazzo runner would read as an expression is refused.
	if literal := template.ReplaceAllString(tmpl, "\x00"); strings.HasPrefix(literal, "$") || strings.Contains(literal, "{$") {
		return "", false
	}
	if m := template.FindStringSubmatchIndex(tmpl); m != nil && m[0] == 0 && m[1] == len(tmpl) {
		return unmapRef(tmpl[m[2]:m[3]], step, vars)
	}
	ok := true
	out := template.ReplaceAllStringFunc(tmpl, func(m string) string {
		expr, mapped := unmapRef(template.FindStringSubmatch(m)[1], step, vars)
		ok = ok && mapped
		return "{" + expr + "}"
	})
	return out, ok
}

// unmapRef maps one Flowi reference back to a runtime expression.
func unmapRef(ref, step string, vars map[string]string) (string, bool) {
	segs, ok := splitRef(ref)
	if !ok || len(segs) < 2 {
		return "", false
	}
	switch segs[0] {
	case "inputs":
		return "$inputs." + strings.Join(segs[1:], "."), true
	case "vars":
		expr, known := vars[segs[1]]
		return expr, known && len(segs) == 2
	case "nodes":
		if segs[1] != step || len(segs) < 3 {
			return "", false
		}
		return unmapResponse(segs[2:])
	}
	return "", false
}

func unmapResponse(segs []string) (string, bool) {
	switch {
	case segs[0] == "status" && len(segs) == 1:
		return "$statusCode", true
	case segs[0] == "headers" && len(segs) == 2:
		return "$response.header." + segs[1], true
	case segs[0] == "body" && len(segs) == 1:
		return "$response.body", true
	case segs[0] == "body":
		var parts []string
		for _, s := range segs[1:] {
			parts = append(parts, strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1"))
		}
		return "$response.body#/" + strings.Join(parts, "/"), true
	}
	return "", false
}

// splitRef splits a plain reference (a.b[0]["c-d"]) into its steps;
// anything else (calls, operators) is not a plain reference.
func splitRef(ref string) ([]string, bool) {
	root := refRoot.FindString(ref)
	if root == "" {
		return nil, false
	}
	out := []string{root}
	for rest := ref[len(root):]; rest != ""; {
		m := refStep.FindStringSubmatch(rest)
		if m == nil {
			return nil, false
		}
		s := m[1] + m[2]
		if m[3] != "" {
			var err error
			if s, err = strconv.Unquote(m[3]); err != nil {
				return nil, false
			}
		}
		out = append(out, s)
		rest = rest[len(m[0]):]
	}
	return out, true
}

// isConditionWord reports the bare words an exported check may contain:
// Arazzo's literals. Words like and, not and nil are Flowi-only.
func isConditionWord(s string) bool {
	return slices.Contains([]string{"true", "false", "null"}, s)
}

// conditionOperators are the operators Arazzo simple conditions have.
func isConditionOperator(op string) bool {
	return slices.Contains([]string{"==", "!=", "<", "<=", ">", ">=", "&&", "||", "!"}, op)
}

// unmapCondition maps an assert check back to a success criterion. Only
// Arazzo's own syntax passes: single-quoted strings, numbers, true, false,
// null, parentheses, its operators and references that map. Anything
// else (double quotes, functions, indexes that did not map, arithmetic,
// ternaries, and/or/not words) makes the check unexportable.
func unmapCondition(check, step string, vars map[string]string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(check); {
		c := check[i]
		switch {
		case c == '\'':
			end := closingQuote(check, i)
			b.WriteString(check[i:end])
			i = end
		case c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z':
			token := referenceAt(check[i:])
			expr, ok := unmapToken(token, step, vars)
			if !ok || strings.HasPrefix(strings.TrimLeft(check[i+len(token):], " \t"), "(") {
				return "", false // unmappable, or a call
			}
			b.WriteString(expr)
			i += len(token)
		case strings.ContainsRune("=!<>&|", rune(c)):
			op := operatorAt(check[i:])
			if !isConditionOperator(op) {
				return "", false
			}
			b.WriteString(op)
			i += len(op)
		case strings.ContainsRune(" \t()0123456789", rune(c)), numberPart(check, i):
			b.WriteByte(c)
			i++
		default:
			return "", false
		}
	}
	return b.String(), true
}

// numberPart reports a '-' that signs a number (after an operator, "(" or
// the start) or a '.' inside one; Arazzo has no arithmetic.
func numberPart(s string, i int) bool {
	digitAt := func(j int) bool { return j >= 0 && j < len(s) && s[j] >= '0' && s[j] <= '9' }
	switch s[i] {
	case '.':
		return digitAt(i-1) && digitAt(i+1)
	case '-':
		before := strings.TrimRight(s[:i], " \t")
		return digitAt(i+1) && (before == "" || strings.ContainsRune("=!<>&|(", rune(before[len(before)-1])))
	}
	return false
}

// operatorAt returns the operator at the start of s: "!" alone when
// followed by anything but "=", else the run of operator characters.
func operatorAt(s string) string {
	if s[0] == '!' && (len(s) == 1 || s[1] != '=') {
		return "!"
	}
	n := 0
	for n < len(s) && strings.ContainsRune("=!<>&|", rune(s[n])) {
		n++
	}
	return s[:n]
}

func unmapToken(token, step string, vars map[string]string) (string, bool) {
	if isConditionWord(token) {
		return token, true
	}
	return unmapRef(token, step, vars)
}

// referenceAt returns the longest plain reference at the start of s.
func referenceAt(s string) string {
	n := len(refRoot.FindString(s))
	for {
		m := refStep.FindString(s[n:])
		if m == "" {
			return s[:n]
		}
		n += len(m)
	}
}

// closingQuote returns the index just past the string literal at i (or
// the end of s for an unterminated one).
func closingQuote(s string, i int) int {
	q := s[i]
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case q:
			return j + 1
		}
	}
	return len(s)
}
