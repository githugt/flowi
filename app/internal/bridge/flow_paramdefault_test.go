// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	"github.com/githugt/flowi/lib/model"
)

func TestParams_Given_a_choice_that_is_not_set_Then_the_view_says_which_one_applies(t *testing.T) {
	// Before: the select of a choice such as a loop's layout was blank when the field was absent, so it looked as if
	// nothing applied; the step's own default (stop: passed) was only in the help text.
	c := vmtest.SampleCollection()
	c.Flows = []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Ends"}, Path: "ends", File: "/c/flows/ends.arazzo.yaml", Nodes: []model.Node{
		{ID: "end", Type: "stop"},
	}}}
	pub := &published{}
	app := bridge.NewApp(&vmtest.Backend{Cols: []model.Collection{c}}, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	flow, _ := findNode(app.Workspace().Tree().Roots, "flow")
	v, err := app.Flows().Open(flow.ID)
	if err != nil {
		t.Fatal(err)
	}

	app.Flows().Select(v.Node, "end")
	step := lastView(pub).Form.Step

	var status *bridge.Param
	for i := range step.Params {
		if step.Params[i].Name == "status" {
			status = &step.Params[i]
		}
	}
	if status == nil || status.Default != "passed" || status.Value != "" {
		t.Fatalf("status param %+v", status)
	}
}

// Before: the flow's Code tab showed only ids and paths, so it did not say what a step was for.
func TestSource_Given_request_steps_Then_each_has_a_comment_made_from_its_request_and_others_have_none(t *testing.T) {
	flows, v, _ := openTwoUnits(t)

	got, err := flows.Source(v.Node)

	if err != nil || got.Comments["login"] != "Login — POST /login" || got.Comments["verify"] != "Verify — POST /verify" {
		t.Fatalf("comments %v (%v)", got.Comments, err)
	}
	if _, ok := got.Comments["wait"]; ok {
		t.Fatalf("a delay step has a comment: %v", got.Comments)
	}
	if strings.Contains(got.Flow, "Login —") {
		t.Fatalf("the comment is in the text:\n%s", got.Flow)
	}
}
