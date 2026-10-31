// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/model"
)

// UsedByRow is one line of the "Used by" tab: a version header, or a
// consumer with its first failing read.
type UsedByRow struct {
	Header     string // set on version rows
	Text       string // "✓ flow signup › node login"
	Detail     string // the first read that is not compatible
	Opens      bool   // a flow node the tab can open
	Collection model.ID
	Flow       string // flow path
	Node       string
}

// UsedBy is the request screen's "Used by" tab (design §9): every
// consumer of the request with ✓/⚠/✗, from impact analysis.
type UsedBy struct {
	b        Backend
	w        *Worker
	target   flowi.RequestTarget
	rows     []UsedByRow
	err      string
	onChange []func()
}

// NewUsedBy returns the tab's view-model; call Load.
func NewUsedBy(b Backend, w *Worker, t flowi.RequestTarget) *UsedBy {
	return &UsedBy{b: b, w: w, target: t}
}

// OnChange registers f to run after a load.
func (u *UsedBy) OnChange(f func()) { u.onChange = append(u.onChange, f) }

// Rows are the tab's lines.
func (u *UsedBy) Rows() []UsedByRow { return u.rows }

// Err is the last problem, or "".
func (u *UsedBy) Err() string { return u.err }

// Load runs impact analysis in the background.
func (u *UsedBy) Load() {
	target := u.target
	type loaded struct {
		rep analysis.Report
		err error
	}
	Run(u.w, func() loaded {
		rep, _, err := u.b.Impact(target, "")
		return loaded{rep, err}
	}, func(l loaded) {
		u.err, u.rows = "", nil
		if l.err != nil {
			u.err = ErrorText(l.err)
		} else {
			u.rows = usedByRows(l.rep)
		}
		for _, f := range u.onChange {
			f()
		}
	})
}

func usedByRows(rep analysis.Report) []UsedByRow {
	var out []UsedByRow
	for _, v := range rep.Versions {
		source := v.Contract.Source
		if source == "" {
			source = "none"
		}
		out = append(out, UsedByRow{Header: "version " + v.Version + " · contract: " + source})
		for _, c := range v.Consumers {
			out = append(out, consumerRow(c))
		}
	}
	return out
}

func consumerRow(c analysis.Consumer) UsedByRow {
	row := UsedByRow{Text: statusMark(c.Status) + " " + consumerText(c), Collection: c.CollectionID, Flow: c.Flow, Node: c.Node,
		Opens: c.Kind == analysis.KindNode && c.Flow != ""}
	for _, r := range c.Reads {
		if r.Status != analysis.Compatible {
			row.Detail = r.Expr + " → " + r.Target + ": " + r.Reason
			break
		}
	}
	return row
}

func consumerText(c analysis.Consumer) string {
	switch {
	case c.Kind == analysis.KindRequest:
		return "request extract + assert"
	case c.Kind == analysis.KindHook:
		return "hook " + c.Hook
	case c.Flow != "":
		return "flow " + c.Flow + " › node " + c.Node
	}
	return strings.TrimSpace("hook " + c.Hook + " › node " + c.Node)
}

func statusMark(s analysis.Status) string {
	switch s {
	case analysis.Broken:
		return "✗"
	case analysis.Unknown:
		return "⚠"
	}
	return "✓"
}
