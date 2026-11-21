// SPDX-License-Identifier: Apache-2.0

package report

import (
	"fmt"
	"html"
	"io"
	"strings"
)

// WriteHTML writes r as one self-contained HTML page: no script, no link, the style inline, so the file can be
// mailed, attached to a CI run or opened from a folder. It holds the summary, every step with its status and time,
// the diagnostics, the warnings and notes, and the request versions that were used.
func WriteHTML(w io.Writer, r Run) error {
	p := &printer{w: w}
	title := html.EscapeString(runTitle(r))
	p.line(0, "<!doctype html>")
	p.line(0, `<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">`)
	p.line(0, "<title>%s: %s</title>", title, html.EscapeString(r.Status))
	p.line(0, "<style>%s</style></head><body>", htmlStyle)
	p.line(0, `<h1>%s <span class="status %s">%s %s</span></h1>`, title, cssClass(r.Status), statusMark(r.Status, false), html.EscapeString(r.Status))
	p.line(0, `<table class="facts">`)
	fact := func(k, v string) { p.line(1, "<tr><th>%s</th><td>%s</td></tr>", k, v) }
	fact("Run", "<code>"+html.EscapeString(r.ID)+"</code>")
	if r.FlowPath != "" {
		fact("Flow", "<code>"+html.EscapeString(strings.Trim(r.Collection+"/"+r.FlowPath, "/"))+"</code>")
	}
	if r.Env != "" {
		fact("Environment", html.EscapeString(r.Env))
	}
	fact("Seed", fmt.Sprint(r.Seed))
	if !r.Started.IsZero() {
		fact("Started", r.Started.UTC().Format("2006-01-02 15:04:05 MST"))
	}
	fact("Duration", html.EscapeString(humanDuration(r.Duration)))
	p.line(0, "</table>")
	if r.Aborted != nil {
		p.line(0, `<p class="aborted"><strong>Aborted</strong> (%s after %s); in flight: %s</p>`,
			html.EscapeString(r.Aborted.Reason), html.EscapeString(fmt.Sprint(r.Aborted.Grace)), html.EscapeString(strings.Join(r.Aborted.Inflight, ", ")))
	}
	p.line(0, "<h2>Steps</h2>")
	htmlNodes(p, r.Nodes, 0)
	htmlDiagnostics(p, "Warnings", r.Warnings)
	if len(r.Notes) > 0 {
		p.line(0, "<h2>Notes</h2><ul>")
		for _, n := range r.Notes {
			p.line(1, "<li><code>%s</code>: %s</li>", html.EscapeString(n.Path), html.EscapeString(n.Text))
		}
		p.line(0, "</ul>")
	}
	if len(r.Versions) > 0 {
		p.line(0, "<h2>Request versions</h2><table><tr><th>Step</th><th>Request</th><th>Version</th><th>Chosen by</th></tr>")
		for _, v := range r.Versions {
			p.line(1, "<tr><td><code>%s</code></td><td><code>%s</code></td><td>%s</td><td>%s</td></tr>",
				html.EscapeString(v.Path), html.EscapeString(v.RequestPath), html.EscapeString(v.Version), html.EscapeString(v.Source))
		}
		p.line(0, "</table>")
	}
	p.line(0, "</body></html>")
	return p.err
}

func htmlNodes(p *printer, nodes []Node, depth int) {
	if len(nodes) == 0 {
		return
	}
	p.line(depth, "<ul>")
	for _, n := range nodes {
		p.line(depth+1, `<li><span class="status %s">%s</span> <strong>%s</strong> <span class="type">(%s)</span> %s, %s`,
			cssClass(n.Status), statusMark(n.Status, n.Handled), html.EscapeString(n.ID), html.EscapeString(n.Type),
			html.EscapeString(statusLabel(n.Status, n.Handled)), html.EscapeString(humanDuration(n.Duration)))
		for _, d := range n.Diagnostics {
			p.line(depth+2, `<div class="diag"><code>%s</code> %s</div>`, html.EscapeString(d.Code), html.EscapeString(d.Message))
		}
		for _, h := range n.Hooks {
			p.line(depth+2, `<div class="hook"><span class="status %s">%s</span> hook %s %s <code>%s</code>, %s</div>`,
				cssClass(h.Status), statusMark(h.Status, false), html.EscapeString(h.Phase), html.EscapeString(h.Level), html.EscapeString(h.Step), html.EscapeString(humanDuration(h.Duration)))
			for _, d := range h.Diagnostics {
				p.line(depth+3, `<div class="diag"><code>%s</code> %s</div>`, html.EscapeString(d.Code), html.EscapeString(d.Message))
			}
			htmlNodes(p, h.Nodes, depth+2)
		}
		for _, l := range n.Lanes {
			if l.Label != "" {
				p.line(depth+2, `<div class="lane">%s</div>`, html.EscapeString(l.Label))
			}
			htmlNodes(p, l.Nodes, depth+2)
		}
		p.line(depth+1, "</li>")
	}
	p.line(depth, "</ul>")
}

func htmlDiagnostics(p *printer, title string, diags []Diagnostic) {
	if len(diags) == 0 {
		return
	}
	p.line(0, "<h2>%s</h2><ul>", html.EscapeString(title))
	for _, d := range diags {
		p.line(1, "<li><code>%s</code> %s</li>", html.EscapeString(d.Code), html.EscapeString(d.Message))
	}
	p.line(0, "</ul>")
}

// cssClass turns a status into a class name; anything odd is "other", so a value cannot break out of the attribute.
func cssClass(status string) string {
	switch status {
	case "passed", "failed", "error", "skipped", "cancelled", "aborted", "running":
		return status
	}
	return "other"
}

const htmlStyle = `body{font:15px/1.5 system-ui,sans-serif;max-width:60rem;margin:2rem auto;padding:0 1rem;color:#1d2330;background:#fff}` +
	`h1{font-size:1.5rem}h2{font-size:1.15rem;margin-top:2rem}code{font:13px ui-monospace,monospace;background:#f1f2f5;padding:0 .25em;border-radius:3px}` +
	`table{border-collapse:collapse}th,td{text-align:left;padding:.2rem .8rem .2rem 0;vertical-align:top}` +
	`ul{list-style:none;padding-left:1.2rem;border-left:1px solid #dfe2e8}.type{color:#667}.diag,.hook,.lane{margin-left:1.2rem;font-size:14px}` +
	`.status{font-weight:600}.passed{color:#187a50}.failed,.error{color:#b42318}.skipped,.cancelled,.aborted,.other{color:#667}.running{color:#a16207}` +
	`.aborted{color:#b42318}@media (prefers-color-scheme:dark){body{background:#14171f;color:#e6e8ee}code{background:#232834}ul{border-color:#2d3340}.passed{color:#4cc38a}.failed,.error{color:#ff7b72}}`
