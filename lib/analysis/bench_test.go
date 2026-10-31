// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"fmt"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// noContracts makes the benchmark measure analysis, not file reads.
type noContracts struct{}

func (noContracts) Contract(model.Collection, model.Request, string) (Contract, diag.List) {
	return Contract{}, nil
}

// syntheticWorkspace is a large collection: requests × 2 versions, and
// flows of nodes that each call a request and read it in the next node.
func syntheticWorkspace(requests, flows, nodesPerFlow int) []model.Collection {
	c := model.Collection{Root: "/w", Manifest: model.Manifest{Meta: model.Meta{ID: "01JD0000000000000000000001", Name: "big"}, File: "/w/flowi.yaml",
		Hooks: model.Hooks{Post: []model.HookStep{{Inline: []model.Node{{ID: "h", Type: "log", Params: []model.Field{{Key: "value", Value: "{{response.body.id}}"}}}}}}}}}
	for i := range requests {
		r := model.Request{Meta: model.Meta{ID: model.ID(fmt.Sprintf("R%025d", i))}, Path: fmt.Sprintf("r%d", i), File: fmt.Sprintf("/w/requests/r%d.req.yaml", i)}
		for _, l := range []string{"1", "2"} {
			r.Versions = append(r.Versions, model.RequestVersion{Label: l, Method: "POST", URL: "{{env.base}}/x", Body: model.Body{JSON: `{"a": "{{a}}"}`},
				Extract: []model.Extraction{{Name: "token", From: "body", Path: "$.token"}}})
		}
		c.Requests = append(c.Requests, r)
	}
	for f := range flows {
		fl := model.Flow{Meta: model.Meta{ID: model.ID(fmt.Sprintf("F%025d", f))}, Path: fmt.Sprintf("f%d", f), File: fmt.Sprintf("/w/flows/f%d.flow.yaml", f)}
		for n := 0; n < nodesPerFlow; n += 2 {
			r := c.Requests[(f*nodesPerFlow+n)%requests]
			id := fmt.Sprintf("n%d", n)
			fl.Nodes = append(fl.Nodes,
				model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: r.ID}, With: []model.KeyValue{{Key: "a", Value: "1"}}}},
				model.Node{ID: id + "r", Type: "log", Params: []model.Field{{Key: "value", Value: "{{nodes." + id + ".token}} {{nodes." + id + ".body.x.y}}"}}})
		}
		c.Flows = append(c.Flows, fl)
	}
	return []model.Collection{c}
}

func BenchmarkAnalyze(b *testing.B) {
	cs := syntheticWorkspace(500, 200, 40)
	b.ResetTimer()
	for range b.N {
		Analyze(cs, noContracts{})
	}
}
