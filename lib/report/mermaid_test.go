// SPDX-License-Identifier: Apache-2.0

package report

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func sequenceOf(t *testing.T, r Run) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteMermaid(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWriteMermaid_Given_calls_to_two_hosts_Then_each_host_is_a_participant_and_each_attempt_a_pair_of_arrows(t *testing.T) {
	// Arrange
	r := Run{FlowName: "Checkout", Nodes: []Node{
		{ID: "login", Type: "request", Status: "passed", Attempts: []Attempt{{N: 1, Method: "POST", URL: "https://api.test/v1/login?x=secret", Status: 200, Duration: 38 * time.Millisecond}}},
		{ID: "pay", Type: "request", Status: "failed", Attempts: []Attempt{
			{N: 1, Method: "POST", URL: "https://pay.test/charge", Status: 503, Duration: 20 * time.Millisecond},
			{N: 2, Method: "POST", URL: "https://pay.test/charge", Status: 200, Duration: 25 * time.Millisecond},
		}},
	}}

	// Act
	got := sequenceOf(t, r)

	// Assert
	for _, want := range []string{
		"sequenceDiagram", "title Checkout", "participant F as Flowi", "participant H1 as api.test", "participant H2 as pay.test",
		"F->>H1: login: POST /v1/login", "H1-->>F: 200 · 38 ms",
		"F->>H2: pay: POST /charge", "H2--xF: 503 · 20 ms", "H2-->>F: 200 · 25 ms",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the diagram lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "secret") || strings.Contains(got, "x=") {
		t.Errorf("a query string must not reach the diagram:\n%s", got)
	}
}

func TestWriteMermaid_Given_a_step_that_is_not_a_call_and_failed_Then_it_is_a_note_with_its_reason(t *testing.T) {
	// Arrange
	r := Run{Nodes: []Node{{ID: "check", Type: "assert", Status: "failed", Diagnostics: []Diagnostic{{Code: "FLW-RUN-001", Message: "assertion failed: status == 200; got 503"}}}}}

	// Act
	got := sequenceOf(t, r)

	// Assert
	if !strings.Contains(got, "Note over F: check failed: assertion failed: status == 200, got 503") {
		t.Fatalf("%s", got)
	}
}

func TestWriteMermaid_Given_a_call_that_got_no_answer_Then_the_arrow_back_is_a_cross_with_the_error(t *testing.T) {
	// Arrange
	r := Run{Nodes: []Node{{ID: "ping", Type: "request", Status: "error", Attempts: []Attempt{{N: 1, Method: "GET", URL: "https://down.test/ping", Error: "connection refused"}}}}}

	// Act
	got := sequenceOf(t, r)

	// Assert
	if !strings.Contains(got, "H1--xF: connection refused") {
		t.Fatalf("%s", got)
	}
}

func TestWriteMermaid_Given_calls_inside_a_loop_Then_they_are_in_order_and_unmarked_text_cannot_break_the_syntax(t *testing.T) {
	// Arrange
	r := Run{Nodes: []Node{{ID: "each", Type: "batch", Status: "passed", Lanes: []Lane{
		{Label: "#0", Nodes: []Node{{ID: "get", Path: "each[0]/get", Type: "request", Status: "passed", Attempts: []Attempt{{N: 1, Method: "GET", URL: "https://api.test/a;b#c", Status: 200}}}}},
		{Label: "#1", Nodes: []Node{{ID: "get", Path: "each[1]/get", Type: "request", Status: "passed", Attempts: []Attempt{{N: 1, Method: "GET", URL: "https://api.test/d", Status: 200}}}}},
	}}}}

	// Act
	got := sequenceOf(t, r)

	// Assert
	if strings.Count(got, "F->>H1") != 2 || strings.Contains(got, ";") && !strings.Contains(got, "sequenceDiagram") {
		t.Fatalf("%s", got)
	}
	if strings.Contains(strings.SplitN(got, "F->>H1", 2)[1], ";") {
		t.Errorf("a semicolon would end the line in Mermaid:\n%s", got)
	}
}

func TestSequenceOf_Given_calls_Then_the_structure_lists_hosts_and_steps_in_order_for_a_window_to_draw(t *testing.T) {
	// Arrange
	r := Run{FlowName: "Checkout", Nodes: []Node{
		{ID: "login", Attempts: []Attempt{{N: 1, Method: "POST", URL: "https://api.test/v1/login?x=1", Status: 200, Duration: 38 * time.Millisecond}}},
		{ID: "pay", Attempts: []Attempt{{N: 1, Method: "POST", URL: "https://pay.test/charge", Status: 503}}},
		{ID: "check", Type: "assert", Status: "failed", Diagnostics: []Diagnostic{{Message: "assertion failed"}}},
	}}

	// Act
	s := SequenceOf(r)

	// Assert
	if s.Title != "Checkout" || len(s.Hosts) != 2 || s.Hosts[0] != "api.test" || s.Hosts[1] != "pay.test" {
		t.Fatalf("%+v", s)
	}
	want := []SequenceStep{
		{Kind: "call", Host: 0, Text: "login: POST /v1/login"},
		{Kind: "reply", Host: 0, Text: "200 · 38 ms"},
		{Kind: "call", Host: 1, Text: "pay: POST /charge"},
		{Kind: "reply", Host: 1, Text: "503", Failed: true},
		{Kind: "note", Host: -1, Text: "check failed: assertion failed", Failed: true},
	}
	if len(s.Steps) != len(want) {
		t.Fatalf("steps %+v", s.Steps)
	}
	for i := range want {
		if s.Steps[i] != want[i] {
			t.Errorf("step %d = %+v, want %+v", i, s.Steps[i], want[i])
		}
	}
}
