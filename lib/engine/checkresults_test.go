// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: the engine said only which checks failed, so a request with six passing checks could not say "6/6"
// and a passing tick was inferred from the absence of an error.
func TestSend_should_report_what_each_check_said_in_order(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("probe", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/echo/x",
		Assert: []string{"status == 200", "status == 201", "vars.nope != empty", "response.body.path == '/echo/x'"}})

	res, _ := f.send(f.input(r))

	var got []string
	for _, c := range res.Checks {
		got = append(got, c.Line+" -> "+c.Verdict)
	}
	want := []string{"status == 200 -> passed", "status == 201 -> failed", "vars.nope != empty -> error", "response.body.path == '/echo/x' -> passed"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
	if res.Checks[1].Note == "" || res.Checks[2].Note == "" {
		t.Errorf("a failed and an errored check should say why: %+v", res.Checks)
	}
}

func TestSend_Given_no_checks_Then_there_are_no_results(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("plain", get("/echo/x"))

	res, _ := f.send(f.input(r))

	if len(res.Checks) != 0 {
		t.Fatalf("%+v", res.Checks)
	}
}

// Before: a flow run's report knew only the failed checks.
func TestRunFlow_should_report_what_each_check_of_a_request_step_said(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("probe", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/echo/x", Assert: []string{"status == 200", "status == 201"}})
	fl := f.addFlow("checks", requestNode("call", r))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	done, _ := finishedNode(events, "call")
	var node *reportNodeChecks
	for _, n := range res.Report.Nodes {
		if n.ID == "call" {
			node = &reportNodeChecks{n.Checks[0].Verdict, n.Checks[1].Verdict, len(n.Checks)}
		}
	}
	if len(done.Checks) != 2 || node == nil || node.first != "passed" || node.second != "failed" || node.n != 2 {
		t.Fatalf("event %+v report %+v", done.Checks, node)
	}
}

type reportNodeChecks struct {
	first, second string
	n             int
}
