// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func request(id, path string, with ...model.KeyValue) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{Path: path}, With: with}}
}

func sentenceTexts(list []Sentence) []string {
	var out []string
	for _, s := range list {
		out = append(out, s.Text)
	}
	return out
}

func TestSentences_should_say_each_step_in_order_with_what_it_takes_from_earlier_steps(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		request("create", "pets/createPet"),
		request("get", "pets/getPet", model.KeyValue{Key: "petId", Value: "{{nodes.create.body.id}}"}),
		{ID: "ok", Type: "assert", Params: []model.Field{{Key: "check", Value: "nodes.get.status == 200"}}},
	}}

	got := Sentences(f)

	want := []string{
		"Call pets/createPet",
		"Call pets/getPet, with petId = create.body.id",
		"Check that nodes.get.status == 200",
	}
	if !slices.Equal(sentenceTexts(got), want) {
		t.Fatalf("got %q", sentenceTexts(got))
	}
	if got[1].Node != "get" || got[1].Indent != 0 {
		t.Fatalf("sentence %+v", got[1])
	}
}

func TestSentences_should_indent_the_branches_of_a_condition_and_the_lanes_of_a_parallel_step(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "cond", Type: "condition", Params: []model.Field{{Key: "check", Value: "nodes.login.status == 200"}},
			Then: []model.Node{request("a", "ok")},
			Else: []model.Node{request("b", "fail")}},
		{ID: "par", Type: "parallel", Branches: []model.Branch{
			{ID: "left", Nodes: []model.Node{request("c", "left")}},
			{ID: "right", Nodes: []model.Node{request("d", "right")}},
		}},
	}}

	got := Sentences(f)

	wantText := []string{
		"If nodes.login.status == 200:", "Call ok", "Otherwise:", "Call fail",
		"At the same time:", "Branch left:", "Call left", "Branch right:", "Call right",
	}
	wantIndent := []int{0, 1, 0, 1, 0, 1, 2, 1, 2}
	var indent []int
	for _, s := range got {
		indent = append(indent, s.Indent)
	}
	if !slices.Equal(sentenceTexts(got), wantText) || !slices.Equal(indent, wantIndent) {
		t.Fatalf("text %q\nindent %v", sentenceTexts(got), indent)
	}
}

func TestSentences_should_say_what_a_set_step_stores_and_where_from(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "keep", Type: "set", Values: []model.SetValue{
		{Name: "token", From: "response", Where: "body", Path: "token"},
		{Name: "email", From: "generate"},
		{Name: "region", From: "literal", Value: "eu"},
		{Name: "total", Value: "{{vars.a}} + 1"},
	}}}}

	got := Sentences(f)

	want := `Set token from the response body (token), email to a generated value, region to "eu" and total to a + 1`
	if len(got) != 1 || got[0].Text != want {
		t.Fatalf("got %q, want %q", sentenceTexts(got), want)
	}
}

func TestSentences_should_name_loops_waits_and_a_condition_on_the_step_itself(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "each", Type: "batch", Params: []model.Field{{Key: "over", Value: "{{vars.ids}}"}}, Body: []model.Node{request("one", "item")}},
		{ID: "wait", Type: "delay", Params: []model.Field{{Key: "duration", Value: "2s"}}},
		{ID: "poll", Type: "run_till", Params: []model.Field{{Key: "until", Value: "nodes.job.body.done"}}, Body: []model.Node{request("job", "job")}},
		{ID: "maybe", Type: "log", When: "vars.debug", Params: []model.Field{{Key: "value", Value: "hi"}}},
	}}

	got := sentenceTexts(Sentences(f))

	want := []string{
		"For each item of ids:", "Call item",
		"Wait 2s",
		"Repeat until nodes.job.body.done:", "Call job",
		`Write "hi" to the log, only when vars.debug`,
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestExplain_should_flag_a_variable_nothing_reads_and_one_nothing_sets(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{
		{ID: "keep", Type: "set", Values: []model.SetValue{{Name: "unused", Value: "1"}, {Name: "used", Value: "2"}}},
		{ID: "log", Type: "log", Params: []model.Field{{Key: "value", Value: "{{vars.used}} {{vars.missing}}"}}},
	}}

	got := Explain(f).Problems

	want := []Problem{
		{Node: "keep", Text: "unused is set but nothing reads it"},
		{Node: "log", Text: "missing is read but no step sets it"},
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestExplain_Given_a_variable_only_a_flow_output_reads_Then_it_is_not_flagged(t *testing.T) {
	f := model.Flow{
		Outputs: []model.Output{{Name: "token", Value: "{{vars.token}}"}},
		Nodes:   []model.Node{{ID: "keep", Type: "set", Values: []model.SetValue{{Name: "token", Value: "x"}}}},
	}

	got := Explain(f).Problems

	if len(got) != 0 {
		t.Fatalf("got %+v", got)
	}
}

func TestSentences_Given_a_step_missing_its_configuration_Then_still_says_something_and_does_not_panic(t *testing.T) {
	f := model.Flow{Nodes: []model.Node{{ID: "a", Type: "request"}, {ID: "b", Type: "fuzz"}, {ID: "c", Type: "subflow"}, {ID: "d", Type: "future"}}}

	got := sentenceTexts(Sentences(f))

	want := []string{"Call a request", "Send bad bodies to a request", "Run another flow", "Run the future step"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}
