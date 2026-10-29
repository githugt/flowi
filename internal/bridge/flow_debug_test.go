// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"
)

func TestToggleBreak_Given_a_step_Then_the_view_lists_it_and_its_card_shows_the_breakpoint(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)

	// Act
	flows.ToggleBreak(v.Node, "verify")
	got := lastView(pub)

	// Assert
	if !slices.Equal(got.Debug.Breaks, []string{"verify"}) || !got.Cards["verify"].Break || got.Cards["login"].Break {
		t.Fatalf("debug %+v verify %+v login %+v", got.Debug, got.Cards["verify"], got.Cards["login"])
	}
	flows.ToggleBreak(v.Node, "verify")
	if len(lastView(pub).Debug.Breaks) != 0 {
		t.Errorf("a second toggle clears it")
	}
}

func TestToggleBreak_Given_a_step_that_is_not_in_the_flow_Then_nothing_is_added(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)

	// Act
	flows.ToggleBreak(v.Node, "ghost")
	flows.ToggleBreak(v.Node, "login")

	// Assert
	if got := lastView(pub).Debug.Breaks; !slices.Equal(got, []string{"login"}) {
		t.Fatalf("breaks %v", got)
	}
}
