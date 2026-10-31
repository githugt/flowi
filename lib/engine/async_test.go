// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// scripted is a transport that records the operation it was given and
// answers with canned messages, applying the operation's Match like a real one.
type scripted struct {
	got      transport.Operation
	incoming []string
	err      error
	block    bool // wait for the context to end
}

func (s *scripted) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	s.got = op
	if s.block {
		<-ctx.Done()
		return transport.Result{}, context.Cause(ctx)
	}
	res := transport.Result{}
	if op.Send != nil {
		sent := *op.Send
		sent.Direction = "sent"
		res.Sent = &sent
	}
	if op.Receive != nil {
		for _, body := range s.incoming {
			m := transport.Message{Direction: "received", Body: []byte(body), Headers: []transport.Header{{Name: "type", Value: "order"}}}
			if op.Receive.Match != nil {
				ok, err := op.Receive.Match(m)
				if err != nil { // a real transport stops the receive, as transport.Wait does
					return res, fmt.Errorf("matching a message: %w", err)
				}
				if !ok {
					continue
				}
			}
			res.Received = append(res.Received, m)
			if op.Receive.OnMessage != nil {
				op.Receive.OnMessage(m)
			}
			if op.Receive.OnKept != nil {
				if err := op.Receive.OnKept(m); err != nil { // a real transport stops the receive, as transport.Wait does
					return res, err
				}
			}
		}
	}
	return res, s.err
}

func asyncVersion(a model.Async) model.RequestVersion {
	if a.Protocol == "" {
		a.Protocol = "mqtt"
	}
	if a.Server == "" {
		a.Server = "mqtt://broker:1883"
	}
	return model.RequestVersion{Async: &a}
}

func sendAsync(t *testing.T, tr *scripted, v model.RequestVersion, tweak func(*SendInput)) (SendResult, *fixture) {
	t.Helper()
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": tr, "ws": tr}
	r := f.addRequest("orders/placed", v)
	in := f.input(r)
	if tweak != nil {
		tweak(&in)
	}
	res, _ := f.send(in)
	return res, f
}

func TestSend_Async_should_send_a_message_and_record_it(t *testing.T) {
	tr := &scripted{}
	v := asyncVersion(model.Async{Channel: "orders/{{id}}", Send: &model.AsyncSend{
		Body: model.Body{JSON: `{"id":"{{id}}"}`}, Headers: []model.KeyValue{{Key: "type", Value: "order"}}}})

	res, _ := sendAsync(t, tr, v, func(in *SendInput) { in.With = map[string]string{"id": "7"} })

	if res.Status != StatusPassed || res.Record == nil {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	rec := res.Record
	if rec.Kind != KindAsync || rec.Outcome != OutcomeOK || rec.Status != 0 || rec.Request.Method != "MQTT" {
		t.Fatalf("record %+v", rec)
	}
	if len(rec.Messages) != 1 || rec.Messages[0].Direction != "sent" || rec.Messages[0].Body.Text != `{"id":"7"}` {
		t.Fatalf("messages %+v", rec.Messages)
	}
	if tr.got.Channel != "orders/7" || string(tr.got.Send.Body) != `{"id":"7"}` || tr.got.Send.Headers[0].Value != "order" {
		t.Fatalf("operation %+v", tr.got)
	}
}

func TestSend_Async_should_receive_and_check_extract_and_assert(t *testing.T) {
	tr := &scripted{incoming: []string{`{"id":"x","n":1}`, `{"id":"y","n":2}`}}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{Count: 2}})
	v.Assert = []string{`received == 2`, `body.id == "x"`, `messages[1].body.n == 2`, `outcome == "ok"`}
	v.Extract = []model.Extraction{{Name: "first", From: "body", Path: "$.id"}}

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	rec := res.Record
	if len(rec.Messages) != 2 || rec.Body.JSON == nil || rec.Outputs["first"] != "x" {
		t.Fatalf("record %+v", rec)
	}
}

func TestSend_Async_should_fail_a_false_assertion(t *testing.T) {
	tr := &scripted{incoming: []string{`{"id":"x"}`}}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{}})
	v.Assert = []string{`body.id == "nope"`}

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusFailed {
		t.Fatalf("status %s", res.Status)
	}
}

func TestSend_Async_should_only_count_messages_that_match(t *testing.T) {
	tr := &scripted{incoming: []string{`{"id":1}`, `{"id":2}`}}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{Match: `body.id == 2`}})

	res, _ := sendAsync(t, tr, v, nil)

	if got := res.Record.Messages; len(got) != 1 || !strings.Contains(got[0].Body.Text, `"id":2`) {
		t.Fatalf("messages %+v", got)
	}
}

func TestSend_Async_should_run_hooks_around_the_transport(t *testing.T) {
	tr := &scripted{incoming: []string{`{"ok":true}`}}
	v := asyncVersion(model.Async{Channel: "{{vars.topic}}", Receive: &model.AsyncReceive{}})
	v.Hooks = model.Hooks{
		Pre:  []model.HookStep{inlineHook(setNode("s", "vars.topic", "orders"))},
		Post: []model.HookStep{inlineHook(assertNode("a", "received == 1 and body.ok"))},
	}

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusPassed || tr.got.Channel != "orders" {
		t.Fatalf("status %s channel %q diags %v", res.Status, tr.got.Channel, res.Diags)
	}
}

func TestSend_Async_should_report_a_timeout_and_keep_the_partial_messages(t *testing.T) {
	tr := &scripted{incoming: []string{`{"id":1}`}, err: transport.ErrTimeout}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{Count: 3, Timeout: time.Second}})

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAsyncTimeout) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if res.Record == nil || res.Record.Outcome != OutcomeTimeout || len(res.Record.Messages) != 1 {
		t.Fatalf("record %+v", res.Record)
	}
}

func TestSend_Async_should_report_a_transport_failure(t *testing.T) {
	tr := &scripted{err: context.DeadlineExceeded}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{}})

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeHTTPFailed) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Async_should_not_leak_secrets(t *testing.T) {
	tr := &scripted{incoming: []string{`{"echo":"hunter2"}`}}
	v := asyncVersion(model.Async{
		Send:    &model.AsyncSend{Body: model.Body{JSON: `{"pw":"{{secret.password}}"}`}},
		Receive: &model.AsyncReceive{},
		Binding: []model.KeyValue{{Key: "password", Value: "{{secret.password}}"}, {Key: "username", Value: "u"}},
	})

	res, _ := sendAsync(t, tr, v, nil)

	if tr.got.Binding["password"] != secretV {
		t.Fatalf("the transport must get the real secret, got %q", tr.got.Binding["password"])
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Fatalf("secret in result: %s", data)
	}
}

func TestSend_Async_should_not_record_a_password_written_in_the_server_url(t *testing.T) {
	tr := &scripted{}
	v := asyncVersion(model.Async{Server: "mqtt://app:plain-pw@broker:1883", Channel: "orders", Send: &model.AsyncSend{}})

	res, _ := sendAsync(t, tr, v, nil)

	if tr.got.Server != "mqtt://app:plain-pw@broker:1883" {
		t.Fatalf("the transport must get the real server, got %q", tr.got.Server)
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), "plain-pw") {
		t.Fatalf("password in result: %s", data)
	}
}

func TestSend_Async_should_stop_when_cancelled(t *testing.T) {
	tr := &scripted{block: true}
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": tr}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Receive: &model.AsyncReceive{Timeout: time.Minute}}))
	ctx, cancel := context.WithCancel(context.Background())
	s := StartSend(ctx, f.deps(), f.input(r))

	cancel()
	for range s.Events() {
	}
	res := s.Wait()

	if res.Status != StatusCancelled {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Async_should_save_a_record_that_loads_back(t *testing.T) {
	tr := &scripted{incoming: []string{`{"id":1}`}}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{}})

	res, _ := sendAsync(t, tr, v, func(in *SendInput) { in.Save = "per-status" })

	if len(res.Saved) != 2 || !strings.HasSuffix(res.Saved[0], "ok.json") {
		t.Fatalf("saved %v diags %v", res.Saved, res.Diags)
	}
	data, err := os.ReadFile(res.Saved[1])
	if err != nil {
		t.Fatal(err)
	}
	var back Record
	if err := json.Unmarshal(data, &back); err != nil || back.Kind != KindAsync || len(back.Messages) != 1 {
		t.Fatalf("loaded %+v err %v", back, err)
	}
}

func TestSend_Async_should_refuse_a_protocol_with_no_transport(t *testing.T) {
	f := newFixture(t)
	f.transports = transport.Registry{}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Receive: &model.AsyncReceive{}}))

	res, _ := f.send(f.input(r))

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAsyncProtocol) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestPreviewSend_Async_should_show_the_protocol_target_and_payload(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Channel: "orders/{{id}}",
		Send:    &model.AsyncSend{Body: model.Body{JSON: `{"pw":"{{secret.password}}","id":"{{id}}"}`}},
		Binding: []model.KeyValue{{Key: "qos", Value: "1"}}}))
	in := f.input(r)
	in.With = map[string]string{"id": "7"}

	p := PreviewSend(f.deps(), in, true)

	if p.Diags.HasErrors() || p.Masked.Method != "MQTT" || p.Masked.URL != "mqtt://broker:1883/orders/7" || p.Masked.BodyKind != "text" {
		t.Fatalf("masked %+v diags %v", p.Masked, p.Diags)
	}
	if strings.Contains(p.Masked.Body, secretV) || !strings.Contains(p.Real.Body, secretV) {
		t.Fatalf("masked %q real %q", p.Masked.Body, p.Real.Body)
	}
}

func TestSend_Async_should_report_the_request_and_its_outcome_as_events(t *testing.T) {
	f := newFixture(t)
	tr := &scripted{incoming: []string{`{"ok":true}`}}
	f.transports = transport.Registry{"nats": tr}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Protocol: "nats", Server: "localhost:4222", Channel: "orders",
		Receive: &model.AsyncReceive{}}))

	res, events := f.send(f.input(r))

	var sent *RequestSent
	var got *ResponseReceived
	for _, ev := range events {
		switch e := ev.(type) {
		case RequestSent:
			sent = &e
		case ResponseReceived:
			got = &e
		}
	}
	if res.Status != StatusPassed || sent == nil || got == nil {
		t.Fatalf("status %s sent %v received %v", res.Status, sent, got)
	}
	if sent.Method != "NATS" || sent.URL != "nats://localhost:4222/orders" || got.Error != "" || got.Outcome != OutcomeOK || got.Depth != 0 {
		t.Fatalf("sent %+v received %+v", sent, got)
	}
	if tr.got.Server != "nats://localhost:4222" {
		t.Fatalf("the transport gets the server with its scheme, got %q", tr.got.Server)
	}
}

func TestSend_Async_should_report_a_failed_attempt_with_its_error(t *testing.T) {
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": &scripted{err: context.DeadlineExceeded}}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Receive: &model.AsyncReceive{}}))

	_, events := f.send(f.input(r))

	for _, ev := range events {
		if e, ok := ev.(ResponseReceived); ok {
			if e.Error == "" || e.Outcome != OutcomeError {
				t.Fatalf("received %+v", e)
			}
			return
		}
	}
	t.Fatal("no ResponseReceived")
}

// Before: a message without the field the filter reads (`body.type` on
// {"action": "x"}) stopped the whole receive with "undefined body.type". A
// filter is there to skip messages, so a missing field is a non-match.
func TestSend_Async_match_should_skip_a_message_that_lacks_the_field_it_reads(t *testing.T) {
	tr := &scripted{incoming: []string{`{"action":"something-else"}`, `plain text`, `{"type":"other"}`, `{"type":"refresh","id":2}`}}
	v := asyncVersion(model.Async{Receive: &model.AsyncReceive{Match: `body.type == "refresh"`}})

	res, _ := sendAsync(t, tr, v, nil)

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if got := res.Record.Messages; len(got) != 1 || !strings.Contains(got[0].Body.Text, `"id":2`) {
		t.Fatalf("messages %+v", got)
	}
}

func TestSend_Async_should_report_each_kept_message_while_it_waits(t *testing.T) {
	f := newFixture(t)
	f.transports = transport.Registry{"mqtt": &announcing{incoming: []string{`{"n":1}`, `{"n":2}`}}}
	r := f.addRequest("orders/placed", asyncVersion(model.Async{Receive: &model.AsyncReceive{Count: 2}}))

	_, events := f.send(f.input(r))

	var got []MessageReceived
	for _, ev := range events {
		if e, ok := ev.(MessageReceived); ok {
			got = append(got, e)
		}
	}
	if len(got) != 2 || got[0].Seq != 1 || got[1].Seq != 2 || got[0].Message.Direction != "received" {
		t.Fatalf("events %+v", got)
	}
}

// announcing is a transport that calls OnMessage for each message, as Wait does.
type announcing struct{ incoming []string }

func (a *announcing) Run(_ context.Context, op transport.Operation) (transport.Result, error) {
	var res transport.Result
	for _, body := range a.incoming {
		m := transport.Message{Direction: "received", Body: []byte(body)}
		res.Received = append(res.Received, m)
		if op.Receive.OnMessage != nil {
			op.Receive.OnMessage(m)
		}
	}
	return res, nil
}

func TestSend_Async_message_hooks_should_run_for_each_message_and_read_it(t *testing.T) {
	// Arrange
	tr := &scripted{incoming: []string{`{"n":1}`, `{"n":2}`, `{"n":3}`}}
	v := asyncVersion(model.Async{Channel: "orders", Receive: &model.AsyncReceive{Count: 3}})
	v.Hooks = model.Hooks{Message: []model.HookStep{inlineHook(
		assertNode("seen", "body.n == seq"),
		setNode("last", "vars.last", "{{body.n}}"),
	)}}

	// Act
	res, _ := sendAsync(t, tr, v, nil)

	// Assert
	if res.Status != StatusPassed || res.Record == nil || len(res.Record.Messages) != 3 {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	if res.Vars["last"] != "3" && res.Vars["last"] != 3 {
		t.Errorf("the hook ran for the last message too: vars %v", res.Vars)
	}
}

func TestSend_Async_message_hooks_should_stop_at_the_first_message_that_fails_a_check(t *testing.T) {
	// Arrange
	tr := &scripted{incoming: []string{`{"ok":true}`, `{"ok":false}`, `{"ok":true}`}}
	v := asyncVersion(model.Async{Channel: "orders", Receive: &model.AsyncReceive{Count: 3}})
	v.Hooks = model.Hooks{Message: []model.HookStep{inlineHook(assertNode("must-be-ok", "body.ok"))}}

	// Act
	res, _ := sendAsync(t, tr, v, nil)

	// Assert
	if res.Status != StatusFailed || res.Record == nil {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
	if res.Record.Outcome != OutcomeStopped || len(res.Record.Messages) != 2 {
		t.Errorf("outcome %q with %d messages: the run must stop at the second message, which is kept", res.Record.Outcome, len(res.Record.Messages))
	}
	if hasCode(res.Diags, diag.CodeHTTPFailed) {
		t.Errorf("the failed hook has reported; the run must not add a transport failure: %v", res.Diags)
	}
}

func TestSend_Async_message_hooks_should_be_skippable_and_ignored_without_a_receive(t *testing.T) {
	// Arrange
	tr := &scripted{incoming: []string{`{"ok":false}`}}
	failing := model.Hooks{Message: []model.HookStep{inlineHook(assertNode("must-be-ok", "body.ok"))}}
	receiving := asyncVersion(model.Async{Channel: "orders", Receive: &model.AsyncReceive{}})
	receiving.Hooks = failing
	sending := asyncVersion(model.Async{Channel: "orders", Send: &model.AsyncSend{Body: model.Body{Text: "x"}}})
	sending.Hooks = failing

	// Act
	skipped, _ := sendAsync(t, tr, receiving, func(in *SendInput) { in.Hooks = model.HookControl{Skip: []string{"message"}} })
	sendOnly, _ := sendAsync(t, tr, sending, nil)

	// Assert
	if skipped.Status != StatusPassed || sendOnly.Status != StatusPassed {
		t.Errorf("skipped %v %v, send only %v %v", skipped.Status, skipped.Diags, sendOnly.Status, sendOnly.Diags)
	}
}
