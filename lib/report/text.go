// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"io"
	"strings"
)

// WriteText writes r as indented plain lines: a header, the node tree,
// then warnings, notes and the abort reason.
func WriteText(w io.Writer, r Run) error {
	p := &printer{w: w}
	p.line(0, "%s %s  %s  %s", statusMark(r.Status, false), runTitle(r), r.Status, humanDuration(r.Duration))
	p.line(1, "run %s · seed %d%s", r.ID, r.Seed, envSuffix(r.Env))
	textNodes(p, r.Nodes, 1)
	if len(r.Warnings) > 0 {
		p.line(0, "Warnings:")
		for _, d := range r.Warnings {
			p.line(1, "%s %s", d.Code, d.Message)
		}
	}
	if len(r.Notes) > 0 {
		p.line(0, "Notes:")
		for _, n := range r.Notes {
			p.line(1, "%s: %s", n.Path, n.Text)
		}
	}
	if r.Aborted != nil {
		p.line(0, "Aborted: %s after %s; in flight: %s", r.Aborted.Reason, r.Aborted.Grace, strings.Join(r.Aborted.Inflight, ", "))
	}
	return p.err
}

func textNodes(p *printer, nodes []Node, depth int) {
	for _, n := range nodes {
		p.line(depth, "%s %s  %s%s", statusMark(n.Status, n.Handled), n.ID, humanDuration(n.Duration), handledSuffix(n))
		for _, d := range n.Diagnostics {
			p.line(depth+2, "%s %s", d.Code, d.Message)
		}
		for _, h := range n.Hooks {
			p.line(depth+1, "%s hook %s %s %s  %s", statusMark(h.Status, false), h.Phase, h.Level, h.Step, humanDuration(h.Duration))
			for _, d := range h.Diagnostics {
				p.line(depth+3, "%s %s", d.Code, d.Message)
			}
			textNodes(p, h.Nodes, depth+2)
		}
		for _, l := range n.Lanes {
			if l.Label == "" { // one unnamed lane (a subflow's nodes)
				textNodes(p, l.Nodes, depth+1)
				continue
			}
			p.line(depth+1, "%s", l.Label)
			textNodes(p, l.Nodes, depth+2)
		}
	}
}

func runTitle(r Run) string {
	if r.FlowName != "" {
		return r.FlowName
	}
	if r.FlowPath != "" {
		return r.FlowPath
	}
	return r.Kind
}

func envSuffix(env string) string {
	if env == "" {
		return ""
	}
	return " · env " + env
}

func handledSuffix(n Node) string {
	if n.Handled {
		return "  (failed, continued)"
	}
	return ""
}

// printer writes indented lines and keeps the first error.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) line(depth int, format string, args ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, "%s%s\n", strings.Repeat("  ", depth), fmt.Sprintf(format, args...))
}
