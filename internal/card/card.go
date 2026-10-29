// SPDX-License-Identifier: AGPL-3.0-or-later

// Package card turns a flow step into what its card on the canvas shows: a
// badge, a sub-line saying what the step does, at most four summary rows, and
// its ports. It is pure: the front end draws what it returns and never reads a
// step's fields itself. What each card shows is set out in
// docs/design/2026-10-26-flow-card-contents.md.
package card

import (
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/model"
)

const (
	// maxRows is the most summary rows a card shows; the rest wait under More.
	maxRows = 4
	// cutAt is the longest a sub-line or a summary gets, in runes.
	cutAt = 40
	// portLabelAt is the longest a port label gets, in runes.
	portLabelAt = 24
)

// Row is one summary row. Key says which part of the step it stands for
// (inputs, hooks, settings, send, receive, connection, values, checks, steps,
// message); Editor names the editor that opens in place when the row is
// clicked; Icon names its icon. Rows carry no label word: the name appears on
// hover and as the heading of the open editor.
type Row struct {
	Key     string `json:"key"`
	Icon    string `json:"icon"`
	Summary string `json:"summary"`
	Editor  string `json:"editor"`
}

// Port is a control port: where a step's flow continues. Tone is done, fail,
// case or else.
type Port struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Tone  string `json:"tone"`
}

// Card is what one step shows. More lists the rows the person can add or open
// (empty parts), plus any filled row that did not fit under the four.
type Card struct {
	Badge   string `json:"badge"`
	Subline string `json:"subline"`
	Rows    []Row  `json:"rows"`
	More    []Row  `json:"more"`
	Ports   []Port `json:"ports"`
}

// Facts are the things about a step that live outside the step itself: for a
// request step, what its version holds; for a flow step, the flow it runs.
// The zero value is fine for the other steps.
type Facts struct {
	// Kind is the request kind: http, graphql, grpc, ws, sse, mqtt, nats,
	// kafka, amqp, amqp10, redis, sns, sqs, pubsub or servicebus.
	Kind string
	// Target is the path, operation, topic, queue or subject; for a flow step,
	// the flow's name.
	Target string
	// Method is the HTTP method, or the action word the version chose (publish,
	// subscribe, unary, query…). Empty picks the kind's word from Sends and
	// Receives.
	Method          string
	Sends, Receives bool

	Query, Variables, Headers int
	Body                      string // "JSON", "text", "1 message"…
	Auth                      string // "Bearer", "Basic", or the saved auth's name
	Props                     int    // properties, attributes or metadata sent with a message
	SendExtras                []string
	ReceiveCount              int
	ReceiveWait               string
	ReceiveExtras             []string

	PreHooks, PostHooks int
	Retry               int
	Timeout             string
	Region              string

	// Returns is the number of values a flow step returns.
	Returns int
}

// Summarize is the card for node n.
func Summarize(n model.Node, f Facts) Card {
	var c Card
	if n.Type == "request" {
		c = summarizeRequest(n, f)
	} else {
		c = summarizeStep(n, f)
	}
	return finish(c)
}

// finish cuts long text, keeps at most maxRows rows, and never returns nil
// slices (the front end iterates them).
func finish(c Card) Card {
	c.Subline = cut(c.Subline, cutAt)
	for i := range c.Rows {
		c.Rows[i].Summary = cut(c.Rows[i].Summary, cutAt)
	}
	if len(c.Rows) > maxRows {
		c.More = append(append([]Row(nil), c.Rows[maxRows:]...), c.More...)
		c.Rows = c.Rows[:maxRows]
	}
	if c.Rows == nil {
		c.Rows = []Row{}
	}
	if c.More == nil {
		c.More = []Row{}
	}
	if c.Ports == nil {
		c.Ports = []Port{}
	}
	return c
}

// cut shortens s to at most limit runes, ending in an ellipsis when it cut.
func cut(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit-1]) + "…"
}

func donePorts() []Port {
	return []Port{{ID: "done", Label: "Done", Tone: "done"}, {ID: "fail", Label: "Fail", Tone: "fail"}}
}

// join joins the non-empty parts with a middle dot.
func join(parts ...string) string {
	var kept []string
	for _, p := range parts {
		if p != "" {
			kept = append(kept, p)
		}
	}
	return strings.Join(kept, " · ")
}

// count is "1 header" or "2 headers", and "" for zero (a default part is not
// shown).
func count(n int, one, many string) string {
	switch {
	case n <= 0:
		return ""
	case n == 1:
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
