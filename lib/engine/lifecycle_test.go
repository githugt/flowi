// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
)

// An endpoint that announces its end (Deprecation, Sunset) is worth a warning on the send that heard it, and the
// send still passes: the endpoint works today.
func TestRequestNode_Given_a_response_with_deprecation_headers_Then_a_warning_names_what_was_announced_and_the_step_passes(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("legacy", get("/legacy"))
	fl := f.addFlow("legacy", requestNode("call", r))

	// Act
	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	call, _ := finishedNode(events, "call")
	var warning diag.Diagnostic
	for _, d := range call.Diags {
		if d.Code == diag.CodeRunDeprecated {
			warning = d
		}
	}
	if res.Status != StatusPassed || warning.Code == "" || warning.Severity != diag.SeverityWarning {
		t.Fatalf("status %v diags %v", res.Status, call.Diags)
	}
	if msg := warning.Args["notice"]; !strings.Contains(msg, "deprecated since 2023-06-30") || !strings.Contains(msg, "2033-12-31") || !strings.Contains(msg, "/v2/legacy") {
		t.Fatalf("message %q", msg)
	}
}

func TestRequestNode_Given_a_response_without_deprecation_headers_Then_no_lifecycle_warning(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", get("/echo/"))
	fl := f.addFlow("plain", requestNode("call", r))

	// Act
	events, _ := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	call, _ := finishedNode(events, "call")
	if hasCode(call.Diags, diag.CodeRunDeprecated) {
		t.Fatalf("diags %v", call.Diags)
	}
}

// A server that reports its quota (the IETF draft's RateLimit field) lets a flow read what is left and wait it out.
func TestRequestNode_Given_a_response_that_reports_its_quota_Then_the_flow_can_read_it_and_wait_for_it(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("limited", get("/limited"))
	check := flowNode("check", "assert", "check", "nodes.call.ratelimit.remaining == 0 && nodes.call.ratelimit.limit == 100 && nodes.call.ratelimit.reset == 30s && nodes.call.ratelimit.wait == 30s")
	fl := f.addFlow("limited", requestNode("call", r), check)

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
}

func TestRequestNode_Given_quota_left_Then_wait_is_zero(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("roomy", get("/roomy"))
	check := flowNode("check", "assert", "check", "nodes.call.ratelimit.remaining == 7 && nodes.call.ratelimit.wait == 0s")
	fl := f.addFlow("roomy", requestNode("call", r), check)

	// Act
	_, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("result %+v", res)
	}
}
