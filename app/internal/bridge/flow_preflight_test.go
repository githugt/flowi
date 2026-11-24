// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// Before: a missing variable or secret was found in the middle of a run, after the earlier steps had made their requests.
func TestFlowServicePreflight_should_say_what_the_environment_lacks_before_the_run(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{{ID: "a", Type: "log"}}}}
	b := &vmtest.Backend{Cols: []model.Collection{c}, Needs: []flowi.Need{{Kind: flowi.NeedSecretValue, Name: "token"}}}
	b.Values = map[string]any{"defaults.environment": "staging"}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}

	lines, err := app.Flows().Preflight(v.Node)

	if err != nil || !slices.Equal(lines, []string{"The secret token has no value in staging."}) || !b.Called("PreflightFlow") {
		t.Fatalf("lines %q err %v calls %v", lines, err, b.CallLog())
	}
}

func TestFlowServicePreflight_Given_nothing_is_missing_Then_the_list_is_empty_not_null(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, _ := app.Flows().Open(flow.ID)

	lines, err := app.Flows().Preflight(v.Node)

	if err != nil || lines == nil || len(lines) != 0 {
		t.Fatalf("lines %#v err %v", lines, err)
	}
}

// Before: a run in a protected environment asked a generic question; the window could not say which steps write.
func TestFlowServiceWrites_should_name_the_steps_that_change_data_and_be_empty_not_null_for_a_read_only_flow(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.arazzo.yaml", Nodes: []model.Node{{ID: "a", Type: "log"}}}}
	b := &vmtest.Backend{Cols: []model.Collection{c}, FlowWritesOf: []flowi.FlowWrite{{Step: "login", Request: "auth/login", What: "POST"}}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, _ := app.Flows().Open(flow.ID)

	lines, err := app.Flows().Writes(v.Node)
	b.Mu.Lock()
	b.FlowWritesOf = nil
	b.Mu.Unlock()
	none, errNone := app.Flows().Writes(v.Node)

	if err != nil || errNone != nil || !slices.Equal(lines, []string{"login: POST (auth/login)"}) || none == nil || len(none) != 0 || !b.Called("FlowWrites") {
		t.Fatalf("lines %q %v, none %#v %v", lines, err, none, errNone)
	}
}
