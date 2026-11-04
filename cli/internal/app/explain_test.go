// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// writeMessyFlow adds a flow to the Tools collection that sets a variable nothing reads and reads one nothing sets.
func writeMessyFlow(t *testing.T, e *sendEnv) {
	t.Helper()
	writeFile(t, filepath.Join(e.dir, "tools", "flows", "messy.arazzo.yaml"), `arazzo: 1.1.0
info:
  title: Messy
  version: 1.0.0
workflows:
  - workflowId: messy
    summary: Messy
    steps: []
    x-flowi-flow:
      id: 01JA00000000000000000000T9
      name: Messy
      nodes:
        - id: keep
          type: set
          values:
            - name: unused
              value: "1"
        - id: say
          type: log
          value: "{{vars.missing}}"
x-flowi-version: 1
`)
}

func TestExplain_should_tell_the_flow_one_sentence_per_step(t *testing.T) {
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "explain", "user-creation")

	if code != 0 {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
	for _, want := range []string{"Call auth/login, with email = inputs.email", "Set token to login.token and subject to login.subject", "Wait env.verify_wait"} {
		if !strings.Contains(out, want) {
			t.Errorf("output lacks %q:\n%s", want, out)
		}
	}
}

func TestExplain_Given_markdown_Then_each_sentence_is_a_list_item(t *testing.T) {
	e := newSendEnv(t)

	_, out, _ := e.run(context.Background(), "explain", "user-creation", "--format", "markdown")

	if !strings.HasPrefix(out, "- Call auth/login") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestExplain_Given_problems_Then_lists_them_and_exits_1(t *testing.T) {
	e := newSendEnv(t)
	writeMessyFlow(t, e)

	code, out, _ := e.run(context.Background(), "explain", "messy", "--problems")

	if code != 1 || !strings.Contains(out, "unused is set but nothing reads it") || !strings.Contains(out, "missing is read but no step sets it") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestExplain_Given_a_flow_with_no_problems_Then_problems_exits_0(t *testing.T) {
	e := newSendEnv(t)

	code, out, _ := e.run(context.Background(), "explain", "user-creation", "--problems")

	if code != 0 || !strings.Contains(out, "No problems found.") {
		t.Fatalf("exit %d\n%s", code, out)
	}
}

func TestExplain_should_print_json_when_asked(t *testing.T) {
	e := newSendEnv(t)

	code, out, _ := e.run(context.Background(), "explain", "user-creation", "--output", "json")

	var got struct {
		Sentences []struct {
			Text string `json:"text"`
			Node string `json:"node"`
		} `json:"sentences"`
		Problems []struct{} `json:"problems"`
	}
	if err := json.Unmarshal([]byte(out), &got); code != 0 || err != nil || len(got.Sentences) == 0 || got.Sentences[0].Node != "login" || got.Problems == nil {
		t.Fatalf("exit %d, %v\n%s", code, err, out)
	}
}

func TestExplain_Given_an_unknown_format_Then_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, _ := e.run(context.Background(), "explain", "user-creation", "--format", "html")

	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}
