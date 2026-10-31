// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"strings"
	"testing"
)

func TestToggleSkip_Given_a_step_read_later_Then_the_view_lists_the_skip_and_names_the_reader(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)
	_ = flows.ConnectData(v.Node, "login", "status", "verify", "with.code")

	// Act
	flows.ToggleSkip(v.Node, "login")
	got := lastView(pub).Skips

	// Assert
	if !slices.Equal(got.Steps, []string{"login"}) {
		t.Fatalf("steps %v", got.Steps)
	}
	if len(got.Problems) != 1 || got.Problems[0].Reader != "verify" || !strings.Contains(got.Problems[0].Text, "login") {
		t.Fatalf("problems %+v", got.Problems)
	}
}

func TestToggleSkip_Given_the_step_toggled_again_Then_nothing_is_skipped(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)

	// Act
	flows.ToggleSkip(v.Node, "wait")
	flows.ToggleSkip(v.Node, "wait")
	got := lastView(pub).Skips

	// Assert
	if len(got.Steps) != 0 || len(got.Problems) != 0 {
		t.Fatalf("skips %+v", got)
	}
}

func TestToggleSkip_Given_a_skipped_step_Then_its_card_says_so(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)
	_ = flows.ConnectData(v.Node, "login", "status", "verify", "with.code")

	// Act
	flows.ToggleSkip(v.Node, "login")
	cards := lastView(pub).Cards

	// Assert
	if !cards["login"].Skipped {
		t.Errorf("the skipped step's card should be marked skipped: %+v", cards["login"])
	}
	if got := cards["verify"].SkipNote; !strings.Contains(got, "login") {
		t.Errorf("the reader's card should say which step it waits on: %q", got)
	}
}
