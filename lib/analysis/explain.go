// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// Sentence is one line of a flow told in words. Indent is how deep it sits inside branches, loops and lanes; Node
// is the step the line is about, so a reader can jump to it ("" for a line that only introduces a group).
type Sentence struct {
	Indent int    `json:"indent"`
	Text   string `json:"text"`
	Node   string `json:"node,omitempty"`
}

// Problem is something about a flow worth a look, said as a sentence about one step.
type Problem struct {
	Node string `json:"node"`
	Text string `json:"text"`
}

// Explanation is a flow in words, and what looks wrong with it.
type Explanation struct {
	Sentences []Sentence `json:"sentences"`
	Problems  []Problem  `json:"problems"`
}

// Explain tells f in words and lists what looks wrong. It works from the flow as written, like Trace, so it needs
// no run and no collection: a request is named by its path.
func Explain(f model.Flow) Explanation {
	return Explanation{Sentences: Sentences(f), Problems: problemsOf(f)}
}

// Sentences tells the steps of f in order, one line each, with branches, loops and parallel lanes indented below
// the line that introduces them.
func Sentences(f model.Flow) []Sentence {
	return sentencesOf(f.Nodes, 0, []Sentence{})
}

func sentencesOf(list []model.Node, indent int, out []Sentence) []Sentence {
	for _, n := range list {
		out = append(out, Sentence{Indent: indent, Text: sentenceOf(n), Node: n.ID})
		out = appendChildren(n, indent+1, out)
	}
	return out
}

// appendChildren adds the nested steps of n below its own sentence, each group under a line that names it.
func appendChildren(n model.Node, indent int, out []Sentence) []Sentence {
	group := func(text string, nodes []model.Node) {
		if len(nodes) == 0 {
			return
		}
		if text != "" {
			out = append(out, Sentence{Indent: indent, Text: text})
			out = sentencesOf(nodes, indent+1, out)
			return
		}
		out = sentencesOf(nodes, indent, out)
	}
	switch n.Type {
	case "condition":
		for _, c := range n.Cases {
			group("When "+plain(c.When)+":", c.Nodes)
		}
		if len(n.Cases) == 0 {
			// then runs without a heading: the sentence of the condition already says when.
			group("", n.Then)
		}
		if len(n.Else) > 0 {
			// The branch lines sit at the condition's own depth (the "Otherwise:" line), their steps below it.
			out = append(out, Sentence{Indent: indent - 1, Text: "Otherwise:"})
			out = sentencesOf(n.Else, indent, out)
		}
	case "parallel":
		for _, b := range n.Branches {
			group("Branch "+b.ID+":", b.Nodes)
		}
	default:
		group("", n.Body)
	}
	return out
}

// sentenceOf says one step in a line, with what makes it conditional added at the end.
func sentenceOf(n model.Node) string {
	text := sayStep(n)
	if n.When != "" {
		text += ", only when " + plain(n.When)
	}
	return text
}

func sayStep(n model.Node) string {
	param := func(key string) string { return plain(paramText(n, key)) }
	switch n.Type {
	case "request":
		return sayRequest(n.Request)
	case "fetch":
		return fmt.Sprintf("Fetch %s %s", methodOr(param("method"), "GET"), param("url"))
	case "webhook":
		return "Send a webhook to " + param("url")
	case "subflow":
		return saySubflow(n.Subflow)
	case "set":
		return saySet(n)
	case "assert":
		return sayAssert(n)
	case "condition":
		if len(n.Cases) > 0 {
			return "Pick the first case that holds:"
		}
		return "If " + param("check") + ":"
	case "parallel":
		return "At the same time:"
	case "batch":
		if over := param("over"); over != "" {
			return "For each item of " + over + ":"
		}
		return "Repeat " + param("times") + " times:"
	case "run_till":
		return "Repeat until " + param("until") + ":"
	case "delay":
		return "Wait " + param("duration")
	case "log":
		return "Write " + strconv.Quote(param("value")) + " to the log"
	case "note":
		return "Note: " + param("text")
	case "stop":
		return sayStop(n)
	case "save":
		return fmt.Sprintf("Save %s as %s", param("value"), param("key"))
	case "digest":
		return "Send a summary of " + valueOr(param("of"), "the run")
	case "fuzz":
		if n.Request == nil {
			return "Send bad bodies to a request"
		}
		return "Send bad bodies to " + n.Request.Request.Path
	}
	return "Run the " + string(n.Type) + " step"
}

func valueOr(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

func methodOr(m, fallback string) string { return strings.ToUpper(valueOr(m, fallback)) }

func sayRequest(c *model.RequestCall) string {
	if c == nil {
		return "Call a request"
	}
	text := "Call " + valueOr(c.Request.Path, string(c.Request.ID))
	return text + withText(c.With)
}

func saySubflow(c *model.SubflowCall) string {
	if c == nil {
		return "Run another flow"
	}
	return "Run the flow " + valueOr(c.Flow.Path, string(c.Flow.ID)) + withText(c.With)
}

// withText lists the values a step is given: ", with petId = create.body.id".
func withText(with []model.KeyValue) string {
	if len(with) == 0 {
		return ""
	}
	parts := make([]string, len(with))
	for i, kv := range with {
		parts[i] = kv.Key + " = " + plain(kv.Value)
	}
	return ", with " + strings.Join(parts, ", ")
}

func saySet(n model.Node) string {
	if len(n.Values) == 0 {
		return "Set " + plain(paramText(n, "target")) + " to " + plain(paramText(n, "value"))
	}
	parts := make([]string, len(n.Values))
	for i, v := range n.Values {
		parts[i] = saySetValue(v)
	}
	return "Set " + joinWithAnd(parts)
}

func saySetValue(v model.SetValue) string {
	switch v.From {
	case "response":
		return fmt.Sprintf("%s from the response %s", v.Name, responsePart(v))
	case "generate":
		return v.Name + " to a generated value"
	case "literal":
		return fmt.Sprintf("%s to %q", v.Name, fmt.Sprint(v.Value))
	}
	return fmt.Sprintf("%s to %s", v.Name, plain(fmt.Sprint(v.Value)))
}

// responsePart says which part of a response a set value reads: "body (token)", "header (ETag)", "status".
func responsePart(v model.SetValue) string {
	where := valueOr(v.Where, "body")
	switch where {
	case "header":
		return fmt.Sprintf("header (%s)", v.Header)
	case "cookie":
		return fmt.Sprintf("cookie (%s)", v.Cookie)
	case "status":
		return "status"
	}
	if v.Path != "" {
		return fmt.Sprintf("%s (%s)", where, v.Path)
	}
	return where
}

// joinWithAnd joins "a, b and c".
func joinWithAnd(parts []string) string {
	if len(parts) < 2 {
		return strings.Join(parts, "")
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

func sayAssert(n model.Node) string {
	switch {
	case paramText(n, "check") != "":
		return "Check that " + plain(paramText(n, "check"))
	case paramText(n, "schema") != "":
		return "Check that the response of " + paramText(n, "schema") + " matches its schema"
	case paramText(n, "problem") != "":
		return "Check that the response of " + paramText(n, "problem") + " is a problem document"
	}
	return "Check something"
}

func sayStop(n model.Node) string {
	text := "Stop the run"
	if paramText(n, "status") == "failed" {
		text = "Stop the run as failed"
	}
	if msg := plain(paramText(n, "message")); msg != "" {
		text += ": " + msg
	}
	return text
}

func paramText(n model.Node, key string) string {
	v := sites.ParamValue(n, key)
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

var braces = regexp.MustCompile(`\{\{\s*(.*?)\s*\}\}`)

// plain writes a template the way a person would say it: the braces go, and so do the "vars." and "nodes."
// prefixes that only say where a name lives ({{nodes.create.body.id}} is create.body.id, {{vars.token}} token).
func plain(text string) string {
	return braces.ReplaceAllStringFunc(text, func(m string) string {
		inner := braces.FindStringSubmatch(m)[1]
		inner = strings.TrimPrefix(inner, "nodes.")
		return strings.TrimPrefix(inner, "vars.")
	})
}

// problemsOf lists variables that are set but not read by any step or flow output, and variables that are read
// but no step sets, in flow order.
func problemsOf(f model.Flow) []Problem {
	flat := flattenNodes(f.Nodes, nil)
	problems := []Problem{}
	for _, n := range flat {
		for _, w := range sites.OwnWrites(n) {
			name, ok := strings.CutPrefix(w, "vars.")
			if ok && !isRead(f, flat, name) {
				problems = append(problems, Problem{Node: n.ID, Text: name + " is set but nothing reads it"})
			}
		}
	}
	for _, n := range flat {
		for _, name := range varsRead(f, n) {
			if !isSet(flat, name) {
				problems = append(problems, Problem{Node: n.ID, Text: name + " is read but no step sets it"})
			}
		}
	}
	return problems
}

func isRead(f model.Flow, flat []model.Node, name string) bool {
	path := []string{"vars", name}
	if slices.ContainsFunc(flat, func(n model.Node) bool { return readsPath(f, n, path) }) {
		return true
	}
	return slices.ContainsFunc(f.Outputs, func(o model.Output) bool { return readsVar(o.Value, false, name) })
}

func isSet(flat []model.Node, name string) bool {
	return slices.ContainsFunc(flat, func(n model.Node) bool { return slices.Contains(sites.OwnWrites(n), "vars."+name) })
}

// varsRead lists the variable names n reads, once each, in the order they appear.
func varsRead(f model.Flow, n model.Node) []string {
	var names []string
	for _, site := range sites.NodeTemplates(sites.Located[model.Node]{File: f.File, Value: n}) {
		refs, err := template.References(site.Src, site.Expression)
		if err != nil {
			continue // the validator reports a template that does not compile
		}
		for _, ref := range refs {
			if len(ref.Path) >= 2 && ref.Path[0] == "vars" && !slices.Contains(names, ref.Path[1]) {
				names = append(names, ref.Path[1])
			}
		}
	}
	return names
}

func readsVar(src string, expression bool, name string) bool {
	refs, err := template.References(src, expression)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(refs, func(r template.Reference) bool {
		return len(r.Path) >= 2 && r.Path[0] == "vars" && r.Path[1] == name
	})
}
