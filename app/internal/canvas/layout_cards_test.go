// SPDX-License-Identifier: AGPL-3.0-or-later

package canvas

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/card"
)

func cardWithRows(n int) card.Card {
	c := card.Card{Badge: "GET", Subline: "/x", Ports: []card.Port{{ID: "done", Label: "Done", Tone: "done"}, {ID: "fail", Label: "Fail", Tone: "fail"}}}
	for range n {
		c.Rows = append(c.Rows, card.Row{Key: "inputs", Summary: "x"})
	}
	return c
}

func TestLayout_Given_a_card_with_four_rows_Then_the_unit_height_matches_the_row_count(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "a", Type: "request"}, {ID: "b", Type: "request"}}}
	cards := map[string]card.Card{"a": cardWithRows(4), "b": cardWithRows(0)}

	d := Layout(f, nil, nil, WithCards(cards))

	a, _ := d.Unit("a")
	b, _ := d.Unit("b")
	if want := int64(cardHeaderH + cardSublineH + 4*cardRowH + cardPad); a.H != want || a.W != cardW {
		t.Fatalf("a is %dx%d, want %dx%d", a.W, a.H, cardW, want)
	}
	if want := int64(cardHeaderH + cardSublineH + cardPad); b.H != want {
		t.Fatalf("b is %d high, want %d", b.H, want)
	}
	if a.Kind != KindCard || b.Kind != KindCard {
		t.Fatalf("kinds %s %s, want %s", a.Kind, b.Kind, KindCard)
	}
}

func TestLayout_Given_cards_Then_every_step_is_its_own_unit_even_when_actions_could_share_a_strip(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "a", Type: "delay"}, {ID: "b", Type: "log"}, {ID: "c", Type: "note"}}}

	d := Layout(f, nil, nil, WithCards(map[string]card.Card{"a": cardWithRows(0), "b": cardWithRows(0), "c": cardWithRows(0)}))

	if len(d.Units) != 3 {
		t.Fatalf("%d units, want 3", len(d.Units))
	}
}

func routeCard(cases int) card.Card {
	c := card.Card{Subline: "by status"}
	for i := range cases {
		c.Ports = append(c.Ports, card.Port{ID: "case-" + string(rune('0'+i)), Label: "c", Tone: "case"})
	}
	c.Ports = append(c.Ports, card.Port{ID: "else", Label: "else", Tone: "else"})
	return c
}

func TestLayout_Given_a_route_with_four_cases_Then_an_edge_leaves_from_each_case_port(t *testing.T) {
	var cases []model.Case
	for _, id := range []string{"w", "x", "y", "z"} {
		cases = append(cases, model.Case{When: "nodes.a.status == 1", Nodes: []model.Node{{ID: id, Type: "log"}}})
	}
	f := model.Flow{Nodes: []model.Node{{ID: "r", Type: "condition", Cases: cases}}}
	cards := map[string]card.Card{"r": routeCard(4)}
	for _, id := range []string{"w", "x", "y", "z"} {
		cards[id] = cardWithRows(0)
	}

	d := Layout(f, nil, nil, WithCards(cards))

	r, _ := d.Unit("r")
	if want := []string{"case-0", "case-1", "case-2", "case-3", "else"}; len(r.Ports) != len(want) {
		t.Fatalf("ports %v, want %v", r.Ports, want)
	}
	if len(d.Lanes) != 5 || d.Lanes[4].Port != "else" {
		t.Fatalf("lanes %+v, want four cases and else", d.Lanes)
	}
	seen := map[int64]bool{}
	for i, lane := range d.Lanes[:4] {
		want := "case-" + string(rune('0'+i))
		if lane.Port != want {
			t.Fatalf("lane %d is fed by %q, want %q", i, lane.Port, want)
		}
		seen[r.PortYOf(lane.Port)] = true
	}
	if len(seen) != 4 {
		t.Fatalf("the four case ports are at %d distinct heights", len(seen))
	}
}

func TestEdge_Given_done_and_fail_ports_Then_the_fail_edge_starts_below_the_done_edge(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "a", Type: "request", OnError: "goto:c"},
		{ID: "b", Type: "log"},
		{ID: "c", Type: "log"},
	}}
	cards := map[string]card.Card{"a": cardWithRows(1), "b": cardWithRows(0), "c": cardWithRows(0)}

	d := Layout(f, nil, nil, WithCards(cards))

	var done, fail Edge
	for _, e := range d.Edges {
		switch {
		case e.From == "a" && e.Fail:
			fail = e
		case e.From == "a" && e.To == "b":
			done = e
		}
	}
	if done.FromPort != "done" || fail.FromPort != "fail" {
		t.Fatalf("ports %q and %q", done.FromPort, fail.FromPort)
	}
	if doneY, failY := EdgePath(d, done)[0].Y, EdgePath(d, fail)[0].Y; failY <= doneY {
		t.Fatalf("fail starts at %d, done at %d: fail must start below", failY, doneY)
	}
}

func TestLayout_Given_a_route_with_twelve_cases_Then_the_card_is_tall_enough_for_every_port(t *testing.T) {
	var cases []model.Case
	for range 12 {
		cases = append(cases, model.Case{When: "x"})
	}
	f := model.Flow{Nodes: []model.Node{{ID: "r", Type: "condition", Cases: cases}}}

	d := Layout(f, nil, nil, WithCards(map[string]card.Card{"r": routeCard(12)}))

	r, _ := d.Unit("r")
	last := r.PortYOf("else")
	if last > r.Y+r.H {
		t.Fatalf("the else port is at %d, below the card's bottom %d", last, r.Y+r.H)
	}
}

func TestLayout_Given_a_container_card_Then_its_own_height_is_known_apart_from_its_lanes(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "r", Type: "condition", Cases: []model.Case{{When: "x", Nodes: []model.Node{{ID: "a", Type: "log"}, {ID: "b", Type: "log"}}}}}}}

	d := Layout(f, nil, nil, WithCards(map[string]card.Card{"r": routeCard(1), "a": cardWithRows(0), "b": cardWithRows(0)}))

	r, _ := d.Unit("r")
	if r.HeadH != cardHeight(routeCard(1)) || r.HeadW != cardW {
		t.Fatalf("head is %dx%d", r.HeadW, r.HeadH)
	}
}
