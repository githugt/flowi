// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Final review, Critical 1: a request node whose call is cut short by its
// timeout or by cancellation must not report passed.
func TestRequestNode_Given_timeout_on_real_HTTP_Then_FLW_RUN_007_failed(t *testing.T) {
	f := newFixture(t)
	slow := f.addRequest("slow", get("/slow"))
	n := requestNode("call", slow)
	n.Timeout = 50 * time.Millisecond
	fl := f.addFlow("timeout-http", n, flowNode("after", "note", "text", "x"))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	call, _ := finishedNode(events, "call")
	after, _ := finishedNode(events, "after")
	if call.Status != StatusFailed || !hasCode(call.Diags, diag.CodeRunNodeTimeout) || after.Status != StatusSkipped || res.Status != StatusFailed {
		t.Fatalf("call %+v after %s run %s", call, after.Status, res.Status)
	}
}

func TestRequestNode_Given_cancel_mid_request_in_batch_Then_iterations_cancelled(t *testing.T) {
	f := newFixture(t)
	hold := f.addRequest("hold", get("/hold"))
	batch := batchOf("each", []model.Field{{Key: "times", Value: int64(2)}, {Key: "concurrency", Value: int64(2)}}, requestNode("call", hold))
	fl := f.addFlow("cancel-http", batch)
	ctx, cancel := context.WithCancel(testCtx(t))

	run := StartRun(ctx, f.deps(), f.runInput(fl))
	<-f.held
	<-f.held
	cancel()
	events, res := drain(run)

	each, _ := finishedNode(events, "each")
	first, _ := finishedNode(events, "each[0]/call")
	if res.Status != StatusCancelled || each.Status != StatusCancelled || first.Status != StatusCancelled {
		t.Fatalf("run %s each %s first %s", res.Status, each.Status, first.Status)
	}
}

// Final review, Critical 2: a background webhook in a send's hook flow
// must finish before the send's event stream closes (it used to emit after
// the close and crash with "send on closed channel").
func TestSend_Given_background_webhook_in_hook_Then_send_waits_for_it(t *testing.T) {
	f := newFixture(t)
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{inlineHook(flowNode("ping", "webhook", "url", "{{env.base_url}}/hold", "wait", false))}
	r := f.addRequest("x", v)
	s := StartSend(testCtx(t), f.deps(), f.input(r))
	events := s.Events()
	var seen []Event
	for ev := range events {
		seen = append(seen, ev)
		if rr, ok := ev.(ResponseReceived); ok && rr.Path == "" {
			break // the send's own response is in; the webhook is still held
		}
	}
	<-f.held

	// While the webhook is held, the stream must stay open (bounded check).
	select {
	case ev, open := <-events:
		if !open {
			t.Fatal("the send ended while its background webhook was still running")
		}
		seen = append(seen, ev)
	case <-time.After(200 * time.Millisecond):
	}
	close(f.hold)
	for ev := range events {
		seen = append(seen, ev)
	}
	res := s.Wait()

	pinged := false
	for _, ev := range seen {
		if rr, ok := ev.(ResponseReceived); ok && rr.Path == "pre:version:0/ping" {
			pinged = true
		}
	}
	if res.Status != StatusPassed || !pinged {
		t.Fatalf("status %s, webhook response seen %v (events %v)", res.Status, pinged, eventNames(seen))
	}
}

// Final review, Important 3: a template error in a flow request node is a
// failed node (exit 1), not a system error.
func TestRequestNode_Given_template_error_Then_failed_not_error(t *testing.T) {
	f := newFixture(t)
	bad := f.addRequest("bad", get("/echo/{{vars.missing}}"))
	fl := f.addFlow("tpl", requestNode("call", bad))

	events, res := runFlow(testCtx(t), t, f.deps(), f.runInput(fl))

	call, _ := finishedNode(events, "call")
	if call.Status != StatusFailed || res.Status != StatusFailed || !hasCode(call.Diags, diag.CodeTemplateUndefined) {
		t.Fatalf("call %+v run %s", call, res.Status)
	}
}

// Final review, Important 5: hook flows of a standalone send use the
// send's persistent cookie jar.
func TestSend_hook_flow_fetch_should_use_the_send_cookie_jar(t *testing.T) {
	f := newFixture(t)
	set := f.addRequest("set", get("/cookie/set"))
	if res, _ := f.send(f.input(set)); res.Status != StatusPassed {
		t.Fatalf("set cookie: %+v", res)
	}
	v := get("/echo/")
	v.Hooks.Pre = []model.HookStep{inlineHook(
		flowNode("look", "fetch", "url", "{{env.base_url}}/cookie/echo"),
		flowNode("keep", "extract", "seen", "{{nodes.look.body}}"),
	)}
	r := f.addRequest("x", v)

	res, _ := f.send(f.input(r))

	if res.Status != StatusPassed || res.Vars["seen"] != "s-1" {
		t.Fatalf("status %s vars %v diags %v", res.Status, res.Vars, res.Diags)
	}
}

// Final review, Minor 7 (graded Important): when the flow has finished
// and the run is cancelled at the same moment, awaitFlow must end
// cancelled at once — not wait out the grace period and report aborted.
// Both channels are ready, so select picks either; 50 rounds hit both.
func TestAwaitFlow_Given_flow_done_and_ctx_cancelled_Then_cancelled_without_grace_wait(t *testing.T) {
	for range 50 {
		r := newRunState(Deps{NewID: func() string { return "R" }, Clock: time.Now, After: newFakeClock().After}, 1, make(chan Event, 8))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		done := make(chan listOutcome, 1)
		done <- listOutcome{status: StatusPassed}
		result := make(chan listOutcome, 1)

		go func() { result <- r.awaitFlow(ctx, done, time.Minute) }()

		select {
		case out := <-result:
			if out.status != StatusCancelled {
				t.Fatalf("status %s", out.status)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("awaitFlow waited for the grace period although the flow had finished")
		}
	}
}
