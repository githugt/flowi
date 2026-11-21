// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/lib/template"
)

func TestExpressionHelp_should_describe_every_context_with_names_and_examples(t *testing.T) {
	contexts := []vm.ExprContext{
		vm.ExprFlowTemplate, vm.ExprFlowWhen, vm.ExprFlowCheck, vm.ExprFlowTransform, vm.ExprFlowCollection,
		vm.ExprRequestFields, vm.ExprRequestCheck, vm.ExprMessageCheck, vm.ExprMessageFilter,
	}
	for _, c := range contexts {
		t.Run(string(c), func(t *testing.T) {
			h := vm.ExpressionHelp(c)

			if h.Title == "" || h.Intro == "" || len(h.Names) == 0 || len(h.Examples) == 0 || len(h.Tools) == 0 {
				t.Fatalf("incomplete help %+v", h)
			}
			for _, n := range h.Names {
				if n.Name == "" || n.What == "" || n.Example == "" {
					t.Fatalf("incomplete name %+v", n)
				}
			}
		})
	}
}

func TestExpressionHelp_Given_an_unknown_context_Then_gives_the_help_for_step_fields(t *testing.T) {
	if got := vm.ExpressionHelp("nowhere"); got.Title != vm.ExpressionHelp(vm.ExprFlowTemplate).Title {
		t.Fatalf("got %q", got.Title)
	}
}

func TestExpressionHelp_for_messages_should_list_what_a_message_run_offers(t *testing.T) {
	names := map[string]bool{}
	for _, n := range vm.ExpressionHelp(vm.ExprMessageCheck).Names {
		names[n.Name] = true
	}

	for _, want := range []string{"messages", "all_messages", "received", "outcome"} {
		if !names[want] {
			t.Errorf("missing %q in %v", want, names)
		}
	}
}

func TestGeneratorHelp_should_list_the_generators(t *testing.T) {
	if got := vm.GeneratorHelp(); len(got) < 10 || got[0].Name == "" {
		t.Fatalf("got %d generators", len(got))
	}
}

func TestExpressionHelp_for_outcome_rules_should_explain_matching_and_list_in_and_not_in(t *testing.T) {
	for _, c := range []vm.ExprContext{vm.ExprOutcomeRule, vm.ExprOutcomeRuleMessage, vm.ExprOutcomeRuleDataset} {
		t.Run(string(c), func(t *testing.T) {
			h := vm.ExpressionHelp(c)

			var tools string
			for _, tool := range h.Tools {
				tools += tool.Name + "|"
			}
			if !strings.Contains(h.Intro, "every outcome") || !strings.Contains(tools, "not in") || !strings.Contains(tools, "x in [a, b]") || len(h.Names) == 0 || len(h.Examples) == 0 {
				t.Fatalf("help %+v, tools %s", h, tools)
			}
		})
	}
}

// Before: the help left out sum, max, matches and the date helpers, although the engine has them, so a user who needed
// them wrote a longer expression or gave up.
func TestExpressionHelp_should_document_only_functions_the_engine_evaluates(t *testing.T) {
	probes := map[string]string{
		"sum": `{{sum([1, 2, 3])}}`, "mean": `{{mean([1, 2, 3])}}`, "max": `{{max([1, 5, 3])}}`, "min": `{{min([1, 5, 3])}}`,
		"abs": `{{abs(-2)}}`, "round": `{{round(2.6)}}`, "first": `{{first([4, 5])}}`, "last": `{{last([4, 5])}}`,
		"sort": `{{sort([3, 1, 2])}}`, "uniq": `{{uniq([1, 1, 2])}}`, "concat": `{{concat([1], [2])}}`,
		"keys": `{{keys({"a": 1})}}`, "values": `{{values({"a": 1})}}`, "now": `{{now()}}`,
		"date": `{{date("2024-01-02")}}`, "duration": `{{duration("90s")}}`,
		"toBase64": `{{toBase64("hi")}}`, "fromBase64": `{{fromBase64("aGk=")}}`,
		"matches": `{{"abc" matches "^a"}}`, "startsWith": `{{"abc" startsWith "a"}}`, "endsWith": `{{"abc" endsWith "c"}}`,
	}
	tools := ""
	for _, item := range vm.ExpressionHelp(vm.ExprFlowCheck).Tools {
		tools += item.Name + "\n"
	}
	offered := map[string]bool{}
	for _, s := range vm.ExpressionVocabulary(vm.ExprFlowCheck) {
		offered[s.Insert] = true
	}
	for name, src := range probes {
		tpl, err := template.Parse(src)
		if err == nil {
			_, err = tpl.Render(template.Scope{})
		}

		if err != nil {
			t.Errorf("%s: the engine rejects %s: %v", name, src, err)
		}
		if !strings.Contains(tools, name) {
			t.Errorf("%s is not in the help", name)
		}
		if !offered[name+"("] && !offered[name] {
			t.Errorf("%s is not offered as you type", name)
		}
	}
}
