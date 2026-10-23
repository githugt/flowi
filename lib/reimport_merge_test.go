// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// specReq is a request as a spec import makes it; change edits a copy.
func specReq(change func(r *model.Request)) model.Request {
	r := model.Request{Path: "pets/getPet", Meta: model.Meta{Name: "Get a pet"}, Versions: []model.RequestVersion{{
		Label: "1", Method: "GET", URL: "{{env.base_url}}/pets/{{petId}}", Docs: "Finds a pet.",
		Query: []model.KeyValue{{Key: "limit", Value: "{{limit}}"}}, Body: model.Body{JSON: `{}`},
	}}}
	r.Versions[0].Query = slices.Clone(r.Versions[0].Query)
	if change != nil {
		change(&r)
	}
	return r
}

func TestMergeReimport_should_keep_your_edits_and_take_the_specs_changes(t *testing.T) {
	tests := []struct {
		name      string
		noBase    bool
		ours      func(r *model.Request)
		theirs    func(r *model.Request)
		check     func(t *testing.T, got model.Request)
		wantCodes []diag.Code
	}{
		{
			name: "a field you changed keeps your value",
			ours: func(r *model.Request) { r.Versions[0].Docs = "mine" },
			check: func(t *testing.T, got model.Request) {
				if got.Versions[0].Docs != "mine" {
					t.Fatalf("docs %q", got.Versions[0].Docs)
				}
			},
		},
		{
			name:   "a field the spec changed takes the spec's value",
			theirs: func(r *model.Request) { r.Versions[0].URL = "{{env.base_url}}/animals/{{petId}}" },
			check: func(t *testing.T, got model.Request) {
				if got.Versions[0].URL != "{{env.base_url}}/animals/{{petId}}" {
					t.Fatalf("url %q", got.Versions[0].URL)
				}
			},
		},
		{
			name:   "a field both changed keeps yours and says so",
			ours:   func(r *model.Request) { r.Versions[0].Body.JSON = `{"mine":1}` },
			theirs: func(r *model.Request) { r.Versions[0].Body.JSON = `{"spec":1}` },
			check: func(t *testing.T, got model.Request) {
				if got.Versions[0].Body.JSON != `{"mine":1}` {
					t.Fatalf("body %q", got.Versions[0].Body.JSON)
				}
			},
			wantCodes: []diag.Code{diag.CodeImportKeptEdit},
		},
		{
			name: "a parameter value you set and one you added stay beside the spec's new one",
			ours: func(r *model.Request) {
				r.Versions[0].Query = []model.KeyValue{{Key: "limit", Value: "20"}, {Key: "page", Value: "2"}}
			},
			theirs: func(r *model.Request) {
				r.Versions[0].Query = append(r.Versions[0].Query, model.KeyValue{Key: "sort", Value: "{{sort}}"})
			},
			check: func(t *testing.T, got model.Request) {
				want := []model.KeyValue{{Key: "limit", Value: "20"}, {Key: "sort", Value: "{{sort}}"}, {Key: "page", Value: "2"}}
				if q := got.Versions[0].Query; !slices.Equal(q, want) {
					t.Fatalf("query %v, want %v", q, want)
				}
			},
		},
		{
			name: "a parameter you removed stays removed",
			ours: func(r *model.Request) { r.Versions[0].Query = nil },
			check: func(t *testing.T, got model.Request) {
				if q := got.Versions[0].Query; len(q) != 0 {
					t.Fatalf("query %v, want none", q)
				}
			},
		},
		{
			name: "what only Flow-I has is kept",
			ours: func(r *model.Request) {
				r.Versions[0].Assert = []string{"status == 200"}
				r.Versions[0].Extract = []model.Extraction{{Name: "id", From: "body", Path: "$.id"}}
				r.Hooks = model.Hooks{Pre: []model.HookStep{{Inline: []model.Node{{}}}}}
			},
			check: func(t *testing.T, got model.Request) {
				if len(got.Versions[0].Assert) != 1 || len(got.Versions[0].Extract) != 1 || len(got.Hooks.Pre) != 1 {
					t.Fatalf("got %+v", got)
				}
			},
		},
		{
			name:   "a version only you have is kept; one the spec dropped goes",
			ours:   func(r *model.Request) { r.Versions = append(r.Versions, model.RequestVersion{Label: "1.1-mine"}) },
			theirs: func(r *model.Request) { r.Versions[0].Label = "2" },
			check: func(t *testing.T, got model.Request) {
				var labels []string
				for _, v := range got.Versions {
					labels = append(labels, v.Label)
				}
				if !slices.Equal(labels, []string{"2", "1.1-mine"}) {
					t.Fatalf("labels %v", labels)
				}
			},
		},
		{
			name:   "without a base the spec's fields come as they are, Flow-I's stay",
			noBase: true,
			ours: func(r *model.Request) {
				r.Versions[0].Docs = "mine"
				r.Versions[0].Assert = []string{"status == 200"}
			},
			check: func(t *testing.T, got model.Request) {
				if got.Versions[0].Docs != "Finds a pet." || len(got.Versions[0].Assert) != 1 {
					t.Fatalf("got %+v", got.Versions[0])
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			base := specReq(nil)
			basePtr := &base
			if tt.noBase {
				basePtr = nil
			}

			got, diags := mergeReimport(basePtr, specReq(tt.ours), specReq(tt.theirs))

			tt.check(t, got)
			if codes := codesOf(diags); !slices.Equal(codes, tt.wantCodes) {
				t.Fatalf("codes %v, want %v", codes, tt.wantCodes)
			}
		})
	}
}
