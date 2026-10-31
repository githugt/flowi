// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"
)

func TestFlowService_ConvertStep_Given_a_preview_Then_it_names_what_would_be_lost_and_changes_nothing(t *testing.T) {
	// Arrange
	flows, v, pub := openSignup(t)
	flows.Select(v.Node, "b")

	// Act
	lost, err := flows.ConvertStep(v.Node, "b", "delay", "", false)

	// Assert
	if err != nil || len(lost) != 1 || lost[0] != "value" {
		t.Fatalf("lost %v err %v", lost, err)
	}
	if got := lastViewOr(pub, v).Form.Step; got != nil && got.Type != "log" {
		t.Fatalf("a preview changed the step to %q", got.Type)
	}
}

func TestFlowService_ConvertStep_Given_apply_Then_the_kind_changes_and_undo_restores_it(t *testing.T) {
	// Arrange
	flows, v, pub := openSignup(t)
	flows.Select(v.Node, "b")

	// Act
	_, err := flows.ConvertStep(v.Node, "b", "delay", "", true)
	changed := lastView(pub).Form.Step.Type
	flows.Undo(v.Node)
	restored := lastView(pub).Form.Step.Type

	// Assert
	if err != nil || changed != "delay" || restored != "log" {
		t.Fatalf("err %v changed %q restored %q", err, changed, restored)
	}
}

func TestFlowService_ConvertStep_Given_an_unknown_step_Then_it_is_refused(t *testing.T) {
	flows, v, _ := openSignup(t)

	_, err := flows.ConvertStep(v.Node, "nope", "delay", "", true)

	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestFlowView_Steps_Then_the_popup_gets_the_ten_entries(t *testing.T) {
	_, v, _ := openSignup(t)

	if len(v.Steps) != 10 || v.Steps[0].Name != "Request" || v.Steps[0].Description == "" {
		t.Fatalf("steps %+v", v.Steps)
	}
}

func TestFlowService_SetColor_Given_a_colour_Then_the_card_carries_it_and_undo_takes_it_back(t *testing.T) {
	// Arrange
	flows, v, pub := openSignup(t)

	// Act
	err := flows.SetColor(v.Node, "b", "amber")
	painted := lastView(pub).Cards["b"].Color
	flows.Undo(v.Node)
	cleared := lastView(pub).Cards["b"].Color

	// Assert
	if err != nil || painted != "amber" || cleared != "" {
		t.Fatalf("err %v painted %q cleared %q", err, painted, cleared)
	}
}

func TestFlowService_SetColor_Given_an_unknown_colour_Then_it_is_refused(t *testing.T) {
	flows, v, _ := openSignup(t)

	if err := flows.SetColor(v.Node, "b", "chartreuse"); err == nil {
		t.Fatal("expected an error")
	}
}

func TestFlowService_Duplicate_Given_a_step_Then_the_copy_follows_it_and_is_selected(t *testing.T) {
	// Arrange
	flows, v, pub := openSignup(t)

	// Act
	id, err := flows.Duplicate(v.Node, "a")
	got := lastView(pub)

	// Assert
	if err != nil || id == "" || order(got) != "a,"+id+",b" || got.Selected != id {
		t.Fatalf("id %q err %v order %q selected %q", id, err, order(got), got.Selected)
	}
}

func TestFlowService_AddRouteByStatus_Then_a_condition_with_four_ports_follows_the_step_and_is_selected(t *testing.T) {
	// Arrange
	flows, v, pub := openTwoUnits(t)

	// Act
	id, err := flows.AddRouteByStatus(v.Node, "login")
	got := lastView(pub)
	var labels []string
	for _, p := range got.Cards[id].Face.Ports {
		labels = append(labels, p.Label)
	}

	// Assert
	if err != nil || got.Selected != id || strings.Join(labels, ",") != "== 200,== 404,5xx,else" {
		t.Fatalf("id %q err %v selected %q ports %v", id, err, got.Selected, labels)
	}
}

func TestFlowService_AddRouteByStatus_Given_undo_Then_it_is_gone(t *testing.T) {
	flows, v, pub := openTwoUnits(t)
	id, _ := flows.AddRouteByStatus(v.Node, "login")

	flows.Undo(v.Node)

	if _, still := lastView(pub).Cards[id]; still {
		t.Fatal("the condition should be gone after undo")
	}
}
