// SPDX-License-Identifier: Apache-2.0

package mqtt_test

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/listeners"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/mqtt"
)

// broker starts an in-process MQTT broker and returns its server URL.
// rules, when set, restrict who may connect.
func broker(t *testing.T, rules auth.AuthRules) (*mochi.Server, string) {
	t.Helper()
	srv := mochi.New(&mochi.Options{InlineClient: true, Logger: slog.New(slog.DiscardHandler)})
	if rules == nil {
		_ = srv.AddHook(new(auth.AllowHook), nil)
	} else {
		_ = srv.AddHook(new(auth.Hook), &auth.Options{Ledger: &auth.Ledger{Auth: rules}})
	}
	ln, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := srv.AddListener(&listener{id: "t", ln: ln}); err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve() }()
	t.Cleanup(func() { _ = srv.Close() })
	return srv, "mqtt://" + addr
}

// listener serves a net.Listener to the broker. mochi's own TCP listener
// shares one error variable between its accept loop and the goroutines it
// starts, which the race detector rightly flags.
type listener struct {
	id     string
	ln     net.Listener
	closed atomic.Bool
}

func (l *listener) Init(*slog.Logger) error { return nil }
func (l *listener) ID() string              { return l.id }
func (l *listener) Address() string         { return l.ln.Addr().String() }
func (l *listener) Protocol() string        { return "tcp" }

func (l *listener) Serve(establish listeners.EstablishFn) {
	for {
		conn, err := l.ln.Accept()
		if err != nil {
			return
		}
		go func() { _ = establish(l.id, conn) }()
	}
}

func (l *listener) Close(closeClients listeners.CloseFn) {
	if l.closed.CompareAndSwap(false, true) {
		closeClients(l.id)
		_ = l.ln.Close()
	}
}

func TestRun_should_publish_with_qos_retain_and_user_properties(t *testing.T) {
	srv, server := broker(t, nil)
	got := make(chan packets.Packet, 1)
	if err := srv.Subscribe("orders/#", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) { got <- pk }); err != nil {
		t.Fatal(err)
	}

	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders/placed",
		Send:    &transport.Message{Body: []byte(`{"id":1}`), Headers: []transport.Header{{Name: "type", Value: "order"}}},
		Binding: map[string]string{"qos": "1", "retain": "true"},
	})

	if err != nil {
		t.Fatal(err)
	}
	pk := <-got
	if string(pk.Payload) != `{"id":1}` || pk.TopicName != "orders/placed" {
		t.Fatalf("broker got %q on %q", pk.Payload, pk.TopicName)
	}
	if len(pk.Properties.User) != 1 || pk.Properties.User[0].Key != "type" || pk.Properties.User[0].Val != "order" {
		t.Fatalf("user properties %+v", pk.Properties.User)
	}
	if res.Sent == nil || res.Sent.Direction != "sent" {
		t.Fatalf("sent %+v", res.Sent)
	}
}

func TestRun_should_receive_retained_messages_with_topic_and_user_properties(t *testing.T) {
	srv, server := broker(t, nil)
	if err := srv.Publish("orders/placed", []byte("one"), true, 1); err != nil {
		t.Fatal(err)
	}

	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders/+", Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || string(res.Received[0].Body) != "one" {
		t.Fatalf("received %+v", res.Received)
	}
	headers := map[string]string{}
	for _, h := range res.Received[0].Headers {
		headers[h.Name] = h.Value
	}
	if headers["topic"] != "orders/placed" {
		t.Fatalf("headers %v", headers)
	}
}

func TestRun_should_get_the_reply_but_not_its_own_message(t *testing.T) {
	srv, server := broker(t, nil)
	var once sync.Once
	if err := srv.Subscribe("rpc/ask", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) {
		if pk.Origin == mochi.InlineClientId {
			return // the reply itself
		}
		once.Do(func() { _ = srv.Publish("rpc/ask", []byte("answer"), false, 0) })
	}); err != nil {
		t.Fatal(err)
	}

	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "rpc/ask", Send: &transport.Message{Body: []byte("question")},
		Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || string(res.Received[0].Body) != "answer" {
		t.Fatalf("received %+v (own message must not come back)", res.Received)
	}
}

func TestRun_should_authenticate_with_username_and_password(t *testing.T) {
	_, server := broker(t, auth.AuthRules{{Username: "u", Password: "p", Allow: true}})
	op := transport.Operation{Server: server, Channel: "t", Send: &transport.Message{Body: []byte("x")}}

	_, bad := mqtt.New().Run(t.Context(), op)
	op.Binding = map[string]string{"username": "u", "password": "p"}
	_, good := mqtt.New().Run(t.Context(), op)

	if bad == nil {
		t.Fatal("a connection without credentials must be refused")
	}
	if good != nil {
		t.Fatalf("with credentials: %v", good)
	}
}

func TestRun_should_time_out_with_the_partial_messages(t *testing.T) {
	srv, server := broker(t, nil)
	_ = srv.Publish("t", []byte("only"), true, 0)

	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "t", Receive: &transport.Receive{Count: 2, Timeout: 200 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_should_return_promptly_when_cancelled(t *testing.T) {
	_, server := broker(t, nil)
	done := make(chan error, 1)
	cctx, stop := context.WithCancel(t.Context())

	go func() {
		_, err := mqtt.New().Run(cctx, transport.Operation{Server: server, Channel: "t", Receive: &transport.Receive{Timeout: time.Minute}})
		done <- err
	}()
	stop()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("want an error after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRun_should_reject_what_it_cannot_dial(t *testing.T) {
	tests := []struct{ name, server, binding string }{
		{"unknown_scheme", "gopher://h", ""},
		{"bad_qos", "mqtt://127.0.0.1:1", "qos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op := transport.Operation{Server: tt.server, Channel: "t", Send: &transport.Message{}}
			if tt.binding != "" {
				op.Binding = map[string]string{tt.binding: "nine"}
			}

			_, err := mqtt.New().Run(t.Context(), op)

			if err == nil || strings.TrimSpace(err.Error()) == "" {
				t.Fatalf("err %v", err)
			}
		})
	}
}
