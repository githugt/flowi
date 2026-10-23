// SPDX-License-Identifier: Apache-2.0

package nats_test

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/nats"
)

// startServer runs an embedded NATS server with JetStream and returns its URL.
func startServer(t *testing.T, tweak func(*natsserver.Options)) string {
	t.Helper()
	opts := &natsserver.Options{Host: "127.0.0.1", Port: -1, JetStream: true, StoreDir: t.TempDir(), NoLog: true, NoSigs: true}
	if tweak != nil {
		tweak(opts)
	}
	srv, err := natsserver.NewServer(opts)
	if err != nil {
		t.Fatal(err)
	}
	go srv.Start()
	if !srv.ReadyForConnections(10 * time.Second) {
		t.Fatal("nats server did not start")
	}
	t.Cleanup(func() { srv.Shutdown(); srv.WaitForShutdown() })
	return srv.ClientURL()
}

func connect(t *testing.T, url string, opts ...natsgo.Option) *natsgo.Conn {
	t.Helper()
	nc, err := natsgo.Connect(url, opts...)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	return nc
}

func makeStream(t *testing.T, url, name string, subjects ...string) jetstream.JetStream {
	t.Helper()
	js, err := jetstream.New(connect(t, url))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := js.CreateStream(t.Context(), jetstream.StreamConfig{Name: name, Subjects: subjects}); err != nil {
		t.Fatal(err)
	}
	return js
}

func bodies(ms []transport.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.Body))
	}
	return out
}

func header(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

func TestRun_should_publish_with_headers(t *testing.T) {
	url := startServer(t, nil)
	nc := connect(t, url)
	got := make(chan *natsgo.Msg, 1)
	if _, err := nc.ChanSubscribe("orders.placed", got); err != nil {
		t.Fatal(err)
	}
	if err := nc.Flush(); err != nil {
		t.Fatal(err)
	}

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed",
		Send: &transport.Message{Body: []byte("hi"), Headers: []transport.Header{{Name: "type", Value: "order"}}},
	})

	if err != nil {
		t.Fatal(err)
	}
	m := <-got
	if string(m.Data) != "hi" || m.Header.Get("type") != "order" {
		t.Fatalf("server got %q %v", m.Data, m.Header)
	}
	if res.Sent == nil || res.Sent.Direction != "sent" {
		t.Fatalf("sent %+v", res.Sent)
	}
}

func TestRun_should_receive_core_messages(t *testing.T) {
	tests := []struct {
		name    string
		binding map[string]string
	}{
		{name: "plain"},
		{name: "queue_group", binding: map[string]string{"queue_group": "workers"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := startServer(t, nil)
			pub := connect(t, url)
			ctx, stop := context.WithCancel(t.Context())
			defer stop()
			go func() { // publish until the receive is done: the subscription's start is not observable
				tick := time.NewTicker(5 * time.Millisecond)
				defer tick.Stop()
				for {
					select {
					case <-ctx.Done():
						return
					case <-tick.C:
						_ = pub.Publish("events.x", []byte("tick"))
					}
				}
			}()

			res, err := nats.New().Run(t.Context(), transport.Operation{
				Server: url, Channel: "events.x", Binding: tt.binding, Receive: &transport.Receive{Count: 2, Timeout: 5 * time.Second},
			})

			if err != nil {
				t.Fatal(err)
			}
			if len(res.Received) != 2 || header(res.Received[0], "subject") != "events.x" {
				t.Fatalf("received %+v", res.Received)
			}
		})
	}
}

func TestRun_should_get_the_reply_on_reply_to_and_not_its_own_message(t *testing.T) {
	url := startServer(t, nil)
	responder := connect(t, url)
	if _, err := responder.Subscribe("svc.echo", func(m *natsgo.Msg) { _ = responder.Publish(m.Reply, append([]byte("echo:"), m.Data...)) }); err != nil {
		t.Fatal(err)
	}
	if err := responder.Flush(); err != nil {
		t.Fatal(err)
	}

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "svc.echo", Binding: map[string]string{"reply_to": "svc.reply.1"},
		Send: &transport.Message{Body: []byte("ping")}, Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); len(got) != 1 || got[0] != "echo:ping" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_not_get_its_own_message_on_the_same_subject(t *testing.T) {
	url := startServer(t, nil)

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "same.subject", Send: &transport.Message{Body: []byte("mine")},
		Receive: &transport.Receive{Timeout: 150 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 0 {
		t.Fatalf("err %v received %v", err, bodies(res.Received))
	}
}

func TestRun_should_authenticate(t *testing.T) {
	tests := []struct {
		name    string
		tweak   func(*natsserver.Options)
		binding map[string]string
	}{
		{"token", func(o *natsserver.Options) { o.Authorization = "s3cret" }, map[string]string{"token": "s3cret"}},
		{"user_password", func(o *natsserver.Options) { o.Username, o.Password = "u", "p" }, map[string]string{"username": "u", "password": "p"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := startServer(t, tt.tweak)
			op := transport.Operation{Server: url, Channel: "a.b", Send: &transport.Message{Body: []byte("x")}}

			_, bad := nats.New().Run(t.Context(), op)
			op.Binding = tt.binding
			_, good := nats.New().Run(t.Context(), op)

			if bad == nil {
				t.Fatal("a connection without credentials must be refused")
			}
			if good != nil {
				t.Fatalf("with credentials: %v", good)
			}
		})
	}
}

func TestRun_should_publish_to_a_stream_and_report_its_sequence(t *testing.T) {
	url := startServer(t, nil)
	js := makeStream(t, url, "ORDERS", "orders.>")

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "ORDERS"},
		Send: &transport.Message{Body: []byte("one")},
	})

	if err != nil {
		t.Fatal(err)
	}
	if res.Sent == nil || res.Sent.Sequence != 1 {
		t.Fatalf("sent %+v", res.Sent)
	}
	stream, _ := js.Stream(t.Context(), "ORDERS")
	if info, _ := stream.Info(t.Context()); info.State.Msgs != 1 {
		t.Fatalf("stream holds %d messages", info.State.Msgs)
	}
}

func TestRun_should_name_an_unknown_stream(t *testing.T) {
	url := startServer(t, nil)

	_, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "NOPE"}, Send: &transport.Message{Body: []byte("x")},
	})

	if err == nil || !strings.Contains(err.Error(), "NOPE") {
		t.Fatalf("err %v", err)
	}
}

func TestRun_should_read_a_stream_from_the_start(t *testing.T) {
	url := startServer(t, nil)
	js := makeStream(t, url, "ORDERS", "orders.>")
	for i := 1; i <= 3; i++ {
		if _, err := js.Publish(t.Context(), "orders.placed", []byte(fmt.Sprint("m", i))); err != nil {
			t.Fatal(err)
		}
	}

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "ORDERS", "deliver": "all"},
		Receive: &transport.Receive{Count: 2, Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); len(got) != 2 || got[0] != "m1" || got[1] != "m2" {
		t.Fatalf("received %v", got)
	}
	if res.Received[1].Sequence != 2 {
		t.Fatalf("sequence %d", res.Received[1].Sequence)
	}
}

func TestRun_should_resume_a_named_consumer_after_the_last_acked_message(t *testing.T) {
	url := startServer(t, nil)
	js := makeStream(t, url, "ORDERS", "orders.>")
	for i := 1; i <= 3; i++ {
		if _, err := js.Publish(t.Context(), "orders.placed", []byte(fmt.Sprint("m", i))); err != nil {
			t.Fatal(err)
		}
	}
	op := transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "ORDERS", "consumer": "c1", "deliver": "all"},
		Receive: &transport.Receive{Timeout: 5 * time.Second},
	}

	first, err1 := nats.New().Run(t.Context(), op)
	second, err2 := nats.New().Run(t.Context(), op)

	if err1 != nil || err2 != nil {
		t.Fatal(err1, err2)
	}
	if a, b := bodies(first.Received), bodies(second.Received); len(a) != 1 || a[0] != "m1" || len(b) != 1 || b[0] != "m2" {
		t.Fatalf("first %v second %v", a, b)
	}
}

func TestRun_should_get_a_stream_reply_but_not_its_own_message(t *testing.T) {
	url := startServer(t, nil)
	js := makeStream(t, url, "RPC", "rpc.>")
	responder := connect(t, url)
	if _, err := responder.Subscribe("rpc.ask", func(m *natsgo.Msg) {
		if string(m.Data) == "question" {
			_, _ = js.Publish(context.Background(), "rpc.ask", []byte("answer"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := responder.Flush(); err != nil {
		t.Fatal(err)
	}

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "rpc.ask", Binding: map[string]string{"stream": "RPC"},
		Send: &transport.Message{Body: []byte("question")}, Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := bodies(res.Received); len(got) != 1 || got[0] != "answer" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_time_out_with_the_partial_messages(t *testing.T) {
	url := startServer(t, nil)
	js := makeStream(t, url, "ORDERS", "orders.>")
	_, _ = js.Publish(t.Context(), "orders.placed", []byte("only"))

	res, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "ORDERS", "deliver": "all"},
		Receive: &transport.Receive{Count: 2, Timeout: 300 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_should_reject_a_bad_deliver_option(t *testing.T) {
	url := startServer(t, nil)
	makeStream(t, url, "ORDERS", "orders.>")

	_, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "orders.placed", Binding: map[string]string{"stream": "ORDERS", "deliver": "yesterday"},
		Receive: &transport.Receive{Timeout: time.Second},
	})

	if err == nil || !strings.Contains(err.Error(), "deliver") {
		t.Fatalf("err %v", err)
	}
}

// Before: any TLS configuration made the client demand TLS from a plain
// nats:// server ("nats: secure connection not available"). The engine always
// passes one.
func TestRun_should_connect_to_a_plain_server_even_when_given_a_tls_config(t *testing.T) {
	url := startServer(t, nil)

	_, err := nats.New().Run(t.Context(), transport.Operation{
		Server: url, Channel: "a.b", TLS: &tls.Config{MinVersion: tls.VersionTLS12}, Send: &transport.Message{Body: []byte("x")},
	})

	if err != nil {
		t.Fatal(err)
	}
}

func TestRun_should_accept_a_server_written_as_host_and_port(t *testing.T) {
	url := startServer(t, nil)

	_, err := nats.New().Run(t.Context(), transport.Operation{
		Server: strings.TrimPrefix(url, "nats://"), Channel: "a.b", Send: &transport.Message{Body: []byte("x")},
	})

	if err != nil {
		t.Fatal(err)
	}
}
