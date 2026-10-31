// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestValidate_Given_bad_template_Then_FLW_TPL_002_at_field_position(t *testing.T) {
	tests := []struct {
		name string
		edit func(c *model.Collection)
		path string
	}{
		{"url", func(c *model.Collection) { c.Requests[0].Versions[0].URL = "http://x/{{ a + }}" }, "$.versions.1.url"},
		{"header", func(c *model.Collection) {
			c.Requests[0].Versions[0].Headers = []model.KeyValue{{Key: "X-A", Value: "{{ unclosed"}}
		}, "$.versions.1.headers.X-A"},
		{"json body", func(c *model.Collection) { c.Requests[0].Versions[0].Body.JSON = `{"a":"{{ ( }}"}` }, "$.versions.1.body.json"},
		{"assert", func(c *model.Collection) { c.Requests[0].Versions[0].Assert = []string{"status =="} }, "$.versions.1.assert[0]"},
		{"node with", func(c *model.Collection) {
			n := requestNode("call", c.Requests[0])
			n.Request.With = []model.KeyValue{{Key: "email", Value: "{{ a + }}"}}
			c.Flows = append(c.Flows, model.Flow{Meta: model.Meta{ID: "01JA000000000000000000FL01", Name: "f"}, Path: "f", File: c.Root + "/flows/f.flow.yaml", Nodes: []model.Node{n}})
		}, "$.nodes[0].with.email"},
		{"node when", func(c *model.Collection) {
			n := model.Node{ID: "l", Type: "log", When: "a ==", Params: []model.Field{{Key: "value", Value: "x"}}}
			c.Flows = append(c.Flows, model.Flow{Meta: model.Meta{ID: "01JA000000000000000000FL02", Name: "f"}, Path: "f", File: c.Root + "/flows/f.flow.yaml", Nodes: []model.Node{n}})
		}, "$.nodes[0].when"},
		{"set value", func(c *model.Collection) {
			c.Manifest.Hooks.Pre = []model.HookStep{{Inline: []model.Node{{ID: "s", Type: "set", Params: []model.Field{{Key: "target", Value: "vars.x"}, {Key: "value", Value: "{{ * }}"}}}}}}
		}, "$.hooks.pre[0].inline[0].value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.request(&c, "r", "1")
			tt.edit(&c)

			got := Validate(Target{Collections: []model.Collection{c}})

			d := only(t, got, diag.CodeTemplateSyntax)
			if d.Path != tt.path || d.Args["detail"] == "" {
				t.Fatalf("diag %+v, want path %s", d, tt.path)
			}
		})
	}
}
