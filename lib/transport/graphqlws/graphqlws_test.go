// SPDX-License-Identifier: Apache-2.0

package graphqlws_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/graphqlws"
)

// peer is the server side of one connection: it speaks graphql-transport-ws frames.
type peer struct {
	ctx  context.Context
	conn *websocket.Conn
	req  *http.Request
}

func (p peer) read(t *testing.T) map[string]any {
	t.Helper()
	_, data, err := p.conn.Read(p.ctx)
	if err != nil {
		t.Errorf("server read: %v", err)
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("server read %q: %v", data, err)
	}
	return m
}

func (p peer) send(t *testing.T, frame string) {
	t.Helper()
	if err := p.conn.Write(p.ctx, websocket.MessageText, []byte(frame)); err != nil {
		t.Errorf("server write: %v", err)
	}
}

// ack waits for connection_init and acknowledges it.
func (p peer) ack(t *testing.T) {
	t.Helper()
	if m := p.read(t); m["type"] != "connection_init" {
		t.Errorf("first message %v, want connection_init", m)
	}
	p.send(t, `{"type":"connection_ack"}`)
}

func (p peer) hold() {
	for {
		if _, _, err := p.conn.Read(p.ctx); err != nil {
			return
		}
	}
}

// serve starts a server whose handler gets the open connection after the subprotocol is chosen.
func serve(t *testing.T, subprotocols []string, handle func(t *testing.T, p peer)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: subprotocols})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		handle(t, peer{ctx: r.Context(), conn: c, req: r})
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

// protocol is what a server that speaks graphql-transport-ws accepts (a function: nothing global can be changed).
func protocol() []string { return []string{graphqlws.Subprotocol} }

func subscribeOp(server string, count int) transport.Operation {
	return transport.Operation{
		Server:  server,
		Send:    &transport.Message{Body: []byte(`{"query":"subscription S { tick }","operationName":"S","variables":{"n":1}}`)},
		Receive: &transport.Receive{Count: count, Timeout: 5 * time.Second},
	}
}

func bodies(ms []transport.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.Body))
	}
	return out
}

func TestRun_should_subscribe_and_keep_the_payload_of_each_next_message(t *testing.T) {
	var subscribe map[string]any
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		subscribe = p.read(t)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":1}}}`)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":2}}}`)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":3}}}`)
		p.hold()
	})

	res, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 2)) // an http URL: the endpoint is the same over ws

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); strings.Join(got, "|") != `{"data":{"tick":1}}|{"data":{"tick":2}}` {
		t.Errorf("received %v", got)
	}
	if res.Received[0].Type != "next" || res.Received[0].Direction != "received" {
		t.Errorf("message %+v", res.Received[0])
	}
	payload, _ := subscribe["payload"].(map[string]any)
	if subscribe["type"] != "subscribe" || subscribe["id"] != "1" || payload["query"] != "subscription S { tick }" || payload["operationName"] != "S" {
		t.Errorf("subscribe frame %v", subscribe)
	}
	if res.Sent == nil || res.Sent.Type != "subscribe" || res.Sent.Direction != "sent" {
		t.Errorf("sent %+v", res.Sent)
	}
}

func TestRun_should_answer_a_ping_with_a_pong(t *testing.T) {
	ponged := make(chan bool, 1)
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		p.read(t)
		p.send(t, `{"type":"ping"}`)
		got := p.read(t)
		ponged <- got["type"] == "pong"
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":1}}}`)
		p.hold()
	})

	_, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 1))

	if err != nil {
		t.Fatal(err)
	}
	if !<-ponged {
		t.Error("the client did not answer the ping with a pong")
	}
}

func TestRun_should_end_without_failing_when_the_server_completes_early(t *testing.T) {
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		p.read(t)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":1}}}`)
		p.send(t, `{"id":"1","type":"complete"}`)
		p.hold()
	})

	res, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 5))

	if err != nil {
		t.Fatalf("a finished stream failed the run: %v", err)
	}
	if len(res.Received) != 1 {
		t.Errorf("received %v", bodies(res.Received))
	}
}

func TestRun_should_fail_with_the_message_of_an_error_frame(t *testing.T) {
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		p.read(t)
		p.send(t, `{"id":"1","type":"error","payload":[{"message":"Cannot query field \"tock\" on type \"Subscription\""}]}`)
		p.hold()
	})

	_, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 1))

	var se *graphqlws.ServerError
	if !errors.As(err, &se) || !strings.Contains(se.Detail, `Cannot query field "tock"`) {
		t.Fatalf("error %v", err)
	}
}

func TestRun_should_say_what_a_close_code_means(t *testing.T) {
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.read(t) // connection_init
		_ = p.conn.Close(4401, "bad token")
	})

	_, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 1))

	if err == nil || !strings.Contains(err.Error(), "4401") || !strings.Contains(err.Error(), "unauthorized") || !strings.Contains(err.Error(), "bad token") {
		t.Fatalf("error %v", err)
	}
}

func TestRun_should_time_out_with_what_arrived(t *testing.T) {
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		p.read(t)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{"tick":1}}}`)
		p.hold()
	})
	op := subscribeOp(server, 3)
	op.Receive.Timeout = 200 * time.Millisecond

	res, err := graphqlws.New().Run(t.Context(), op)

	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("error %v", err)
	}
	if len(res.Received) != 1 {
		t.Errorf("received %v", bodies(res.Received))
	}
}

func TestRun_should_refuse_a_server_that_does_not_speak_the_protocol(t *testing.T) {
	// Before: a subscription was refused for every server; now a server that picks another subprotocol is.
	server := serve(t, nil, func(_ *testing.T, p peer) { p.hold() })

	_, err := graphqlws.New().Run(t.Context(), subscribeOp(server, 1))

	if err == nil || !strings.Contains(err.Error(), graphqlws.Subprotocol) {
		t.Fatalf("error %v", err)
	}
}

func TestRun_should_send_the_connection_headers_with_the_handshake(t *testing.T) {
	seen := make(chan string, 1)
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		seen <- p.req.Header.Get("Authorization")
		p.ack(t)
		p.read(t)
		p.send(t, `{"id":"1","type":"next","payload":{"data":{}}}`)
		p.hold()
	})
	op := subscribeOp(server, 1)
	op.Headers = []transport.Header{{Name: "Authorization", Value: "Bearer t0k3n"}}

	_, err := graphqlws.New().Run(t.Context(), op)

	if err != nil {
		t.Fatal(err)
	}
	if got := <-seen; got != "Bearer t0k3n" {
		t.Errorf("Authorization %q", got)
	}
}

func TestRun_should_not_name_the_path_of_the_url_when_it_cannot_connect(t *testing.T) {
	op := subscribeOp("http://127.0.0.1:1/graphql?token=hunter2", 1)

	_, err := graphqlws.New().Run(t.Context(), op)

	if err == nil || strings.Contains(err.Error(), "hunter2") {
		t.Fatalf("error %v", err)
	}
}

func TestProbe_should_connect_and_get_the_connection_acknowledged(t *testing.T) {
	server := serve(t, protocol(), func(t *testing.T, p peer) {
		p.ack(t)
		p.hold()
	})

	err := graphqlws.New().Probe(t.Context(), transport.Operation{Server: server})

	if err != nil {
		t.Fatal(err)
	}
}
