// SPDX-License-Identifier: Apache-2.0

package ws_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/ws"
)

// serve starts a WebSocket server whose handler gets the open connection.
func serve(t *testing.T, handle func(ctx context.Context, c *websocket.Conn, r *http.Request)) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, &websocket.AcceptOptions{Subprotocols: []string{"v1"}})
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		handle(r.Context(), c, r)
	}))
	t.Cleanup(srv.Close)
	return "ws" + strings.TrimPrefix(srv.URL, "http")
}

// waitClosed holds a handler open until the client goes away: a hijacked
// connection's request context is not cancelled when the client closes it.
func waitClosed(ctx context.Context, c *websocket.Conn) {
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return
		}
	}
}

func texts(ms []transport.Message) []string {
	var out []string
	for _, m := range ms {
		out = append(out, string(m.Body))
	}
	return out
}

func TestRun_should_send_a_message_and_report_it(t *testing.T) {
	got := make(chan string, 1)
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		_, b, err := c.Read(ctx)
		if err == nil {
			got <- string(b)
		}
	})

	res, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Send: &transport.Message{Body: []byte("hello")},
	})

	if err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "hello" {
		t.Fatalf("server got %q", v)
	}
	if res.Sent == nil || res.Sent.Direction != "sent" || string(res.Sent.Body) != "hello" {
		t.Fatalf("sent %+v", res.Sent)
	}
}

func TestRun_should_receive_the_requested_number_of_messages(t *testing.T) {
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		for _, m := range []string{"a", "b", "c"} {
			if c.Write(ctx, websocket.MessageText, []byte(m)) != nil {
				return
			}
		}
		waitClosed(ctx, c)
	})

	res, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Receive: &transport.Receive{Count: 2, Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := texts(res.Received); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("received %v", got)
	}
	if res.Received[0].Direction != "received" {
		t.Fatalf("direction %q", res.Received[0].Direction)
	}
}

func TestRun_should_not_lose_an_instant_reply(t *testing.T) {
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		_, b, err := c.Read(ctx)
		if err == nil {
			_ = c.Write(ctx, websocket.MessageText, append([]byte("echo:"), b...))
		}
	})

	res, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Send: &transport.Message{Body: []byte("ping")},
		Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := texts(res.Received); len(got) != 1 || got[0] != "echo:ping" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_return_what_arrived_when_the_receive_times_out(t *testing.T) {
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		_ = c.Write(ctx, websocket.MessageText, []byte("only"))
		waitClosed(ctx, c)
	})

	res, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Receive: &transport.Receive{Count: 3, Timeout: 100 * time.Millisecond},
	})

	if !errors.Is(err, transport.ErrTimeout) {
		t.Fatalf("err %v, want ErrTimeout", err)
	}
	if got := texts(res.Received); len(got) != 1 || got[0] != "only" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_only_count_matching_messages(t *testing.T) {
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		for _, m := range []string{"skip", "keep"} {
			_ = c.Write(ctx, websocket.MessageText, []byte(m))
		}
		waitClosed(ctx, c)
	})
	match := func(m transport.Message) (bool, error) { return string(m.Body) == "keep", nil }

	res, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Receive: &transport.Receive{Timeout: 5 * time.Second, Match: match},
	})

	if err != nil {
		t.Fatal(err)
	}
	if got := texts(res.Received); len(got) != 1 || got[0] != "keep" {
		t.Fatalf("received %v", got)
	}
}

func TestRun_should_return_promptly_when_cancelled(t *testing.T) {
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) { waitClosed(ctx, c) })
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)

	go func() {
		_, err := ws.New().Run(ctx, transport.Operation{Server: server, Receive: &transport.Receive{Timeout: time.Minute}})
		done <- err
	}()
	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err %v, want context.Canceled", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancel")
	}
}

func TestRun_should_pass_headers_subprotocol_and_channel(t *testing.T) {
	seen := make(chan *http.Request, 1)
	server := serve(t, func(_ context.Context, c *websocket.Conn, r *http.Request) {
		seen <- r
		c.Close(websocket.StatusNormalClosure, "")
	})

	_, err := ws.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "events/live",
		Headers: []transport.Header{{Name: "X-Token", Value: "abc"}},
		Binding: map[string]string{"subprotocol": "v1"},
		Send:    &transport.Message{Body: []byte("x")},
	})

	if err != nil {
		t.Fatal(err)
	}
	r := <-seen
	if r.URL.Path != "/events/live" || r.Header.Get("X-Token") != "abc" || r.Header.Get("Sec-WebSocket-Protocol") != "v1" {
		t.Fatalf("path %q headers %v", r.URL.Path, r.Header)
	}
}

func TestRun_should_fail_when_the_handshake_is_refused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusForbidden) }))
	t.Cleanup(srv.Close)

	_, err := ws.New().Run(t.Context(), transport.Operation{
		Server: "ws" + strings.TrimPrefix(srv.URL, "http"), Send: &transport.Message{Body: []byte("x")},
	})

	if err == nil || !strings.Contains(err.Error(), "403") {
		t.Fatalf("err %v, want it to name status 403", err)
	}
}
