// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/graphqlws"
)

const tickQuery = "subscription Ticks($from: Int) { tick(from: $from) }"

func subscription(g model.GraphQL, mutate func(*model.RequestVersion)) model.RequestVersion {
	if g.Query == "" {
		g.Query = tickQuery
	}
	return nativeGraphQL(g, mutate)
}

// sendSubscription sends v with the scripted transport standing in for the WebSocket.
func sendSubscriptionWith(t *testing.T, tr *scripted, v model.RequestVersion, tweak func(*SendInput)) SendResult {
	t.Helper()
	f := newFixture(t)
	f.transports = transport.Registry{model.ProtocolGraphQLWS: tr}
	in := f.input(f.addRequest("gql/ticks", v))
	if tweak != nil {
		tweak(&in)
	}
	res, _ := f.send(in)
	return res
}

// Before: a subscription was refused with FLW-GQL-001 and nothing was sent.
func TestSend_Given_a_subscription_Then_it_subscribes_over_the_transport_and_records_the_events(t *testing.T) {
	tr := &scripted{incoming: []string{`{"data":{"tick":1}}`, `{"data":{"tick":2}}`}}
	v := subscription(model.GraphQL{Variables: `{"from": {{from}}}`, Operation: "Ticks", Receive: &model.AsyncReceive{Count: 2}}, func(v *model.RequestVersion) {
		v.Headers = []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.password}}"}}
		v.Assert = []string{`received == 2`, `body.data.tick == 1`, `messages[1].body.data.tick == 2`}
		v.Extract = []model.Extraction{{Name: "first", From: "body", Path: "$.data.tick"}}
	})

	res := sendSubscriptionWith(t, tr, v, func(in *SendInput) { in.With = map[string]string{"from": "5"} })

	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	rec := res.Record
	if rec.Kind != KindAsync || rec.Request.Method != "GQL" || rec.Proto != model.ProtocolGraphQLWS || len(rec.Messages) != 3 {
		t.Fatalf("record kind %s method %s proto %s messages %d", rec.Kind, rec.Request.Method, rec.Proto, len(rec.Messages))
	}
	if fmtSprint(rec.Outputs["first"]) != "1" {
		t.Errorf("outputs %v", rec.Outputs)
	}
	var sent map[string]any
	if err := json.Unmarshal(tr.got.Send.Body, &sent); err != nil {
		t.Fatal(err)
	}
	vars, _ := sent["variables"].(map[string]any)
	// a value that came in as text stays text, as in a JSON body
	if sent["query"] != tickQuery || sent["operationName"] != "Ticks" || fmtSprint(vars["from"]) != `"5"` {
		t.Errorf("subscribe payload %s", tr.got.Send.Body)
	}
	if !strings.HasSuffix(tr.got.Server, "/graphql") || len(tr.got.Headers) != 1 || tr.got.Headers[0].Value != "Bearer "+secretV {
		t.Errorf("server %q headers %+v", tr.got.Server, tr.got.Headers)
	}
	if strings.Contains(rec.Request.Header[0].Value, secretV) {
		t.Errorf("the recorded header shows the secret: %+v", rec.Request.Header)
	}
}

func fmtSprint(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func TestSend_Given_a_subscription_event_with_errors_Then_the_step_fails(t *testing.T) {
	tr := &scripted{incoming: []string{`{"data":null,"errors":[{"message":"not allowed"}]}`}}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{}, nil), nil)

	if res.Status != StatusFailed || !hasCode(res.Diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_subscription_that_asserts_on_the_errors_Then_they_are_the_expected_result(t *testing.T) {
	tr := &scripted{incoming: []string{`{"data":null,"errors":[{"message":"not allowed"}]}`}}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{}, func(v *model.RequestVersion) {
		v.Assert = []string{`body.errors[0].message == "not allowed"`}
	}), nil)

	if res.Status != StatusPassed || hasCode(res.Diags, diag.CodeRunGraphQLError) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_subscription_match_Then_only_matching_events_count(t *testing.T) {
	tr := &scripted{incoming: []string{`{"data":{"tick":1}}`, `{"data":{"tick":2}}`}}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{Receive: &model.AsyncReceive{Match: `body.data.tick == 2`}}, nil), nil)

	if res.Status != StatusPassed || len(res.Record.Messages) != 2 || res.Record.Messages[1].Body.Text != `{"data":{"tick":2}}` {
		t.Fatalf("status %s messages %+v diags %v", res.Status, res.Record.Messages, res.Diags)
	}
}

func TestSend_Given_a_subscription_the_server_refuses_Then_the_diagnostic_carries_its_reason(t *testing.T) {
	tr := &scripted{err: serverErr("the server closed the connection with code 4401 (unauthorized): bad token")}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{}, nil), nil)

	var found diag.Diagnostic
	for _, d := range res.Diags {
		if d.Code == diag.CodeGraphQLSubscription {
			found = d
		}
	}
	if res.Status != StatusError || !strings.Contains(found.Args["detail"], "4401") {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_subscription_with_a_template_in_the_query_Then_it_is_refused(t *testing.T) {
	tr := &scripted{}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{Query: "subscription { tick(from: {{from}}) }"}, nil), nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeGraphQLRequest) || tr.got.Send != nil {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestSend_Given_a_subscription_that_never_delivers_Then_it_times_out_saying_how_many_arrived(t *testing.T) {
	tr := &scripted{err: transport.ErrTimeout, incoming: []string{`{"data":{"tick":1}}`}}

	res := sendSubscriptionWith(t, tr, subscription(model.GraphQL{Receive: &model.AsyncReceive{Count: 3, Timeout: time.Second}}, nil), nil)

	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAsyncTimeout) {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
}

func TestPreview_Given_a_subscription_Then_it_shows_the_subscribe_message_not_a_post(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("gql/ticks", subscription(model.GraphQL{Variables: `{"from": 1}`}, nil))

	p := PreviewSend(f.deps(), f.input(r), false)

	if p.Masked.Method != "GQL" || !strings.Contains(p.Masked.Body, `"query":"subscription Ticks`) {
		t.Fatalf("preview %+v", p.Masked)
	}
}

// serverErr is an error as the transport reports a refusal by the server.
func serverErr(detail string) error { return &graphqlws.ServerError{Detail: detail} }

// subscriptionServer is a GraphQL endpoint that speaks graphql-transport-ws on GET /graphql-ws.
func subscriptionServer(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"graphql-transport-ws"}})
	if err != nil {
		return
	}
	defer func() { _ = c.CloseNow() }()
	ctx := r.Context()
	read := func() map[string]any {
		_, data, err := c.Read(ctx)
		if err != nil {
			return nil
		}
		var m map[string]any
		_ = json.Unmarshal(data, &m)
		return m
	}
	write := func(s string) { _ = c.Write(ctx, websocket.MessageText, []byte(s)) }
	if m := read(); m == nil || m["type"] != "connection_init" {
		return
	}
	write(`{"type":"connection_ack"}`)
	sub := read()
	if sub == nil {
		return
	}
	payload, _ := json.Marshal(sub["payload"])
	write(`{"id":"1","type":"next","payload":{"data":{"echo":` + string(payload) + `,"authorization":"` + r.Header.Get("Authorization") + `"}}}`)
	write(`{"id":"1","type":"next","payload":{"data":{"tick":2}}}`)
	write(`{"id":"1","type":"complete"}`)
}

func TestSend_Given_a_subscription_and_a_real_server_Then_the_events_come_over_the_websocket(t *testing.T) {
	f := newFixture(t)
	v := subscription(model.GraphQL{Variables: `{"from": 1}`, Receive: &model.AsyncReceive{Count: 5, Timeout: 5 * time.Second}}, func(v *model.RequestVersion) {
		v.URL = "{{env.base_url}}/graphql-ws" // an http URL: the subscription uses the same address over ws
		v.Headers = []model.KeyValue{{Key: "Authorization", Value: "token-1"}}
	})

	res, _ := f.send(f.input(f.addRequest("gql/ticks", v)))

	if res.Status != StatusPassed || res.Record == nil || len(res.Record.Messages) != 3 { // the subscribe message and two events: the server completes after two
		t.Fatalf("status %s diags %v record %+v", res.Status, res.Diags, res.Record)
	}
	first := res.Record.Messages[1].Body.JSON.(map[string]any)["data"].(map[string]any)
	echo, _ := first["echo"].(map[string]any)
	if echo["operationName"] != nil && echo["operationName"] != "" || first["authorization"] != "token-1" || !strings.Contains(echo["query"].(string), "subscription Ticks") {
		t.Errorf("first event %v", first)
	}
}
