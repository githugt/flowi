// SPDX-License-Identifier: Apache-2.0

package report

import (
	"io"
	"strings"
)

// WriteMarkdown writes r as a Markdown summary (for digests, PR comments
// and CI job summaries).
func WriteMarkdown(w io.Writer, r Run) error {
	p := &printer{w: w}
	p.line(0, "# %s — %s %s", runTitle(r), statusMark(r.Status, false), r.Status)
	p.line(0, "")
	p.line(0, "| | |")
	p.line(0, "|---|---|")
	p.line(0, "| Run | `%s` |", r.ID)
	if r.FlowPath != "" {
		p.line(0, "| Flow | `%s` |", strings.Trim(r.Collection+"/"+r.FlowPath, "/"))
	}
	if r.Env != "" {
		p.line(0, "| Environment | %s |", r.Env)
	}
	p.line(0, "| Seed | %d |", r.Seed)
	if !r.Started.IsZero() {
		p.line(0, "| Started | %s |", r.Started.UTC().Format("2006-01-02 15:04:05 MST"))
	}
	p.line(0, "| Duration | %s |", humanDuration(r.Duration))
	if r.Aborted != nil {
		p.line(0, "")
		p.line(0, "> **Aborted** (%s after %s); in flight: %s", r.Aborted.Reason, r.Aborted.Grace, codeList(r.Aborted.Inflight))
	}
	p.line(0, "")
	p.line(0, "## Nodes")
	p.line(0, "")
	markdownNodes(p, r.Nodes, 0)
	markdownDiagSection(p, "Warnings", r.Warnings)
	if len(r.Notes) > 0 {
		p.line(0, "")
		p.line(0, "## Notes")
		p.line(0, "")
		for _, n := range r.Notes {
			p.line(0, "- `%s`: %s", n.Path, n.Text)
		}
	}
	markdownVersions(p, r.Versions)
	return p.err
}

func markdownNodes(p *printer, nodes []Node, depth int) {
	for _, n := range nodes {
		p.line(depth, "- %s **%s** (%s) — %s, %s", statusMark(n.Status, n.Handled), n.ID, n.Type, statusLabel(n.Status, n.Handled), humanDuration(n.Duration))
		for _, d := range n.Diagnostics {
			p.line(depth+1, "- `%s` %s", d.Code, d.Message)
		}
		for _, h := range n.Hooks {
			p.line(depth+1, "- %s hook %s %s `%s` — %s", statusMark(h.Status, false), h.Phase, h.Level, h.Step, humanDuration(h.Duration))
			for _, d := range h.Diagnostics {
				p.line(depth+2, "- `%s` %s", d.Code, d.Message)
			}
			markdownNodes(p, h.Nodes, depth+2)
		}
		for _, l := range n.Lanes {
			if l.Label == "" { // one unnamed lane (a subflow's nodes)
				markdownNodes(p, l.Nodes, depth+1)
				continue
			}
			p.line(depth+1, "- %s", l.Label)
			markdownNodes(p, l.Nodes, depth+2)
		}
	}
}

func markdownDiagSection(p *printer, title string, diags []Diagnostic) {
	if len(diags) == 0 {
		return
	}
	p.line(0, "")
	p.line(0, "## %s", title)
	p.line(0, "")
	for _, d := range diags {
		p.line(0, "- `%s` %s", d.Code, d.Message)
	}
}

func markdownVersions(p *printer, versions []VersionUse) {
	if len(versions) == 0 {
		return
	}
	p.line(0, "")
	p.line(0, "## Request versions")
	p.line(0, "")
	p.line(0, "| Node | Request | Version | Chosen by |")
	p.line(0, "|---|---|---|---|")
	for _, v := range versions {
		p.line(0, "| `%s` | `%s` | %s | %s |", v.Path, v.RequestPath, v.Version, v.Source)
	}
}

func codeList(items []string) string {
	quoted := make([]string, len(items))
	for i, s := range items {
		quoted[i] = "`" + s + "`"
	}
	return strings.Join(quoted, ", ")
}
