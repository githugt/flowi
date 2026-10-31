// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func note(id string, needs ...string) model.Node {
	return model.Node{ID: id, Type: "note", Needs: needs, Params: params("text", "x")}
}

func fields(kv ...any) []model.Field {
	var out []model.Field
	for i := 0; i < len(kv); i += 2 {
		out = append(out, model.Field{Key: kv[i].(string), Value: kv[i+1]})
	}
	return out
}

// validateFlow validates a collection holding one flow of nodes.
func validateFlow(t *testing.T, change func(f *fixture, c *model.Collection, fl *model.Flow), nodes ...model.Node) diag.List {
	t.Helper()
	f := newFixture(t)
	c := f.collection("c")
	f.flow(&c, "f", nodes...)
	if change != nil {
		change(f, &c, &c.Flows[0])
	}
	return Validate(Target{Collections: []model.Collection{c}})
}

func hasDiag(l diag.List, code diag.Code, sev diag.Severity) bool {
	return slices.ContainsFunc(l, func(d diag.Diagnostic) bool { return d.Code == code && d.Severity == sev })
}

func TestCheckParams(t *testing.T) {
	tests := []struct {
		name string
		node model.Node
		want diag.Code // "" → valid
		sev  diag.Severity
	}{
		{name: "valid_delay", node: model.Node{ID: "d", Type: "delay", Params: fields("duration", "1s..3s")}},
		{name: "bad_delay", node: model.Node{ID: "d", Type: "delay", Params: fields("duration", "3s..1s")}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "templated_delay", node: model.Node{ID: "d", Type: "delay", Params: fields("duration", "{{env.wait}}")}},
		{name: "times_too_big", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(100000000)), Body: []model.Node{note("x")}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "over_and_times", node: model.Node{ID: "b", Type: "batch", Params: fields("times", int64(2), "over", "{{[1]}}"), Body: []model.Node{note("x")}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "neither_over_nor_times", node: model.Node{ID: "b", Type: "batch", Body: []model.Node{note("x")}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "enum_miss", node: model.Node{ID: "p", Type: "parallel", Params: fields("wait", "most"), Branches: []model.Branch{{ID: "a", Nodes: []model.Node{note("x")}}, {ID: "b", Nodes: []model.Node{note("y")}}}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "wrong_kind", node: model.Node{ID: "r", Type: "run_till", Params: fields("until", "true", "max", "lots"), Body: []model.Node{note("x")}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "bad_interval", node: model.Node{ID: "r", Type: "run_till", Params: fields("until", "true", "interval", "soon"), Body: []model.Node{note("x")}}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "save_traversal", node: model.Node{ID: "s", Type: "save", Params: fields("key", "../x", "value", "1")}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "digest_file_dir", node: model.Node{ID: "d", Type: "digest", Params: fields("sink", "file", "file", "a/b")}, want: diag.CodeNodeParam, sev: diag.SeverityError},
		{name: "digest_file_missing", node: model.Node{ID: "d", Type: "digest", Params: fields("sink", "file")}, want: diag.CodeNodeMissingField, sev: diag.SeverityError},
		{name: "digest_webhook_missing_url", node: model.Node{ID: "d", Type: "digest", Params: fields("sink", "webhook")}, want: diag.CodeNodeMissingField, sev: diag.SeverityError},
		{name: "unknown_param_warning", node: model.Node{ID: "n", Type: "note", Params: fields("text", "x", "colour", "red")}, want: diag.CodeNodeParam, sev: diag.SeverityWarning},
		{name: "set_values_ok", node: model.Node{ID: "g", Type: "set", Values: []model.SetValue{{Name: "email", From: "generate", Value: "{{$email}}", Seed: 3}}}},
		{name: "required_note_text", node: model.Node{ID: "n", Type: "note"}, want: diag.CodeNodeMissingField, sev: diag.SeverityError},
		{name: "condition_without_check", node: model.Node{ID: "c", Type: "condition", Then: []model.Node{note("x")}}, want: diag.CodeNodeMissingField, sev: diag.SeverityError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, nil, tt.node)

			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("got %v %v", codesOf(got), got)
				}
				return
			}
			if !hasDiag(got, tt.want, tt.sev) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}

func TestCheckExecution(t *testing.T) {
	tests := []struct {
		name   string
		change func(fl *model.Flow)
		bad    bool
	}{
		{name: "ok", change: func(fl *model.Flow) {
			fl.Execution = model.Execution{RateLimit: "20/s", HostRateLimits: []model.KeyValue{{Key: "a", Value: "1/m"}}, OnBranchError: "continue_all"}
		}},
		{name: "bad_rate", change: func(fl *model.Flow) { fl.Execution.RateLimit = "fast" }, bad: true},
		{name: "bad_host_rate", change: func(fl *model.Flow) { fl.Execution.HostRateLimits = []model.KeyValue{{Key: "a", Value: "0/s"}} }, bad: true},
		{name: "bad_policy", change: func(fl *model.Flow) { fl.Execution.OnBranchError = "panic" }, bad: true},
		{name: "run_without_until", change: func(fl *model.Flow) { fl.Run = &model.RunTill{MaxIterations: 3} }, bad: true},
		{name: "run_negative_max", change: func(fl *model.Flow) { fl.Run = &model.RunTill{Until: "true", MaxIterations: -1} }, bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, func(_ *fixture, _ *model.Collection, fl *model.Flow) { tt.change(fl) }, note("a"))

			if tt.bad != slices.Contains(codesOf(got), diag.CodeFlowOption) || (!tt.bad && len(got) != 0) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}

func TestCheckGotos(t *testing.T) {
	onError := func(v string) model.Node { n := note("a"); n.OnError = v; return n }
	tests := []struct {
		name    string
		onError string
		want    diag.Code
	}{
		{name: "forward", onError: "goto:c"},
		{name: "continue", onError: "continue"},
		{name: "fail", onError: "fail"},
		{name: "backward", onError: "goto:start", want: diag.CodeNodeGoto},
		{name: "self", onError: "goto:a", want: diag.CodeNodeGoto},
		{name: "unknown", onError: "goto:nowhere", want: diag.CodeNodeGoto},
		{name: "bad_value", onError: "retry", want: diag.CodeNodeParam},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, nil, note("start"), onError(tt.onError), note("b"), note("c"))

			if tt.want == "" && len(got) != 0 || tt.want != "" && !slices.Contains(codesOf(got), tt.want) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}

func setVar(id, name string) model.Node {
	return model.Node{ID: id, Type: "set", Params: params("target", "vars."+name, "value", "1")}
}

func TestCheckConcurrentWrites(t *testing.T) {
	branches := func(a, b model.Node) model.Node {
		return model.Node{ID: "par", Type: "parallel", Branches: []model.Branch{{ID: "x", Nodes: []model.Node{a}}, {ID: "y", Nodes: []model.Node{b}}}}
	}
	concurrent := func(n model.Node, needs ...string) model.Node { n.Concurrent, n.Needs = true, needs; return n }
	tests := []struct {
		name  string
		nodes []model.Node
		clash bool
	}{
		{name: "parallel_branches_same_var", nodes: []model.Node{branches(setVar("a", "token"), setVar("b", "token"))}, clash: true},
		{name: "parallel_branches_other_vars", nodes: []model.Node{branches(setVar("a", "token"), setVar("b", "other"))}},
		{name: "batch_body_is_iteration_local", nodes: []model.Node{{ID: "each", Type: "batch", Params: fields("times", int64(2), "concurrency", int64(2)), Body: []model.Node{setVar("a", "token"), setVar("b", "token")}}}},
		{name: "serial_siblings", nodes: []model.Node{setVar("a", "token"), setVar("b", "token")}},
		{name: "concurrent_sibling_not_a_dependant", nodes: []model.Node{note("start"), setVar("a", "token"), concurrent(setVar("b", "token"), "start")}, clash: true},
		{name: "concurrent_sibling_depending_on_writer", nodes: []model.Node{setVar("a", "token"), concurrent(setVar("b", "token"), "a")}},
		{name: "nested_condition_writes_count", nodes: []model.Node{branches(
			model.Node{ID: "c", Type: "condition", Params: params("check", "true"), Then: []model.Node{setVar("a", "token")}},
			model.Node{ID: "e", Type: "set", Values: []model.SetValue{{Name: "token", Value: "{{1}}"}}})}, clash: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, nil, tt.nodes...)

			if tt.clash != slices.Contains(codesOf(got), diag.CodeNodeWriteClash) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}

func TestCheckReferences(t *testing.T) {
	login := func(f *fixture, c *model.Collection) model.Request {
		f.request(c, "auth/login", "1")
		c.Requests[len(c.Requests)-1].Versions[0].Extract = []model.Extraction{{Name: "token", From: "body", Path: "$.token"}}
		return c.Requests[len(c.Requests)-1]
	}
	tests := []struct {
		name  string
		build func(f *fixture, c *model.Collection) []model.Node
		want  diag.Code
		sev   diag.Severity
		hint  string
	}{
		{name: "earlier_node_ok", build: func(f *fixture, c *model.Collection) []model.Node {
			return []model.Node{requestNode("login", login(f, c)), {ID: "n", Type: "note", Params: params("text", "{{nodes.login.token}} {{nodes.login.status}}")}}
		}},
		{name: "unknown_node", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "n", Type: "note", Params: params("text", "{{nodes.ghost.x}}")}}
		}, want: diag.CodeNodeUnresolved, sev: diag.SeverityError},
		{name: "later_sibling", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "n", Type: "note", Params: params("text", "{{nodes.later.text}}")}, note("later")}
		}, want: diag.CodeNodeUnresolved, sev: diag.SeverityError, hint: "later"},
		{name: "batch_inner_from_outside", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "each", Type: "batch", Params: fields("times", int64(2)), Body: []model.Node{note("inner")}},
				{ID: "n", Type: "note", Params: params("text", "{{nodes.inner.text}}")}}
		}, want: diag.CodeNodeUnresolved, sev: diag.SeverityError, hint: "results"},
		{name: "run_till_until_sees_body", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "poll", Type: "run_till", Params: params("until", "nodes.inner.text == 'x'"), Body: []model.Node{note("inner")}}}
		}},
		{name: "undeclared_input", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "n", Type: "note", Params: params("text", "{{inputs.email}}")}}
		}, want: diag.CodeNodeUnresolved, sev: diag.SeverityError},
		{name: "never_written_var", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{{ID: "n", Type: "note", Params: params("text", "{{vars.token}}")}}
		}, want: diag.CodeNodeUnresolved, sev: diag.SeverityError},
		{name: "written_var_ok", build: func(*fixture, *model.Collection) []model.Node {
			return []model.Node{setVar("s", "token"), {ID: "n", Type: "note", Params: params("text", "{{vars.token}}")}}
		}},
		{name: "contract_warning", build: func(f *fixture, c *model.Collection) []model.Node {
			return []model.Node{requestNode("login", login(f, c)), {ID: "n", Type: "note", Params: params("text", "{{nodes.login.tokn}}")}}
		}, want: diag.CodeNodeContract, sev: diag.SeverityWarning},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			f.flow(&c, "f", tt.build(f, &c)...)

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("got %v %v", codesOf(got), got)
				}
				return
			}
			d := only(t, got, tt.want)
			if d.Severity != tt.sev || (tt.hint != "" && !containsText(d.Args["hint"], tt.hint)) {
				t.Fatalf("diag %+v", d)
			}
		})
	}
}

func TestCheckReferences_Given_flow_used_as_hook_Then_vars_not_checked(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	hook := f.flow(&c, "hooks/h", model.Node{ID: "n", Type: "note", Params: params("text", "{{vars.from_earlier_hook}}")})
	c.Manifest.Hooks.Pre = []model.HookStep{{Flow: &model.Ref{ID: hook.ID, Path: hook.Path}}}

	got := Validate(Target{Collections: []model.Collection{c}})

	if len(got) != 0 {
		t.Fatalf("got %v %v", codesOf(got), got)
	}
}

func TestCheckSubflowInputs(t *testing.T) {
	tests := []struct {
		name string
		with []model.KeyValue
		want bool
	}{
		{name: "ok", with: []model.KeyValue{{Key: "who", Value: "a"}}},
		{name: "undeclared", with: []model.KeyValue{{Key: "who", Value: "a"}, {Key: "extra", Value: "b"}}, want: true},
		{name: "missing_required", with: nil, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			sub := f.flow(&c, "shared/greet", note("x"))
			c.Flows[0].Inputs = []model.Input{{Name: "who"}, {Name: "greeting", Default: "hi"}}
			f.flow(&c, "main", model.Node{ID: "s", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: sub.ID, Path: sub.Path}, With: tt.with}})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want != slices.Contains(codesOf(got), diag.CodeNodeSubflowInput) || (!tt.want && len(got) != 0) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}

func containsText(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// Final review, Important 4: a concurrent node may only read nodes its
// needs reach; other earlier siblings may still be running.
func TestCheckReferences_Given_concurrent_reader_Then_only_needed_nodes_are_available(t *testing.T) {
	reader := func(needs ...string) model.Node {
		return model.Node{ID: "c", Type: "note", Needs: needs, Concurrent: true, Params: params("text", "{{nodes.b.text}}")}
	}
	concurrentB := model.Node{ID: "b", Type: "note", Needs: []string{"a"}, Concurrent: true, Params: params("text", "x")}
	tests := []struct {
		name  string
		nodes []model.Node
		want  bool
	}{
		{name: "sibling_may_still_run", nodes: []model.Node{note("a"), concurrentB, reader("a")}, want: true},
		{name: "needed_sibling", nodes: []model.Node{note("a"), concurrentB, reader("b")}},
		{name: "transitively_needed", nodes: []model.Node{note("a"), note("b"), note("x", "b"), reader("x")}},
		{name: "serial_reader", nodes: []model.Node{note("a"), concurrentB, {ID: "c", Type: "note", Params: params("text", "{{nodes.b.text}}")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateFlow(t, nil, tt.nodes...)

			if tt.want {
				d := only(t, got, diag.CodeNodeUnresolved)
				if !containsText(d.Args["hint"], "needs") {
					t.Fatalf("hint %q", d.Args["hint"])
				}
				return
			}
			if len(got) != 0 {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}
