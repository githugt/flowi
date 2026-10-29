// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"reflect"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func baseVersion() model.RequestVersion {
	return model.RequestVersion{
		Label:     "1",
		Method:    "GET",
		URL:       "{{base}}/users",
		Query:     []model.KeyValue{{Key: "page", Value: "1"}},
		Headers:   []model.KeyValue{{Key: "Accept", Value: "application/json"}},
		Body:      model.Body{JSON: `{"name":"ada","plan":{"tier":"free"}}`},
		Assert:    []string{"status == 200"},
		Extract:   []model.Extraction{{Name: "id", From: "body", Path: "id"}},
		Settings:  model.Settings{"timeouts.total": "10s"},
		Variables: []model.KeyValue{{Key: "who", Value: "ada"}},
	}
}

func TestApply_should_set_remove_and_add_across_fields(t *testing.T) {
	// Arrange
	ops := []model.OverlayOp{
		{Verb: model.OverlaySet, Path: "headers.x-tier", Value: "vip"},
		{Verb: model.OverlaySet, Path: "headers.accept", Value: "text/plain"},
		{Verb: model.OverlayRemove, Path: "query.page"},
		{Verb: model.OverlaySet, Path: "url", Value: "{{base}}/vip"},
		{Verb: model.OverlayAdd, Path: "assert", Value: "headers.x-tier == 'vip'"},
		{Verb: model.OverlaySet, Path: "settings.timeouts.total", Value: "1s"},
		{Verb: model.OverlaySet, Path: "extract.tier", Value: map[string]any{"from": "header", "header": "X-Tier"}},
	}

	// Act
	got, problems := Apply(baseVersion(), ops)

	// Assert
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	if got.URL != "{{base}}/vip" || len(got.Query) != 0 {
		t.Errorf("url %q query %v", got.URL, got.Query)
	}
	if want := []model.KeyValue{{Key: "Accept", Value: "text/plain"}, {Key: "x-tier", Value: "vip"}}; !reflect.DeepEqual(got.Headers, want) {
		t.Errorf("headers = %v", got.Headers)
	}
	if len(got.Assert) != 2 || got.Settings["timeouts.total"] != "1s" || len(got.Extract) != 2 {
		t.Errorf("assert %v settings %v extract %v", got.Assert, got.Settings, got.Extract)
	}
}

func TestApply_should_not_change_the_version_it_was_given(t *testing.T) {
	// Arrange
	base := baseVersion()

	// Act
	Apply(base, []model.OverlayOp{
		{Verb: model.OverlaySet, Path: "headers.Accept", Value: "x"},
		{Verb: model.OverlaySet, Path: "settings.timeouts.total", Value: "1s"},
		{Verb: model.OverlayAdd, Path: "assert", Value: "a"},
	})

	// Assert
	if !reflect.DeepEqual(base, baseVersion()) {
		t.Errorf("version was changed: %+v", base)
	}
}

func TestApply_should_leave_out_what_does_not_fit_and_apply_the_rest(t *testing.T) {
	// Arrange
	ops := []model.OverlayOp{
		{Verb: model.OverlayRemove, Path: "headers.X-Gone"},
		{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"},
		{Verb: model.OverlayRemove, Path: "assert", Value: "never there"},
		{Verb: model.OverlaySet, Path: "colour", Value: "red"},
		{Verb: model.OverlayRemove, Path: "url"},
	}

	// Act
	got, problems := Apply(baseVersion(), ops)

	// Assert
	if len(problems) != 4 {
		t.Fatalf("problems = %v", problems)
	}
	for _, p := range problems {
		if p.Drift || p.Reason == "" {
			t.Errorf("problem %+v should be a left-out operation with a reason", p)
		}
	}
	if len(got.Headers) != 2 {
		t.Errorf("the fitting operation was not applied: %v", got.Headers)
	}
}

func TestApply_should_report_drift_but_still_apply(t *testing.T) {
	// Arrange
	ops := []model.OverlayOp{{Verb: model.OverlaySet, Path: "url", Value: "{{base}}/vip", Was: "{{base}}/people"}}

	// Act
	got, problems := Apply(baseVersion(), ops)

	// Assert
	if got.URL != "{{base}}/vip" {
		t.Errorf("url = %q", got.URL)
	}
	if len(problems) != 1 || !problems[0].Drift || !strings.Contains(problems[0].Reason, "changed") {
		t.Errorf("problems = %+v", problems)
	}
}

func TestApply_should_merge_patch_a_json_body(t *testing.T) {
	// Arrange
	patch := map[string]any{"plan": map[string]any{"tier": "vip"}, "name": nil, "extra": 1}

	// Act
	got, problems := Apply(baseVersion(), []model.OverlayOp{{Verb: model.OverlayMerge, Path: "body.json", Value: patch}})

	// Assert
	if len(problems) != 0 {
		t.Fatalf("problems = %v", problems)
	}
	for _, want := range []string{`"tier": "vip"`, `"extra": 1`} {
		if !strings.Contains(got.Body.JSON, want) {
			t.Errorf("body %s lacks %s", got.Body.JSON, want)
		}
	}
	if strings.Contains(got.Body.JSON, `"name"`) {
		t.Errorf("null should remove name: %s", got.Body.JSON)
	}
}

func TestApply_should_refuse_to_patch_a_body_that_is_not_json(t *testing.T) {
	// Arrange
	v := baseVersion()
	v.Body = model.Body{Text: "plain"}

	// Act
	_, problems := Apply(v, []model.OverlayOp{{Verb: model.OverlayMerge, Path: "body.json", Value: map[string]any{"a": 1}}})

	// Assert
	if len(problems) != 1 || !strings.Contains(problems[0].Reason, "not JSON") {
		t.Errorf("problems = %+v", problems)
	}
}

func TestApply_should_not_use_url_or_query_on_a_message_request(t *testing.T) {
	// Arrange
	v := baseVersion()
	v.Async = &model.Async{Protocol: "mqtt"}

	// Act
	_, problems := Apply(v, []model.OverlayOp{
		{Verb: model.OverlaySet, Path: "url", Value: "x"},
		{Verb: model.OverlaySet, Path: "headers.k", Value: "v"},
	})

	// Assert
	if len(problems) != 1 || problems[0].Op.Path != "url" {
		t.Errorf("problems = %+v", problems)
	}
}

func TestDiff_should_give_ops_that_apply_back_to_the_edit(t *testing.T) {
	// Arrange
	base := baseVersion()
	edited := baseVersion()
	edited.Headers = append(edited.Headers, model.KeyValue{Key: "X-Tier", Value: "vip"})
	edited.Query = nil
	edited.URL = "{{base}}/vip"
	edited.Assert = []string{"status == 201"}
	edited.Body.JSON = `{"plan":{"tier":"vip"},"extra":true}`
	edited.Settings = model.Settings{"timeouts.total": "1s", "retries.max": 2}
	edited.Extract = nil
	edited.Variables = []model.KeyValue{{Key: "who", Value: "grace"}}

	// Act
	ops, unsupported := Diff(base, edited)
	got, problems := Apply(base, ops)

	// Assert
	if len(unsupported) != 0 || len(problems) != 0 {
		t.Fatalf("unsupported %v problems %v", unsupported, problems)
	}
	if got.URL != edited.URL || len(got.Query) != 0 || len(got.Extract) != 0 || !reflect.DeepEqual(got.Assert, edited.Assert) {
		t.Errorf("got %+v", got)
	}
	if !reflect.DeepEqual(got.Headers, edited.Headers) || !reflect.DeepEqual(got.Variables, edited.Variables) || !reflect.DeepEqual(got.Settings, edited.Settings) {
		t.Errorf("got headers %v variables %v settings %v", got.Headers, got.Variables, got.Settings)
	}
	if !strings.Contains(got.Body.JSON, `"extra": true`) || !strings.Contains(got.Body.JSON, `"vip"`) || strings.Contains(got.Body.JSON, `"name"`) {
		t.Errorf("body = %s", got.Body.JSON)
	}
}

func TestDiff_should_give_nothing_for_an_unchanged_version(t *testing.T) {
	// Act
	ops, unsupported := Diff(baseVersion(), baseVersion())

	// Assert
	if len(ops) != 0 || len(unsupported) != 0 {
		t.Errorf("ops %v unsupported %v", ops, unsupported)
	}
}

func TestDiff_should_name_the_parts_an_overlay_cannot_hold(t *testing.T) {
	// Arrange
	edited := baseVersion()
	edited.Body = model.Body{Form: []model.KeyValue{{Key: "a", Value: "b"}}}
	edited.Hooks = model.Hooks{Pre: []model.HookStep{{}}}

	// Act
	_, unsupported := Diff(baseVersion(), edited)

	// Assert
	if len(unsupported) != 2 {
		t.Errorf("unsupported = %v", unsupported)
	}
}

func TestEncode_should_round_trip_through_Decode(t *testing.T) {
	// Arrange
	o := model.Overlay{
		Name: "vip", Description: "the VIP customer",
		Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}},
		Requests: []model.RequestOverlay{{Request: "01JREQ", Path: "users/get", Ops: []model.OverlayOp{
			{Verb: model.OverlayMerge, Path: "body.json", Value: map[string]any{"plan": map[string]any{"tier": "vip"}}},
			{Verb: model.OverlayRemove, Path: "query.page", Was: "1"},
			{Verb: model.OverlayAdd, Path: "assert", Value: "status == 200"},
		}}},
	}

	// Act
	data, err := Encode(o)
	if err != nil {
		t.Fatal(err)
	}
	got, diags := Decode("vip.flowi-overlay.yaml", "vip", data)

	// Assert
	if len(diags) != 0 {
		t.Fatalf("diags = %v\n%s", diags, data)
	}
	got.File = ""
	if !reflect.DeepEqual(got, o) {
		t.Errorf("round trip changed it:\n%s\n got %+v\nwant %+v", data, got, o)
	}
}

func TestDecode_should_reject_what_is_not_an_overlay(t *testing.T) {
	cases := map[string]string{
		"no format":   "name: a\n",
		"two verbs":   "flowi_overlay: 1\nshared:\n  - set: url\n    remove: url\n",
		"no verb":     "flowi_overlay: 1\nshared:\n  - value: 1\n",
		"unknown key": "flowi_overlay: 1\nsurprise: 1\n",
	}
	for name, src := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, diags := Decode("x.flowi-overlay.yaml", "x", []byte(src))

			// Assert
			if len(diags) == 0 {
				t.Error("expected a diagnostic")
			}
		})
	}
}

func TestForRequest_should_put_the_shared_part_first(t *testing.T) {
	// Arrange
	o := model.Overlay{
		Shared:   []model.OverlayOp{{Verb: model.OverlaySet, Path: "url", Value: "s"}},
		Requests: []model.RequestOverlay{{Request: "a", Ops: []model.OverlayOp{{Verb: model.OverlaySet, Path: "url", Value: "a"}}}, {Request: "b"}},
	}

	// Act
	ops := ForRequest(o, "a")

	// Assert
	if len(ops) != 2 || ops[0].Value != "s" || ops[1].Value != "a" {
		t.Errorf("ops = %v", ops)
	}
}

func TestDescribe_should_read_as_a_sentence(t *testing.T) {
	cases := map[string]model.OverlayOp{
		`Set header X-Tier to "vip"`:    {Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"},
		`Remove query parameter page`:   {Verb: model.OverlayRemove, Path: "query.page"},
		`Add check "status == 200"`:     {Verb: model.OverlayAdd, Path: "assert", Value: "status == 200"},
		`Set the url to "/x"`:           {Verb: model.OverlaySet, Path: "url", Value: "/x"},
		`Change the JSON body: {"a":1}`: {Verb: model.OverlayMerge, Path: "body.json", Value: map[string]any{"a": 1}},
	}
	for want, op := range cases {
		// Act
		got := Describe(op)

		// Assert
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestResolve_should_join_the_operations_of_stacked_overlays_in_the_order_named(t *testing.T) {
	// Arrange
	id := model.ID("r1")
	vip := model.Overlay{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.x-tier", Value: "vip"}}}
	slow := model.Overlay{Name: "slow", Requests: []model.RequestOverlay{{Request: id, Ops: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.x-tier", Value: "slow"}}}}}

	// Act
	ops, missing := Resolve([]model.Overlay{slow, vip}, "vip+slow", id)
	got, _ := Apply(baseVersion(), ops)

	// Assert
	if missing != "" || len(ops) != 2 {
		t.Fatalf("missing %q, ops %v", missing, ops)
	}
	if v, _ := headerOf(got, "x-tier"); v != "slow" {
		t.Fatalf("the later overlay should win, got %q", v)
	}
}

func TestResolve_should_name_the_first_overlay_of_a_stack_that_does_not_exist(t *testing.T) {
	// Arrange
	vip := model.Overlay{Name: "vip"}

	// Act
	_, missing := Resolve([]model.Overlay{vip}, "vip+ghost", "r1")

	// Assert
	if missing != "ghost" {
		t.Fatalf("missing %q", missing)
	}
}

func TestStack_should_split_on_plus_and_ignore_blanks(t *testing.T) {
	got := Stack("a++b+")

	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %v", got)
	}
}

func TestCheckName_should_refuse_the_stacking_separator(t *testing.T) {
	if err := CheckName("a+b"); err == nil {
		t.Fatal("a plus sign would read as two stacked overlays")
	}
}

func headerOf(v model.RequestVersion, name string) (string, bool) {
	for _, h := range v.Headers {
		if strings.EqualFold(h.Key, name) {
			return h.Value, true
		}
	}
	return "", false
}

func hookFlow(id string) model.HookStep { return model.HookStep{Flow: &model.Ref{ID: model.ID(id)}} }

func TestApply_should_add_and_remove_a_reusable_flow_as_a_hook(t *testing.T) {
	// Arrange
	base := baseVersion()
	base.Hooks.Pre = []model.HookStep{hookFlow("login")}
	ops := []model.OverlayOp{
		{Verb: model.OverlayAdd, Path: "hooks.pre", Value: map[string]any{"flow": "audit"}},
		{Verb: model.OverlayRemove, Path: "hooks.pre", Value: "login"},
		{Verb: model.OverlayAdd, Path: "hooks.post", Value: "cleanup"},
	}

	// Act
	got, problems := Apply(base, ops)

	// Assert
	if len(problems) != 0 {
		t.Fatalf("problems %v", problems)
	}
	if !reflect.DeepEqual(got.Hooks.Pre, []model.HookStep{hookFlow("audit")}) || !reflect.DeepEqual(got.Hooks.Post, []model.HookStep{hookFlow("cleanup")}) {
		t.Fatalf("hooks %+v", got.Hooks)
	}
	if len(base.Hooks.Pre) != 1 || base.Hooks.Pre[0].Flow.ID != "login" {
		t.Fatal("the base version must not change")
	}
}

func TestApply_should_report_a_hook_it_cannot_remove_or_add_twice(t *testing.T) {
	// Arrange
	base := baseVersion()
	base.Hooks.Pre = []model.HookStep{hookFlow("login")}
	ops := []model.OverlayOp{
		{Verb: model.OverlayRemove, Path: "hooks.pre", Value: "ghost"},
		{Verb: model.OverlayAdd, Path: "hooks.pre", Value: "login"},
		{Verb: model.OverlayAdd, Path: "hooks.nowhere", Value: "x"},
	}

	// Act
	_, problems := Apply(base, ops)

	// Assert
	if len(problems) != 3 {
		t.Fatalf("problems %v", problems)
	}
}

func TestDiff_should_carry_changes_of_flow_hooks_and_still_refuse_inline_ones(t *testing.T) {
	// Arrange
	base := baseVersion()
	base.Hooks.Pre = []model.HookStep{hookFlow("login")}
	edited := baseVersion()
	edited.Hooks.Pre = []model.HookStep{hookFlow("audit")}
	inline := baseVersion()
	inline.Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "n", Type: "wait"}}}}

	// Act
	ops, unsupported := Diff(base, edited)
	_, inlineNotes := Diff(baseVersion(), inline)

	// Assert
	if len(unsupported) != 0 || len(ops) != 2 {
		t.Fatalf("ops %v unsupported %v", ops, unsupported)
	}
	applied, _ := Apply(base, ops)
	if !reflect.DeepEqual(applied.Hooks, edited.Hooks) {
		t.Fatalf("applied %+v", applied.Hooks)
	}
	if len(inlineNotes) != 1 {
		t.Fatalf("inline hooks should be reported as not kept, got %v", inlineNotes)
	}
}

func TestCheckOp_should_take_add_and_remove_on_hooks_only_with_a_phase(t *testing.T) {
	if err := CheckOp(model.OverlayOp{Verb: model.OverlayAdd, Path: "hooks.pre", Value: "x"}); err != nil {
		t.Fatal(err)
	}
	if err := CheckOp(model.OverlayOp{Verb: model.OverlayAdd, Path: "hooks", Value: "x"}); err == nil {
		t.Fatal("hooks needs pre, post or message")
	}
	if err := CheckOp(model.OverlayOp{Verb: model.OverlaySet, Path: "hooks.pre", Value: "x"}); err == nil {
		t.Fatal("hooks does not take set")
	}
}
