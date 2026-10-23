// SPDX-License-Identifier: Apache-2.0

package sites_test

import (
	"testing"

	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/model"
)

func TestNodeTemplates_should_carry_the_owning_node_id(t *testing.T) {
	n := model.Node{ID: "x", Type: "set", Params: []model.Field{
		{Key: "target", Value: "vars.y"}, {Key: "value", Value: "{{nodes.a.b}}"},
	}}

	got := sites.NodeTemplates(sites.Located[model.Node]{File: "f.yaml", Path: "$.nodes[0]", Value: n})

	if len(got) != 1 || got[0].Node != "x" || got[0].Src != "{{nodes.a.b}}" {
		t.Fatalf("got %+v, want one site owned by x", got)
	}
}

func TestFlowTemplates_should_have_no_owning_node(t *testing.T) {
	f := model.Flow{File: "f.yaml", Outputs: []model.KeyValue{{Key: "token", Value: "{{nodes.a.b}}"}}}

	got := sites.FlowTemplates(f)

	if len(got) != 1 || got[0].Node != "" {
		t.Fatalf("got %+v, want one flow-level site", got)
	}
}

func TestResolvedVersion_Given_pin_prefer_default_Then_pin_wins(t *testing.T) {
	r := model.Request{Default: "1", Versions: []model.RequestVersion{{Label: "1"}, {Label: "2"}, {Label: "3"}}}

	pinned, _ := sites.ResolvedVersion(r, "3", "2")
	preferred, _ := sites.ResolvedVersion(r, "", "2")
	byDefault, _ := sites.ResolvedVersion(r, "", "9")

	if pinned.Label != "3" || preferred.Label != "2" || byDefault.Label != "1" {
		t.Fatalf("got %s/%s/%s, want 3/2/1", pinned.Label, preferred.Label, byDefault.Label)
	}
}

func TestWrittenVars_should_include_nested_writers(t *testing.T) {
	list := []model.Node{{ID: "p", Type: "parallel", Branches: []model.Branch{{ID: "b", Nodes: []model.Node{
		{ID: "g", Type: "generate", Params: []model.Field{{Key: "city", Value: "{{$city-name}}"}}},
	}}}}}

	got := sites.WrittenVars(list)

	if !got["city"] {
		t.Fatalf("got %v, want city", got)
	}
}
