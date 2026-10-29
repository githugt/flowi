// SPDX-License-Identifier: AGPL-3.0-or-later

package card

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// stepBuilder makes the card of one step type.
type stepBuilder func(model.Node, Facts) Card

// stepBuilders are the cards of every step that is not a request, by type.
func stepBuilders() map[model.NodeType]stepBuilder {
	return map[model.NodeType]stepBuilder{
		"subflow":   flowCard,
		"set":       setCard,
		"assert":    checkCard,
		"fuzz":      fuzzCard,
		"condition": routeCard,
		"batch":     loopCard,
		"run_till":  loopCard,
		"parallel":  groupCard,
		"delay":     waitCard,
		"log":       outputCard,
		"save":      outputCard,
		"digest":    outputCard,
		"stop":      stopCard,
		"note":      noteCard,
		"fetch":     legacyCallCard,
		"webhook":   legacyCallCard,
	}
}

func summarizeStep(n model.Node, f Facts) Card {
	if build, ok := stepBuilders()[n.Type]; ok {
		return build(n, f)
	}
	return Card{Subline: string(n.Type), Ports: donePorts()}
}

func row(key, editor, summary string) Row {
	return Row{Key: key, Icon: key, Editor: editor, Summary: summary}
}

// param is the text of a node parameter, "" when unset or not text.
func param(n model.Node, key string) string {
	for _, p := range n.Params {
		if p.Key != key {
			continue
		}
		switch v := p.Value.(type) {
		case string:
			return v
		case int64, int, float64, bool:
			return fmt.Sprint(v)
		}
	}
	return ""
}

// bare is a template without its braces, as the person thinks of it.
func bare(src string) string {
	src = strings.TrimSpace(src)
	if inner, ok := strings.CutPrefix(src, "{{"); ok {
		if inner, ok = strings.CutSuffix(inner, "}}"); ok {
			return strings.TrimSpace(inner)
		}
	}
	return src
}

func flowCard(n model.Node, f Facts) Card {
	c := Card{Subline: f.Target, Ports: donePorts()}
	passed := 0
	if n.Subflow != nil {
		passed = len(n.Subflow.With)
	}
	if s := count(passed, "passed", "passed"); s != "" {
		c.Rows = append(c.Rows, row("inputs", "inputs", s))
	} else {
		c.More = append(c.More, row("inputs", "inputs", ""))
	}
	if f.Returns > 0 {
		c.Rows = append(c.Rows, row("values", "returns", fmt.Sprintf("%d returned", f.Returns)))
	}
	return c
}

// setCard lists the names a set step writes: up to three, then "+n".
func setCard(n model.Node, _ Facts) Card {
	names := setNames(n)
	c := Card{Subline: count(len(names), "value", "values"), Ports: donePorts()}
	if len(names) == 0 { // an empty set still has a row to open, or there would be nothing to click
		c.Rows = []Row{row("values", "values", "add a value")}
		return c
	}
	shown := names
	if len(shown) > 3 {
		shown = shown[:3]
	}
	summary := strings.Join(shown, ", ")
	if extra := len(names) - len(shown); extra > 0 {
		summary += fmt.Sprintf(" +%d", extra)
	}
	c.Rows = []Row{row("values", "values", summary)}
	return c
}

func setNames(n model.Node) []string {
	var names []string
	for _, v := range n.Values {
		if v.Name != "" {
			names = append(names, v.Name)
		}
	}
	if target := param(n, "target"); len(names) == 0 && target != "" {
		names = append(names, strings.TrimPrefix(target, "vars."))
	}
	return names
}

func checkCard(n model.Node, _ Facts) Card {
	c := Card{Subline: "1 check", Ports: donePorts()}
	if check := param(n, "check"); check != "" {
		c.Rows = append(c.Rows, row("checks", "checks", check))
	}
	if step := param(n, "schema"); step != "" {
		c.Rows = append(c.Rows, row("checks", "checks", "matches the schema of "+step))
	}
	if len(c.Rows) > 1 {
		c.Subline = fmt.Sprintf("%d checks", len(c.Rows))
	}
	return c
}

// fuzzCard says which request it sends bad bodies to, how many, and what makes it fail. Its rows have no editor
// of their own: the count and the expectation are edited in the side panel.
func fuzzCard(n model.Node, _ Facts) Card {
	count := param(n, "count")
	if count == "" {
		count = "20"
	}
	c := Card{Subline: count + " bad bodies", Ports: donePorts()}
	if n.Request != nil {
		c.Rows = append(c.Rows, Row{Key: "request", Icon: "request", Summary: n.Request.Request.Path})
	}
	expect := "fails on a server error"
	if param(n, "expect") == "rejected" {
		expect = "fails when a bad body is accepted"
	}
	c.Rows = append(c.Rows, Row{Key: "checks", Icon: "checks", Summary: expect})
	return c
}

// routeCard has no rows: the cases are its ports.
func routeCard(n model.Node, _ Facts) Card {
	c := Card{Subline: routeBasis(n)}
	for i, k := range n.Cases {
		c.Ports = append(c.Ports, Port{ID: fmt.Sprintf("case-%d", i), Label: caseLabel(k.When), Tone: "case"})
	}
	if len(n.Cases) == 0 {
		c.Ports = append(c.Ports, Port{ID: "case-0", Label: caseLabel(param(n, "check")), Tone: "case"})
	}
	c.Ports = append(c.Ports, Port{ID: "else", Label: "else", Tone: "else"})
	return c
}

var (
	statusTest  = regexp.MustCompile(`^\s*(?:nodes\.[\w-]+\.)?status\s*(==|!=|>=|<=|>|<)\s*(\d{3})\s*$`)
	statusClass = regexp.MustCompile(`^\s*(?:nodes\.[\w-]+\.)?status\s*>=\s*(\d)00\s*&&\s*(?:nodes\.[\w-]+\.)?status\s*<\s*(\d)00\s*$`)
)

// caseLabel is what a case's port says: a test of the response status in its
// short form ("== 404", "5xx"), anything else as written, cut to fit.
func caseLabel(when string) string {
	if m := statusClass.FindStringSubmatch(when); m != nil && next(m[1]) == m[2] {
		return m[1] + "xx"
	}
	if m := statusTest.FindStringSubmatch(when); m != nil {
		return m[1] + " " + m[2]
	}
	return cut(when, portLabelAt)
}

// next is the digit after d, "" when d is not a digit below 9.
func next(d string) string {
	if len(d) != 1 || d[0] < '0' || d[0] >= '9' {
		return ""
	}
	return string(d[0] + 1)
}

// routeBasis says whether the cases test a response status or a value.
func routeBasis(n model.Node) string {
	conditions := []string{param(n, "check")}
	for _, k := range n.Cases {
		conditions = append(conditions, k.When)
	}
	for _, src := range conditions {
		if readsStatus(src) {
			return "by status"
		}
	}
	return "by value"
}

func readsStatus(src string) bool {
	refs, err := template.References(src, true)
	if err != nil {
		return false
	}
	for _, r := range refs {
		if len(r.Path) > 0 && r.Path[len(r.Path)-1] == "status" {
			return true
		}
	}
	return false
}

func loopCard(n model.Node, _ Facts) Card {
	c := Card{Subline: loopMode(n), Ports: donePorts()}
	c.Rows = []Row{row("steps", "lane", count(len(n.Body), "step", "steps"))}
	if len(n.Body) == 0 {
		c.Rows[0].Summary = "no steps yet"
	}
	return c
}

func loopMode(n model.Node) string {
	if n.Type == "run_till" {
		out := "until " + bare(param(n, "until"))
		if limit := param(n, "max"); limit != "" {
			out += ", max " + limit
		}
		return out
	}
	out := "over " + bare(param(n, "over"))
	if times := param(n, "times"); times != "" {
		out = times + " times"
	}
	if width := param(n, "concurrency"); width != "" && width != "1" {
		out += ", " + width + " at a time"
	}
	return out
}

func groupCard(n model.Node, _ Facts) Card {
	basis := map[string]string{"any": "together, wait for any", "first-success": "together, first success wins"}
	subline, ok := basis[param(n, "wait")]
	if !ok {
		subline = "together, wait for all"
	}
	c := Card{Subline: subline, Ports: donePorts()}
	c.Rows = []Row{row("steps", "lane", count(len(n.Branches), "branch", "branches"))}
	return c
}

// waitCard shows the delay; a range "1s..3s" reads "1s to 3s".
func waitCard(n model.Node, _ Facts) Card {
	return Card{Subline: strings.Replace(param(n, "duration"), "..", " to ", 1), Ports: donePorts()}
}

// outputCard covers log, save and digest: where the value goes, and the value.
func outputCard(n model.Node, _ Facts) Card {
	c := Card{Ports: donePorts()}
	switch n.Type {
	case "save":
		c.Subline = "File (" + cmpOr(param(n, "mode"), "overwrite") + ")"
	case "digest":
		c.Subline = digestWhere(param(n, "sink"))
	default:
		c.Subline = "Run log"
	}
	if value := param(n, "value"); value != "" {
		c.Rows = []Row{row("values", "values", value)}
	}
	return c
}

func digestWhere(sink string) string {
	switch sink {
	case "file":
		return "File"
	case "webhook":
		return "Webhook"
	}
	return "Run log"
}

func cmpOr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// stopCard has only a fail port when it stops as failed: the run goes no
// further either way, so it has no Done to follow.
func stopCard(n model.Node, _ Facts) Card {
	status := cmpOr(param(n, "status"), "passed")
	c := Card{Subline: "as " + status}
	if status == "failed" {
		c.Ports = []Port{{ID: "fail", Label: "Fail", Tone: "fail"}}
	} else {
		c.Ports = []Port{{ID: "done", Label: "Done", Tone: "done"}}
	}
	if msg := param(n, "message"); msg != "" {
		c.Rows = []Row{row("message", "message", msg)}
	}
	return c
}

// noteCard is a sticky note: its text, no ports.
func noteCard(n model.Node, _ Facts) Card {
	return Card{Badge: "NOTE", Subline: param(n, "text")}
}

// legacyCallCard is a fetch or webhook step: the url and nothing to open.
func legacyCallCard(n model.Node, _ Facts) Card {
	method := strings.ToUpper(cmpOr(param(n, "method"), "GET"))
	if n.Type == "webhook" {
		method = "POST"
	}
	return Card{Badge: method, Subline: param(n, "url"), Ports: donePorts()}
}
