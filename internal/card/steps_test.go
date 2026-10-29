// SPDX-License-Identifier: AGPL-3.0-or-later

package card_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/card"
)

func fields(kv ...any) []model.Field {
	var out []model.Field
	for i := 0; i < len(kv); i += 2 {
		out = append(out, model.Field{Key: kv[i].(string), Value: kv[i+1]})
	}
	return out
}

func TestSummarize_Given_each_other_step_Then_subline_rows_and_ports_follow_the_design_note(t *testing.T) {
	step := func(id string, typ model.NodeType, p ...any) model.Node {
		return model.Node{ID: id, Type: typ, Params: fields(p...)}
	}
	tests := []struct {
		name    string
		node    model.Node
		facts   card.Facts
		subline string
		rows    []string
		ports   []string
	}{
		{
			name:    "flow",
			node:    model.Node{ID: "f", Type: "subflow", Subflow: &model.SubflowCall{With: []model.KeyValue{{Key: "a"}, {Key: "b"}, {Key: "c"}}}},
			facts:   card.Facts{Target: "Shared login", Returns: 2},
			subline: "Shared login",
			rows:    []string{"inputs: 3 passed", "values: 2 returned"},
			ports:   []string{"done", "fail"},
		},
		{
			name: "set_value_four_names",
			node: model.Node{ID: "s", Type: "set", Values: []model.SetValue{
				{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
			}},
			subline: "4 values",
			rows:    []string{"values: a, b, c +1"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "set_one_target",
			node:    step("s", "set", "target", "vars.token", "value", "x"),
			subline: "1 value",
			rows:    []string{"values: token"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "set_with_nothing_yet_still_offers_its_row",
			node:    model.Node{ID: "s", Type: "set"},
			subline: "",
			rows:    []string{"values: add a value"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "check",
			node:    step("c", "assert", "check", "nodes.login.status == 200"),
			subline: "1 check",
			rows:    []string{"checks: nodes.login.status == 200"},
			ports:   []string{"done", "fail"},
		},
		{
			name: "route_by_status_with_four_cases",
			node: model.Node{ID: "r", Type: "condition", Cases: []model.Case{
				{When: "nodes.login.status == 200"}, {When: "nodes.login.status == 404"}, {When: "nodes.login.status >= 500"},
			}, Else: []model.Node{{ID: "x"}}},
			subline: "by status",
			ports:   []string{"case-0", "case-1", "case-2", "else"},
		},
		{
			name:    "route_by_value",
			node:    model.Node{ID: "r", Type: "condition", Cases: []model.Case{{When: "vars.plan == 'pro'"}}},
			subline: "by value",
			ports:   []string{"case-0", "else"},
		},
		{
			name: "loop_over_a_list",
			node: model.Node{ID: "l", Type: "batch", Params: fields("over", "{{nodes.list.body.items}}", "concurrency", int64(3)),
				Body: []model.Node{{ID: "a"}, {ID: "b"}}},
			subline: "over nodes.list.body.items, 3 at a time",
			rows:    []string{"steps: 2 steps"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "loop_n_times",
			node:    model.Node{ID: "l", Type: "batch", Params: fields("times", int64(5)), Body: []model.Node{{ID: "a"}}},
			subline: "5 times",
			rows:    []string{"steps: 1 step"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "loop_until",
			node:    model.Node{ID: "l", Type: "run_till", Params: fields("until", "nodes.poll.body.done", "max", int64(10)), Body: []model.Node{{ID: "a"}}},
			subline: "until nodes.poll.body.done, max 10",
			rows:    []string{"steps: 1 step"},
			ports:   []string{"done", "fail"},
		},
		{
			name: "group_in_parallel",
			node: model.Node{ID: "g", Type: "parallel", Params: fields("wait", "all"),
				Branches: []model.Branch{{ID: "x"}, {ID: "y"}}},
			subline: "together, wait for all",
			rows:    []string{"steps: 2 branches"},
			ports:   []string{"done", "fail"},
		},
		{name: "wait_fixed", node: step("w", "delay", "duration", "2s"), subline: "2s", ports: []string{"done", "fail"}},
		{name: "wait_range", node: step("w", "delay", "duration", "1s..3s"), subline: "1s to 3s", ports: []string{"done", "fail"}},
		{
			name:    "output_to_log",
			node:    step("o", "log", "value", "hello {{vars.who}}"),
			subline: "Run log",
			rows:    []string{"values: hello {{vars.who}}"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "output_to_file",
			node:    step("o", "save", "key", "report", "value", "{{nodes.a}}", "mode", "append"),
			subline: "File (append)",
			rows:    []string{"values: {{nodes.a}}"},
			ports:   []string{"done", "fail"},
		},
		{
			name:    "stop_as_failed",
			node:    step("x", "stop", "status", "failed", "message", "no token"),
			subline: "as failed",
			rows:    []string{"message: no token"},
			ports:   []string{"fail"},
		},
		{name: "note", node: step("n", "note", "text", "remember the cookie"), subline: "remember the cookie", ports: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := card.Summarize(tt.node, tt.facts)

			if got.Subline != tt.subline {
				t.Fatalf("subline %q, want %q", got.Subline, tt.subline)
			}
			if r := rowsOf(got.Rows); strings.Join(r, "|") != strings.Join(tt.rows, "|") {
				t.Fatalf("rows %q, want %q", r, tt.rows)
			}
			if p := portIDs(got.Ports); strings.Join(p, "|") != strings.Join(tt.ports, "|") {
				t.Fatalf("ports %v, want %v", p, tt.ports)
			}
		})
	}
}

func TestSummarize_Given_a_route_with_twelve_cases_Then_every_case_keeps_its_own_port_with_a_short_label(t *testing.T) {
	var cases []model.Case
	for range 12 {
		cases = append(cases, model.Case{When: "nodes.login.body.some.very.long.path.that.goes.on.and.on == 'value'"})
	}

	got := card.Summarize(model.Node{ID: "r", Type: "condition", Cases: cases}, card.Facts{})

	if len(got.Ports) != 13 {
		t.Fatalf("%d ports, want 12 cases and else", len(got.Ports))
	}
	for _, p := range got.Ports {
		if n := len([]rune(p.Label)); n > 24 {
			t.Fatalf("port %s label has %d runes", p.ID, n)
		}
	}
}

func TestSummarize_Given_a_request_that_fails_over_to_a_step_Then_ports_stay_done_and_fail(t *testing.T) {
	n := model.Node{ID: "r", Type: "request", OnError: "goto:later", Request: &model.RequestCall{}}

	got := card.Summarize(n, card.Facts{Kind: "http", Method: "GET", Target: "/x"})

	if p := portIDs(got.Ports); strings.Join(p, "|") != "done|fail" {
		t.Fatalf("ports %v", p)
	}
	if got.Ports[1].Tone != "fail" || got.Ports[0].Tone != "done" {
		t.Fatalf("tones %+v", got.Ports)
	}
}

func TestSummarize_Given_status_cases_Then_port_labels_are_short(t *testing.T) {
	tests := []struct{ when, want string }{
		{"nodes.login.status == 200", "== 200"},
		{"nodes.login.status == 404", "== 404"},
		{"status != 204", "!= 204"},
		{"nodes.login.status >= 500 && nodes.login.status < 600", "5xx"},
		{"nodes.a.status >= 400 && nodes.a.status < 500", "4xx"},
		{"nodes.a.status >= 400 && nodes.a.status < 600", "nodes.a.status >= 400 &…"}, // not one class: shown as written, cut
		{"vars.plan == 'pro'", "vars.plan == 'pro'"},
	}
	for _, tt := range tests {
		t.Run(tt.when, func(t *testing.T) {
			n := model.Node{ID: "r", Type: "condition", Cases: []model.Case{{When: tt.when}}}

			got := card.Summarize(n, card.Facts{}).Ports[0].Label

			if got != tt.want {
				t.Fatalf("label %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSummarize_Given_a_check_with_a_schema_Then_it_has_a_row_for_it(t *testing.T) {
	// Arrange
	n := model.Node{ID: "c", Type: "assert", Params: fields("check", "status == 200", "schema", "login")}

	// Act
	got := card.Summarize(n, card.Facts{})

	// Assert
	if got.Subline != "2 checks" || len(got.Rows) != 2 || got.Rows[1].Summary != "matches the schema of login" {
		t.Fatalf("%+v", got)
	}
}

func TestSummarize_Given_a_fuzz_step_Then_the_card_says_what_it_sends_and_what_it_expects(t *testing.T) {
	// Arrange
	plain := model.Node{ID: "f", Type: "fuzz", Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}}}
	strict := model.Node{ID: "f", Type: "fuzz", Params: fields("count", int64(50), "expect", "rejected"), Request: &model.RequestCall{Request: model.Ref{Path: "auth/login"}}}

	// Act
	a, b := card.Summarize(plain, card.Facts{}), card.Summarize(strict, card.Facts{})

	// Assert
	if a.Subline != "20 bad bodies" || len(a.Rows) != 2 || a.Rows[0].Summary != "auth/login" || a.Rows[1].Summary != "fails on a server error" {
		t.Fatalf("plain %+v", a)
	}
	if b.Subline != "50 bad bodies" || b.Rows[1].Summary != "fails when a bad body is accepted" {
		t.Fatalf("strict %+v", b)
	}
	if len(a.Ports) != 2 || a.Ports[0].ID != "done" {
		t.Fatalf("ports %+v", a.Ports)
	}
}
