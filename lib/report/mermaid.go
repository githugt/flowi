// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"io"
	"net/url"
	"strings"
)

// WriteMermaid writes r as a Mermaid sequence diagram of what actually happened: Flowi and one participant for
// each host it called, an arrow out and an arrow back for each attempt (with its status and time), and a note
// for a step that is not a call and failed. It is the living picture of a run, drawn from the report, so it
// cannot drift from the flow the way a hand-drawn diagram does. Only the host and path of a URL are shown:
// the query string can hold a credential.
func WriteMermaid(w io.Writer, r Run) error {
	s := SequenceOf(r)
	var sb strings.Builder
	sb.WriteString("sequenceDiagram\n")
	if s.Title != "" {
		fmt.Fprintf(&sb, "    title %s\n", mermaidText(s.Title))
	}
	sb.WriteString("    participant F as Flowi\n")
	for i, h := range s.Hosts {
		fmt.Fprintf(&sb, "    participant H%d as %s\n", i+1, mermaidText(h))
	}
	for _, st := range s.Steps {
		sb.WriteString("    " + mermaidLine(st) + "\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func mermaidLine(st SequenceStep) string {
	switch st.Kind {
	case "call":
		return fmt.Sprintf("F->>H%d: %s", st.Host+1, mermaidText(st.Text))
	case "reply":
		arrow := "-->>"
		if st.Failed {
			arrow = "--x"
		}
		return fmt.Sprintf("H%d%sF: %s", st.Host+1, arrow, mermaidText(st.Text))
	}
	return "Note over F: " + mermaidText(st.Text)
}

// Sequence is a run drawn as a sequence diagram, as data: a window can draw it, WriteMermaid writes it as text.
type Sequence struct {
	Title string
	Hosts []string // the hosts the run called, in the order it first called them; Flowi is implied
	Steps []SequenceStep
}

// SequenceStep is one arrow or note. Kind is "call" (Flowi to Host), "reply" (Host back to Flowi) or "note"
// (Host is -1). Failed marks a refusal, an error or a step that failed.
type SequenceStep struct {
	Kind   string
	Host   int
	Text   string
	Failed bool
}

// SequenceOf draws the run: every attempt of every request in the order it happened, loops and hooks included.
func SequenceOf(r Run) Sequence {
	d := &sequence{hosts: map[string]int{}}
	collectCalls(r.Nodes, d)
	return Sequence{Title: runTitle(r), Hosts: d.order, Steps: d.steps}
}

// sequence collects the hosts and steps of a diagram in the order things happened.
type sequence struct {
	hosts map[string]int // host → index in order
	order []string
	steps []SequenceStep
}

func (d *sequence) host(host string) int {
	if i, ok := d.hosts[host]; ok {
		return i
	}
	d.hosts[host] = len(d.order)
	d.order = append(d.order, host)
	return len(d.order) - 1
}

// collectCalls walks the nodes in run order, loops and hooks included.
func collectCalls(nodes []Node, d *sequence) {
	for _, n := range nodes {
		for _, h := range n.Hooks {
			collectCalls(h.Nodes, d)
		}
		for _, a := range n.Attempts {
			d.attempt(n.ID, a)
		}
		if len(n.Attempts) == 0 && n.Status == "failed" && len(n.Lanes) == 0 {
			d.steps = append(d.steps, SequenceStep{Kind: "note", Host: -1, Text: n.ID + " failed" + failureWhy(n), Failed: true})
		}
		for _, l := range n.Lanes {
			collectCalls(l.Nodes, d)
		}
	}
}

func failureWhy(n Node) string {
	if len(n.Diagnostics) == 0 {
		return ""
	}
	return ": " + strings.NewReplacer(";", ",", "\n", " ").Replace(n.Diagnostics[0].Message)
}

func (d *sequence) attempt(step string, a Attempt) {
	host, path := hostAndPath(a.URL)
	i := d.host(host)
	d.steps = append(d.steps, SequenceStep{Kind: "call", Host: i, Text: step + ": " + a.Method + " " + path})
	switch {
	case a.Error != "":
		d.steps = append(d.steps, SequenceStep{Kind: "reply", Host: i, Text: a.Error, Failed: true})
	default:
		d.steps = append(d.steps, SequenceStep{Kind: "reply", Host: i, Text: answerText(a), Failed: a.Status >= 400})
	}
}

func answerText(a Attempt) string {
	text := fmt.Sprint(a.Status)
	switch ms := a.Duration.Milliseconds(); {
	case ms > 0:
		text += fmt.Sprintf(" · %d ms", ms)
	case a.Duration > 0:
		text += " · <1 ms"
	}
	return text
}

// hostAndPath splits a URL for display, dropping the query and any credentials.
func hostAndPath(raw string) (host, path string) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "unknown", "/"
	}
	path = u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if len(path) > 60 {
		path = path[:57] + "..."
	}
	return u.Host, path
}

// mermaidText makes text safe in a sequence diagram line: a semicolon would end the line and a hash starts an
// entity, and a line break would end it too.
func mermaidText(s string) string {
	return strings.NewReplacer(";", ",", "#", "", "\n", " ", "\r", " ").Replace(s)
}
