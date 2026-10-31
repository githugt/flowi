// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/internal/vm"
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
