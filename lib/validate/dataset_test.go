// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestDataset_should_report_what_is_wrong_with_a_version(t *testing.T) {
	ok := model.Dataset{Driver: "postgres", Server: "postgres://u:p@h/db", Query: "SELECT id FROM t WHERE id = $1", Params: []string{"{{var.id}}"}}
	with := func(change func(*model.Dataset)) model.RequestVersion {
		d := ok
		change(&d)
		return model.RequestVersion{Label: "1", Dataset: &d}
	}
	tests := []struct {
		name string
		v    model.RequestVersion
		want diag.Code // "" → no diagnostic
		path string
	}{
		{name: "valid", v: with(func(*model.Dataset) {})},
		{name: "templated_server", v: with(func(d *model.Dataset) { d.Server = "{{env.db}}" })},
		{name: "sqlite_file", v: with(func(d *model.Dataset) { d.Driver, d.Server = "sqlite", "shop.db" })},
		{name: "unknown_driver", v: with(func(d *model.Dataset) { d.Driver = "oracle" }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.driver"},
		{name: "no_server", v: with(func(d *model.Dataset) { d.Server = "" }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.server"},
		{name: "no_query", v: with(func(d *model.Dataset) { d.Query = "  " }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.query"},
		{name: "writing_query", v: with(func(d *model.Dataset) { d.Query = "DELETE FROM t" }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.query"},
		{name: "two_statements", v: with(func(d *model.Dataset) { d.Query = "SELECT 1; SELECT 2" }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.query"},
		{name: "template_in_the_query", v: with(func(d *model.Dataset) { d.Query = "SELECT * FROM t WHERE id = {{var.id}}" }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.query"},
		{name: "negative_limit", v: with(func(d *model.Dataset) { d.Limit = -1 }), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset.limit"},
		{name: "unknown_template_in_a_param", v: with(func(d *model.Dataset) { d.Params = []string{"{{"} }), want: diag.CodeTemplateSyntax, path: "$.versions.1.dataset.params[0]"},
		{name: "also_grpc", v: func() model.RequestVersion {
			v := with(func(*model.Dataset) {})
			v.GRPC = &model.GRPC{}
			return v
		}(), want: diag.CodeDatasetRequest, path: "$.versions.1.dataset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Requests = append(c.Requests, model.Request{
				Meta: model.Meta{ID: "01J9ZA3M4N5P6Q7R8S9T0VAAAA", Name: "r"}, Path: "r", File: "r.req.yaml", Versions: []model.RequestVersion{tt.v},
			})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want == "" {
				for _, d := range got {
					t.Fatalf("unexpected %v", d)
				}
				return
			}
			if d := only(t, got, tt.want); d.Path != tt.path {
				t.Fatalf("path %q, want %q", d.Path, tt.path)
			}
		})
	}
}
