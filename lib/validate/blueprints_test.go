// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func (f *fixture) blueprint(c *model.Collection, path string, params []model.Param, nodes ...model.Node) model.Blueprint {
	bp := model.Blueprint{
		Meta: model.Meta{ID: f.ids.New(), Name: path}, Path: path, Params: params,
		File: c.Root + "/blueprints/" + path + ".blueprint.yaml", Body: model.Flow{Nodes: nodes},
	}
	c.Blueprints = append(c.Blueprints, bp)
	return bp
}

func paramNode(id, param string) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{Param: param}}}
}

func TestValidate_should_accept_a_well_formed_blueprint(t *testing.T) {
	f := newFixture(t)
	c := f.collection("app")
	f.blueprint(&c, "standard", []model.Param{{Name: "api", Type: model.ParamRequestRef}}, paramNode("call", "api"))

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("unexpected %v", codesOf(got))
	}
}

func TestValidate_should_check_blueprints_and_point_at_their_file(t *testing.T) {
	f := newFixture(t)
	c := f.collection("app")
	bp := f.blueprint(&c, "standard", nil, paramNode("call", "missing"))

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 1 || got[0].Code != diag.CodeBlueprintRef || got[0].File != bp.File {
		t.Fatalf("got %+v", got)
	}
}

func TestDuplicateIDs_should_include_blueprints(t *testing.T) {
	f := newFixture(t)
	c := f.collection("app")
	fl := f.flow(&c, "journey")
	bp := f.blueprint(&c, "standard", nil)
	c.Blueprints[0].ID = fl.ID

	got := Validate(Target{Collections: []model.Collection{c}})

	found := false
	for _, d := range got {
		found = found || d.Code == diag.CodeIDDuplicate
	}
	if !found {
		t.Fatalf("a blueprint sharing a flow's id went unnoticed: %v (%s)", codesOf(got), bp.Path)
	}
}
