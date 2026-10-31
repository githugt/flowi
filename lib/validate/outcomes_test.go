// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestOutcomes_should_report_what_is_wrong_with_a_list(t *testing.T) {
	tests := []struct {
		name     string
		outcomes []model.Outcome
		want     bool
		path     string
	}{
		{name: "none_written"},
		{name: "defaults_and_a_named_one", outcomes: []model.Outcome{{Name: "success"}, {Name: "failure"}, {Name: "ok", When: "status in [200, 201]"}}},
		{name: "success_left_out", outcomes: []model.Outcome{{Name: "failure"}}},
		{name: "bad_name", outcomes: []model.Outcome{{Name: "not found", When: "status == 404"}}, want: true, path: "$.versions.1.outcomes[0].name"},
		{name: "duplicate", outcomes: []model.Outcome{{Name: "a", When: "status == 1"}, {Name: "a", When: "status == 2"}}, want: true, path: "$.versions.1.outcomes[1].name"},
		{name: "builtin_with_a_rule", outcomes: []model.Outcome{{Name: "success", When: "status == 200"}}, want: true, path: "$.versions.1.outcomes[0].when"},
		{name: "named_without_a_rule", outcomes: []model.Outcome{{Name: "ok"}}, want: true, path: "$.versions.1.outcomes[0].when"},
		{name: "rule_that_does_not_parse", outcomes: []model.Outcome{{Name: "ok", When: "status =="}}, want: true, path: "$.versions.1.outcomes[0].when"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "01JA00000000000000000000O1", Name: "r"}, Path: "r",
				Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "https://example.test", Outcomes: tt.outcomes}}})

			got := Validate(Target{Collections: []model.Collection{c}})

			var found []diag.Diagnostic
			for _, d := range got {
				if d.Code == diag.CodeOutcome {
					found = append(found, d)
				}
			}
			if !tt.want {
				if len(found) != 0 {
					t.Fatalf("unexpected %+v", found)
				}
				return
			}
			if len(found) != 1 || found[0].Path != tt.path {
				t.Fatalf("got %+v, want one at %s", found, tt.path)
			}
		})
	}
}

func TestNodes_should_report_an_after_outcome_that_is_not_step_and_name(t *testing.T) {
	log := func(id string) model.Node { return model.Node{ID: id, Type: "note", Params: params("text", "x")} }
	for name, entry := range map[string]string{"no_colon": "login", "bad_name": "a:not found", "unknown_step": "zzz:ok"} {
		t.Run(name, func(t *testing.T) {
			b := log("b")
			b.AfterOutcome = []string{entry}
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "x", log("a"), b)

			got := Validate(Target{Collections: []model.Collection{c}})

			var codes []diag.Code
			for _, d := range got {
				codes = append(codes, d.Code)
			}
			if !containsCode(codes, diag.CodeOutcome) && !containsCode(codes, diag.CodeNodeNeedsSibling) {
				t.Fatalf("codes %v", codes)
			}
		})
	}
}

func containsCode(codes []diag.Code, want diag.Code) bool {
	for _, c := range codes {
		if c == want {
			return true
		}
	}
	return false
}

func TestOutcomes_should_report_a_colour_that_is_not_one_a_step_can_have(t *testing.T) {
	good := outcomeDiags("f", "$.versions.1.outcomes", []model.Outcome{{Name: "success", Color: "teal"}})
	bad := outcomeDiags("f", "$.versions.1.outcomes", []model.Outcome{{Name: "success", Color: "mauve"}})

	if len(good) != 0 || len(bad) != 1 || bad[0].Path != "$.versions.1.outcomes[0].color" {
		t.Fatalf("good %+v, bad %+v", good, bad)
	}
}

func TestOutcomes_should_check_the_outcomes_a_request_step_carries(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	req := model.Request{Meta: model.Meta{ID: "01JA00000000000000000000O2", Name: "r"}, Path: "r", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "https://example.test"}}}
	c.Requests = append(c.Requests, req)
	step := model.Node{ID: "call", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: req.ID, Path: "r"}, Outcomes: []model.Outcome{{Name: "gone"}}}}
	f.flow(&c, "x", step)

	got := Validate(Target{Collections: []model.Collection{c}})

	var found []string
	for _, d := range got {
		if d.Code == diag.CodeOutcome {
			found = append(found, d.Path)
		}
	}
	if len(found) != 1 || found[0] != "$.nodes[0].outcomes[0].when" {
		t.Fatalf("paths %v", found)
	}
}
