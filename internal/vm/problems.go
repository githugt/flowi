// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"path/filepath"
	"slices"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
)

// Problem is one row of the Problems panel.
type Problem struct {
	Severity string // error | warning
	Code     string
	Message  string // with its hint
	File     string
	Where    string // file name with line and column
	Node     string // tree node to open on click; "" when the file has no screen
}

// Problems is the Problems panel: workspace validation, refreshed after
// loads and saves.
type Problems struct {
	b        Backend
	w        *Worker
	ws       *Workspace
	items    []Problem                         // validation problems
	run      map[flowi.RequestTarget][]Problem // what each request's last send reported
	onChange []func()
}

// NewProblems returns the panel's view-model; it refreshes whenever ws
// changes.
func NewProblems(b Backend, w *Worker, ws *Workspace) *Problems {
	p := &Problems{b: b, w: w, ws: ws, run: map[flowi.RequestTarget][]Problem{}}
	ws.OnChange(p.Refresh)
	return p
}

// OnChange registers f to run after a refresh.
func (p *Problems) OnChange(f func()) { p.onChange = append(p.onChange, f) }

// Refresh validates the workspace in the background.
func (p *Problems) Refresh() {
	Run(p.w, p.b.Validate, func(l diag.List) {
		p.items = make([]Problem, len(l))
		for i, d := range l {
			p.items[i] = p.problemOf(d)
		}
		for _, f := range p.onChange {
			f()
		}
	})
}

func (p *Problems) problemOf(d diag.Diagnostic) Problem {
	msg, hint := diag.English().Message(d)
	if hint != "" {
		msg += " (" + hint + ")"
	}
	where := filepath.Base(d.File)
	if d.Pos.Line > 0 {
		where += fmt.Sprintf(":%d:%d", d.Pos.Line, d.Pos.Column)
	}
	sev := "warning"
	if d.Severity == diag.SeverityError {
		sev = "error"
	}
	return Problem{Severity: sev, Code: string(d.Code), Message: msg, File: d.File, Where: where, Node: p.nodeFor(d.File)}
}

// nodeFor finds the tree node showing file: a request or an environment.
func (p *Problems) nodeFor(file string) string {
	for _, c := range p.ws.Collections() {
		for _, r := range c.Requests {
			if r.File == file {
				return "r:" + string(c.Manifest.ID) + ":" + string(r.ID)
			}
		}
	}
	for _, e := range p.ws.Environment() {
		if e.File == file {
			return "e:" + e.File
		}
	}
	return ""
}

// SetRun records what the last send of request t reported (assertion
// failures, missing variables, …), replacing its earlier report; an empty
// list clears it, as a passing send does. They are listed after the
// validation problems.
func (p *Problems) SetRun(t flowi.RequestTarget, l diag.List) {
	if len(l) == 0 {
		delete(p.run, t)
	} else {
		items := make([]Problem, len(l))
		for i, d := range l {
			items[i] = p.problemOf(d)
			items[i].Where = lastSendText
			items[i].Node = "r:" + string(t.Collection) + ":" + string(t.Request)
		}
		p.run[t] = items
	}
	for _, f := range p.onChange {
		f()
	}
}

// ClearRun forgets what the sends reported. The validation problems stay: they
// are the files' own and go when the files are fixed.
func (p *Problems) ClearRun() {
	p.run = map[flowi.RequestTarget][]Problem{}
	for _, f := range p.onChange {
		f()
	}
}

// lastSendText is the "where" of a problem reported by a send.
const lastSendText = "last send"

// Items are the problems: validation first (errors first as validate sorts
// them), then what the last sends reported.
func (p *Problems) Items() []Problem {
	out := slices.Clone(p.items)
	targets := make([]flowi.RequestTarget, 0, len(p.run))
	for t := range p.run {
		targets = append(targets, t)
	}
	slices.SortFunc(targets, func(a, b flowi.RequestTarget) int {
		if a.Collection != b.Collection {
			return compareIDs(string(a.Collection), string(b.Collection))
		}
		return compareIDs(string(a.Request), string(b.Request))
	})
	for _, t := range targets {
		out = append(out, p.run[t]...)
	}
	return out
}

func compareIDs(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// ForFile lists the problems of one file.
func (p *Problems) ForFile(file string) []Problem {
	var out []Problem
	for _, it := range p.items {
		if it.File == file {
			out = append(out, it)
		}
	}
	return out
}

// Summary counts errors and warnings: "2 errors, 1 warning".
func (p *Problems) Summary() string {
	var errs, warns int
	for _, it := range p.Items() {
		if it.Severity == "error" {
			errs++
		} else {
			warns++
		}
	}
	return plural(errs, "error") + ", " + plural(warns, "warning")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
