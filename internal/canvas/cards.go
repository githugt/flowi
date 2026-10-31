// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"strconv"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/card"
)

// Card sizes in canvas units. A card is a header, a sub-line, one 34-unit row
// for each summary row, and some padding; rows are 34 so they read at 13px or
// more without a border between them.
const (
	cardW        = 280
	CardW        = cardW // exported for hit-testing and tests
	cardHeaderH  = 46
	cardSublineH = 28
	cardRowH     = 34
	cardPad      = 12
	cardPortGap  = 24                // between the control ports on a card's right edge
	cardInPort   = cardHeaderH / 2   // the in port and the first out port sit on the header's middle line
	KindCard     = "card"            // one step drawn as a card (see WithCards)
	portDone     = "done"            // the port the flow continues from
	portFail     = "fail"            // the port a failing step's on_error route leaves from
	portElse     = "else"            // a route's last port
	portCasePref = "case-"           // a route's case ports are case-0, case-1…
	cardMinH     = cardInPort + 2*12 // no card is shorter than its in port plus room
)

// WithCards draws every step as a card sized from its summary (see
// internal/card): one unit for each step, containers as a card with their lanes
// to its right, edges leaving from the port that matches the outcome. Nothing is
// drawn as a strip or a block of attached actions.
func WithCards(cards map[string]card.Card) Option {
	return func(l *layouter) {
		l.cards = cards
		l.branches = true
		l.attached = false
	}
}

// cardHeight is how tall a card is: its header, sub-line and rows, and tall
// enough for every control port to be reachable.
func cardHeight(c card.Card) int64 {
	h := int64(cardHeaderH + cardSublineH + len(c.Rows)*cardRowH + cardPad)
	return max(h, int64(cardInPort+len(c.Ports)*cardPortGap), cardMinH)
}

// cardGroups makes one unit of each node.
func cardGroups(nodes []model.Node) []groupSpan {
	out := make([]groupSpan, len(nodes))
	for i, n := range nodes {
		kind := KindCard
		if unitKind(n) == KindContainer {
			kind = KindContainer
		}
		out[i] = groupSpan{start: i, end: i + 1, kind: kind}
	}
	return out
}

// cardPorts are the ids of a card's ports, top to bottom.
func cardPorts(c card.Card) []string {
	ids := make([]string, len(c.Ports))
	for i, p := range c.Ports {
		ids[i] = p.ID
	}
	return ids
}

// PortYOf is the y of the unit's out port id; the in port's y for "" or a port
// the unit does not have.
func (u Unit) PortYOf(id string) int64 {
	for i, p := range u.Ports {
		if p == id {
			return u.Y + cardInPort + int64(i)*cardPortGap
		}
	}
	return u.PortY()
}

// laneFeed is the card port that takes a condition's lane: case-i for the
// cases (case-0 for a condition with only a check), else for the else lane.
func laneFeed(n model.Node, label string, i int) string {
	if n.Type != "condition" {
		return ""
	}
	if label == "else" {
		return portElse
	}
	if len(n.Cases) == 0 {
		return portCasePref + "0"
	}
	return portCasePref + strconv.Itoa(i)
}

// portEdges names the port each edge leaves from: fail for an on_error route,
// done for the rest, on cards. Edges into a lane keep the lane's own feed port.
func portEdges(d *Diagram) {
	for i, e := range d.Edges {
		from, ok := d.Unit(e.From)
		if !ok || !from.Card || e.Into {
			continue
		}
		d.Edges[i].FromPort = portDone
		switch {
		case e.Outcome != "":
			d.Edges[i].FromPort = "outcome-" + e.Outcome
		case e.Fail:
			d.Edges[i].FromPort = portFail
		}
	}
}
