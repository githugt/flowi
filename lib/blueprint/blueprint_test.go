// SPDX-License-Identifier: Apache-2.0

package blueprint

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// resolver is a fake that knows a few requests and flows by path.
type resolver struct{}

func (resolver) Request(v string) (model.Ref, error) {
	known := map[string]model.ID{"auth/login": "01JREQ00000000000000000001", "users/create": "01JREQ00000000000000000002"}
	if id, ok := known[v]; ok {
		return model.Ref{ID: id, Path: v}, nil
	}
	return model.Ref{}, fmt.Errorf("no request %q", v)
}

func (resolver) Flow(v string) (model.Ref, error) {
	if v == "verify" {
		return model.Ref{ID: "01JFLW00000000000000000001", Path: "verify"}, nil
	}
	return model.Ref{}, fmt.Errorf("no flow %q", v)
}

func newID() func() model.ID {
	n := 0
	return func() model.ID {
		n++
		return model.ID(fmt.Sprintf("01JNEW0000000000000000%04d", n))
	}
}

func requestNode(id, param string) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{Param: param}, Version: "2"}}
}

func sampleBlueprint() model.Blueprint {
	return model.Blueprint{
		Meta: model.Meta{ID: "01JBP000000000000000000001", Name: "Onboarding"},
		Params: []model.Param{
			{Name: "api", Type: model.ParamRequestRef, Default: "auth/login"},
			{Name: "creator", Type: model.ParamRequestRef},
			{Name: "email", Type: model.ParamString, Default: "demo@example.com"},
			{Name: "retries", Type: model.ParamInt},
			{Name: "notify", Type: model.ParamBool, Default: "false"},
			{Name: "after", Type: model.ParamFlowRef, Default: "verify"},
		},
		Body: model.Flow{
			PreferVersion: "2",
			Inputs:        []model.Input{{Name: "note", Default: "hello"}},
			Nodes: []model.Node{
				requestNode("login", "api"),
				{ID: "each", Type: "batch", Body: []model.Node{requestNode("create", "creator")}},
				{ID: "branch", Type: "condition", Then: []model.Node{requestNode("then", "api")}, Else: []model.Node{{ID: "sub", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{Param: "after"}}}}},
				{ID: "par", Type: "parallel", Branches: []model.Branch{{ID: "b1", Nodes: []model.Node{requestNode("p1", "creator")}}}},
				{ID: "sw", Type: "condition", Cases: []model.Case{{When: "x", Nodes: []model.Node{requestNode("c1", "api")}}}},
			},
		},
	}
}

func values() map[string]string {
	return map[string]string{"creator": "users/create", "retries": "3"}
}

func hasCode(l diag.List, c diag.Code) bool {
	for _, d := range l {
		if d.Code == c {
			return true
		}
	}
	return false
}

func TestCheck_should_accept_a_well_formed_blueprint(t *testing.T) {
	if diags := Check(sampleBlueprint()); len(diags) != 0 {
		t.Fatalf("diags %v", diags)
	}
}

func TestCheck_should_report_each_structural_problem(t *testing.T) {
	tests := []struct {
		name  string
		edit  func(*model.Blueprint)
		code  diag.Code
		match string
	}{
		{"undeclared param", func(b *model.Blueprint) { b.Body.Nodes[0] = requestNode("login", "nope") }, diag.CodeBlueprintRef, `"nope"`},
		{"wrong type for a request", func(b *model.Blueprint) { b.Body.Nodes[0] = requestNode("login", "email") }, diag.CodeBlueprintRef, "request-ref"},
		{"wrong type for a subflow", func(b *model.Blueprint) {
			b.Body.Nodes[2].Else[0].Subflow.Flow.Param = "api"
		}, diag.CodeBlueprintRef, "flow-ref"},
		{"bad name", func(b *model.Blueprint) { b.Params[2].Name = "not valid" }, diag.CodeBlueprintParamName, "not valid"},
		{"duplicate name", func(b *model.Blueprint) { b.Params = append(b.Params, b.Params[0]) }, diag.CodeBlueprintParamName, "twice"},
		{"clashes with an input", func(b *model.Blueprint) { b.Params[2].Name = "note" }, diag.CodeBlueprintParamName, "input"},
		{"unknown type", func(b *model.Blueprint) { b.Params[3].Type = "money" }, diag.CodeBlueprintParamType, "money"},
		{"unused reference parameter", func(b *model.Blueprint) {
			b.Params = append(b.Params, model.Param{Name: "spare", Type: model.ParamRequestRef})
		}, diag.CodeBlueprintParamUnused, "spare"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := sampleBlueprint()
			tt.edit(&bp)

			diags := Check(bp)

			if !hasCode(diags, tt.code) {
				t.Fatalf("want %s in %v", tt.code, diags)
			}
			joined := messages(diags)
			if !strings.Contains(joined, tt.match) {
				t.Fatalf("diagnostics do not mention %q: %s", tt.match, joined)
			}
		})
	}
}

func TestInstantiate_should_make_an_ordinary_flow_with_no_parameters_left(t *testing.T) {
	bp := sampleBlueprint()

	f, diags := Instantiate(bp, Input{Values: values(), Resolve: resolver{}, NewID: newID(), Path: "team/onboarding", Name: "Team onboarding"})

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	if f.ID == "" || f.ID == bp.ID || f.Path != "team/onboarding" || f.Name != "Team onboarding" || f.PreferVersion != "2" {
		t.Fatalf("flow identity %+v", f.Meta)
	}
	login := f.Nodes[0].Request.Request
	if login.ID != "01JREQ00000000000000000001" || login.Path != "auth/login" || login.Param != "" || f.Nodes[0].Request.Version != "2" {
		t.Fatalf("login %+v", f.Nodes[0].Request)
	}
	if got := f.Nodes[1].Body[0].Request.Request; got.Path != "users/create" {
		t.Fatalf("batch body %+v", got)
	}
	if got := f.Nodes[2].Else[0].Subflow.Flow; got.Path != "verify" || got.Param != "" {
		t.Fatalf("subflow %+v", got)
	}
	if left := paramRefsIn(f.Nodes); left != 0 {
		t.Fatalf("%d parameter references left in the flow", left)
	}
}

func TestInstantiate_should_turn_value_parameters_into_inputs_with_the_given_defaults(t *testing.T) {
	v := values()
	v["email"] = "ops@example.com"

	f, diags := Instantiate(sampleBlueprint(), Input{Values: v, Resolve: resolver{}, NewID: newID(), Path: "x", Name: "x"})

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	got := map[string]string{}
	for _, in := range f.Inputs {
		got[in.Name] = in.Default
	}
	want := map[string]string{"note": "hello", "email": "ops@example.com", "retries": "3", "notify": "false"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("inputs %v, want %v", got, want)
	}
}

func TestInstantiate_should_not_change_the_blueprint(t *testing.T) {
	bp := sampleBlueprint()

	_, _ = Instantiate(bp, Input{Values: values(), Resolve: resolver{}, NewID: newID(), Path: "x", Name: "x"})

	if !reflect.DeepEqual(bp, sampleBlueprint()) {
		t.Fatal("Instantiate changed the blueprint")
	}
}

func TestInstantiate_should_reject_what_cannot_be_used(t *testing.T) {
	tests := []struct {
		name   string
		values map[string]string
		match  string
	}{
		{"missing value", map[string]string{"retries": "3"}, `"creator"`},
		{"unknown parameter", map[string]string{"creator": "users/create", "retries": "3", "colour": "red"}, `"colour"`},
		{"bad int", map[string]string{"creator": "users/create", "retries": "many"}, "whole number"},
		{"bad bool", map[string]string{"creator": "users/create", "retries": "3", "notify": "maybe"}, "true or false"},
		{"request not found", map[string]string{"creator": "users/delete-everything", "retries": "3"}, "users/delete-everything"},
		{"flow not found", map[string]string{"creator": "users/create", "retries": "3", "after": "nowhere"}, "nowhere"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diags := Instantiate(sampleBlueprint(), Input{Values: tt.values, Resolve: resolver{}, NewID: newID(), Path: "x", Name: "x"})

			if !diags.HasErrors() || !hasCode(diags, diag.CodeBlueprintUse) || !strings.Contains(fmt.Sprint(diags), tt.match) {
				t.Fatalf("want a %s mentioning %q, got %v", diag.CodeBlueprintUse, tt.match, diags)
			}
		})
	}
}

func TestInstantiate_should_report_every_problem_at_once(t *testing.T) {
	_, diags := Instantiate(sampleBlueprint(), Input{Values: map[string]string{"colour": "red", "retries": "many"}, Resolve: resolver{}, NewID: newID(), Path: "x", Name: "x"})

	if len(diags) < 3 {
		t.Fatalf("expected the unknown parameter, the missing creator and the bad number, got %v", diags)
	}
}

func TestFromFlow_should_turn_references_and_inputs_into_parameters_with_defaults(t *testing.T) {
	f := model.Flow{
		Meta:   model.Meta{ID: "01JFLOW0000000000000000001", Name: "Signup", Description: "Sign up a user"},
		Inputs: []model.Input{{Name: "email", Default: "a@example.com"}, {Name: "plan"}},
		Nodes: []model.Node{
			{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JREQ00000000000000000001", Path: "auth/login"}}},
			{ID: "again", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JREQ00000000000000000001", Path: "auth/login"}}},
			{ID: "other", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JREQ00000000000000000002", Path: "other/login"}}},
			{ID: "foreign", Type: "request", Request: &model.RequestCall{Request: model.Ref{Collection: "identity-api", ID: "01JREQ00000000000000000009", Path: "id/me"}}},
			{ID: "sub", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: "01JFLW00000000000000000001", Path: "verify"}}},
		},
	}

	bp, err := FromFlow(f, newID(), "Signup template")

	if err != nil {
		t.Fatal(err)
	}
	names := map[string]model.Param{}
	for _, p := range bp.Params {
		names[p.Name] = p
	}
	if bp.Name != "Signup template" || bp.Description != "Sign up a user" || bp.ID == f.ID || bp.ID == "" {
		t.Fatalf("identity %+v", bp.Meta)
	}
	if login := names["login"]; login.Type != model.ParamRequestRef || login.Default != "auth/login" {
		t.Fatalf("login %+v (params %+v)", login, bp.Params)
	}
	if second := names["login_2"]; second.Default != "other/login" {
		t.Fatalf("a second request with the same last segment needs its own name: %+v", bp.Params)
	}
	if verify := names["verify"]; verify.Type != model.ParamFlowRef || verify.Default != "verify" {
		t.Fatalf("verify %+v", verify)
	}
	if email := names["email"]; email.Type != model.ParamString || email.Default != "a@example.com" {
		t.Fatalf("email %+v", email)
	}
	if plan, ok := names["plan"]; !ok || plan.Default != "" {
		t.Fatalf("a required input stays required: %+v", plan)
	}
	if bp.Body.Nodes[0].Request.Request.Param != "login" || bp.Body.Nodes[1].Request.Request.Param != "login" || bp.Body.Nodes[2].Request.Request.Param != "login_2" {
		t.Fatalf("nodes not re-pointed: %+v", bp.Body.Nodes)
	}
	if foreign := bp.Body.Nodes[3].Request.Request; foreign.Param != "" || foreign.Collection != "identity-api" {
		t.Fatalf("a reference into another collection stays literal: %+v", foreign)
	}
	if len(bp.Body.Inputs) != 0 {
		t.Fatalf("inputs became parameters, got %+v", bp.Body.Inputs)
	}
	if diags := Check(bp); diags.HasErrors() {
		t.Fatalf("the blueprint made from a flow must pass Check: %v", diags)
	}
}

func TestFromFlow_then_Instantiate_with_defaults_should_rebuild_the_flow(t *testing.T) {
	original := model.Flow{
		Meta:   model.Meta{ID: "01JFLOW0000000000000000001", Name: "Signup"},
		Inputs: []model.Input{{Name: "email", Default: "a@example.com"}},
		Nodes: []model.Node{
			{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JREQ00000000000000000001", Path: "auth/login"}, Version: "2"}},
			{ID: "each", Type: "batch", Body: []model.Node{{ID: "make", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JREQ00000000000000000002", Path: "users/create"}}}}},
		},
	}
	bp, err := FromFlow(original, newID(), "t")
	if err != nil {
		t.Fatal(err)
	}

	rebuilt, diags := Instantiate(bp, Input{Resolve: resolver{}, NewID: newID(), Path: "copy", Name: "Copy"})

	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	if !reflect.DeepEqual(rebuilt.Nodes, original.Nodes) || !reflect.DeepEqual(rebuilt.Inputs, original.Inputs) {
		t.Fatalf("rebuilt differs:\n%+v\n%+v", rebuilt, original)
	}
}

func TestFromFlow_should_refuse_a_flow_whose_name_would_make_an_invalid_blueprint(t *testing.T) {
	_, err := FromFlow(model.Flow{}, newID(), "")

	if !errors.Is(err, ErrNoName) {
		t.Fatalf("err %v", err)
	}
}

// paramRefsIn counts node references that still name a parameter.
func paramRefsIn(nodes []model.Node) int {
	n := 0
	for _, node := range nodes {
		if node.Request != nil && node.Request.Request.Param != "" {
			n++
		}
		if node.Subflow != nil && node.Subflow.Flow.Param != "" {
			n++
		}
		for _, list := range node.Children() {
			n += paramRefsIn(list)
		}
	}
	return n
}

// messages renders diagnostics in English, as a user would read them.
func messages(l diag.List) string {
	var parts []string
	for _, d := range l {
		msg, _ := diag.English().Message(d)
		parts = append(parts, msg)
	}
	return strings.Join(parts, "\n")
}

func TestBaseName_should_make_a_usable_parameter_name_from_a_path(t *testing.T) {
	tests := map[string]string{
		"auth/login":     "login",
		"users/1abc":     "request_1abc",
		"users/e-mail":   "e_mail",
		"weird/--":       "request_",
		"plain":          "plain",
		"deep/a/b/c.d.e": "c_d_e",
	}
	for target, want := range tests {
		if got := baseName(target, model.ParamRequestRef); got != want {
			t.Errorf("baseName(%q) = %q, want %q", target, got, want)
		}
	}
	if got := baseName("1x", model.ParamFlowRef); got != "flow_1x" {
		t.Errorf("flow kind: %q", got)
	}
}

func TestInstantiate_should_refuse_a_blueprint_that_fails_Check(t *testing.T) {
	tests := map[string]func(*model.Blueprint){
		"undeclared param":     func(b *model.Blueprint) { b.Body.Nodes[0] = requestNode("login", "nope") },
		"wrong type":           func(b *model.Blueprint) { b.Body.Nodes[0] = requestNode("login", "email") },
		"param named an input": func(b *model.Blueprint) { b.Params[2].Name = "note" },
	}
	for name, edit := range tests {
		t.Run(name, func(t *testing.T) {
			bp := sampleBlueprint()
			edit(&bp)

			f, diags := Instantiate(bp, Input{Values: values(), Resolve: resolver{}, NewID: newID(), Path: "x", Name: "x"})

			if !diags.HasErrors() || f.ID != "" {
				t.Fatalf("a broken blueprint produced a flow: %+v %v", f.Meta, diags)
			}
		})
	}
}
