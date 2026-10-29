// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/bridge"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

// openWithIO opens a flow that declares three inputs and no outputs.
func openWithIO(t *testing.T) (*bridge.FlowService, bridge.FlowView, *published) {
	t.Helper()
	app, v, pub := openWithIOApp3(t)
	return app.Flows(), v, pub
}

// openWithIOApp is openWithIO that returns the app and not the publisher.
func openWithIOApp(t *testing.T) (*bridge.App, bridge.FlowView) {
	t.Helper()
	app, v, _ := openWithIOApp3(t)
	return app, v
}

func openWithIOApp3(t *testing.T) (*bridge.App, bridge.FlowView, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Io"}, Path: "io", File: "/c/flows/io.arazzo.yaml",
		Inputs: []model.Input{{Name: "email", Type: "string"}, {Name: "count", Type: "number"}, {Name: "debug"}},
		Nodes:  []model.Node{{ID: "a", Type: "log", Params: []model.Field{{Key: "value", Value: "hi"}}}},
	}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}
	return app, v, pub
}

func startRows(v bridge.FlowView) string {
	var rows []string
	for _, r := range v.Start.Rows {
		rows = append(rows, r.Summary)
	}
	return strings.Join(rows, ",")
}

func TestFlowView_Start_Then_it_lists_the_inputs_in_declared_order(t *testing.T) {
	_, v, _ := openWithIO(t)

	if got := startRows(v); got != "email · string,count · number,debug" {
		t.Fatalf("rows %q", got)
	}
	if v.End != nil {
		t.Fatalf("a flow with no outputs has no End card: %+v", v.End)
	}
}

func TestMoveInput_Then_the_file_order_changes_and_undo_restores_it(t *testing.T) {
	// Arrange
	flows, v, pub := openWithIO(t)

	// Act
	err := flows.MoveInput(v.Node, 2, 0)
	moved := startRows(lastView(pub))
	flows.Undo(v.Node)
	restored := startRows(lastView(pub))

	// Assert
	if err != nil || moved != "debug,email · string,count · number" || restored != "email · string,count · number,debug" {
		t.Fatalf("err %v moved %q restored %q", err, moved, restored)
	}
}

func TestMoveInput_Given_a_place_outside_the_list_Then_it_is_refused(t *testing.T) {
	flows, v, _ := openWithIO(t)

	if err := flows.MoveInput(v.Node, 0, 7); err == nil {
		t.Fatal("expected an error")
	}
	if err := flows.MoveInput(v.Node, -1, 0); err == nil {
		t.Fatal("expected an error")
	}
}

func TestAddOutput_Then_the_End_card_appears_and_undo_removes_it(t *testing.T) {
	// Arrange
	flows, v, pub := openWithIO(t)

	// Act
	name, err := flows.AddOutput(v.Node, "")
	drawn := lastView(pub).End != nil
	flows.Undo(v.Node)
	gone := lastView(pub).End == nil

	// Assert
	if err != nil || name == "" || !drawn || !gone {
		t.Fatalf("name %q err %v drawn %v gone %v", name, err, drawn, gone)
	}
}

func TestAddOutput_Given_the_name_is_taken_Then_a_free_one_is_made(t *testing.T) {
	flows, v, _ := openWithIO(t)
	first, _ := flows.AddOutput(v.Node, "token")

	second, err := flows.AddOutput(v.Node, "token")

	if err != nil || first != "token" || second == "token" || second == "" {
		t.Fatalf("first %q second %q err %v", first, second, err)
	}
}

func TestSetOutputValue_Then_the_row_shows_it(t *testing.T) {
	// Arrange
	flows, v, pub := openWithIO(t)
	name, _ := flows.AddOutput(v.Node, "token")

	// Act
	err := flows.SetOutputValue(v.Node, name, "{{vars.token}}")
	end := lastView(pub).End

	// Assert
	if err != nil || end == nil || end.Rows[0].Summary != "token = {{vars.token}}" {
		t.Fatalf("err %v end %+v", err, end)
	}
}

func TestSetInput_Given_a_type_and_name_Then_the_row_follows_and_a_bad_type_is_refused(t *testing.T) {
	// Arrange
	flows, v, pub := openWithIO(t)

	// Act
	err := flows.SetInput(v.Node, 2, "verbose", "boolean", "false")
	bad := flows.SetInput(v.Node, 2, "verbose", "colour", "")

	// Assert
	if err != nil || bad == nil {
		t.Fatalf("err %v bad %v", err, bad)
	}
	if got := startRows(lastView(pub)); !strings.HasSuffix(got, ",verbose · boolean") {
		t.Fatalf("rows %q", got)
	}
}

func TestRemoveInput_Then_the_row_goes(t *testing.T) {
	flows, v, pub := openWithIO(t)

	err := flows.RemoveInput(v.Node, 1)

	if got := startRows(lastView(pub)); err != nil || got != "email · string,debug" {
		t.Fatalf("err %v rows %q", err, got)
	}
}

func TestAddInput_Then_a_new_row_ends_the_list(t *testing.T) {
	flows, v, pub := openWithIO(t)

	name, err := flows.AddInput(v.Node, "")

	if got := startRows(lastView(pub)); err != nil || name == "" || !strings.HasSuffix(got, ","+name) {
		t.Fatalf("name %q err %v rows %q", name, err, got)
	}
}

func TestFlowView_Inputs_Then_the_popover_gets_name_type_and_default_in_order(t *testing.T) {
	_, v, _ := openWithIO(t)

	if len(v.Inputs) != 3 || v.Inputs[1] != (bridge.IOView{Name: "count", Type: "number"}) || len(v.Outputs) != 0 {
		t.Fatalf("inputs %+v outputs %+v", v.Inputs, v.Outputs)
	}
}
