// SPDX-License-Identifier: Apache-2.0

package sse_test

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/sse"
)

func stream(t *testing.T, body string, seen chan<- *http.Request) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if seen != nil {
			seen <- r
		}
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, body)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}

func TestRun_should_turn_events_into_messages(t *testing.T) {
	url := stream(t, "id: 7\nevent: tick\ndata: one\n\ndata: two\ndata: lines\n\n", nil)

	res, err := sse.New().Run(t.Context(), transport.Operation{Server: url, Receive: &transport.Receive{Count: 2, Timeout: 5 * time.Second}})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 2 || string(res.Received[0].Body) != "one" || string(res.Received[1].Body) != "two\nlines" {
		t.Fatalf("received %+v", res.Received)
	}
	headers := map[string]string{}
	for _, h := range res.Received[0].Headers {
		headers[h.Name] = h.Value
	}
	if headers["id"] != "7" || headers["event"] != "tick" {
		t.Fatalf("headers %v", headers)
	}
}

func TestRun_should_send_the_request_headers_and_last_event_id(t *testing.T) {
	seen := make(chan *http.Request, 1)
	url := stream(t, "data: x\n\n", seen)

	_, err := sse.New().Run(t.Context(), transport.Operation{
		Server: url + "/feed", Headers: []transport.Header{{Name: "Authorization", Value: "Bearer t"}},
		Binding: map[string]string{"last_event_id": "41"}, Receive: &transport.Receive{Timeout: 5 * time.Second},
	})

	if err != nil {
		t.Fatal(err)
	}
	r := <-seen
	if r.URL.Path != "/feed" || r.Header.Get("Authorization") != "Bearer t" || r.Header.Get("Last-Event-ID") != "41" || r.Header.Get("Accept") != "text/event-stream" {
		t.Fatalf("path %q headers %v", r.URL.Path, r.Header)
	}
}

func TestRun_should_time_out_with_the_partial_messages(t *testing.T) {
	url := stream(t, "data: only\n\n", nil)

	res, err := sse.New().Run(t.Context(), transport.Operation{Server: url, Receive: &transport.Receive{Count: 2, Timeout: 100 * time.Millisecond}})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_should_name_the_status_of_a_refused_request(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", http.StatusUnauthorized) }))
	t.Cleanup(srv.Close)

	_, err := sse.New().Run(t.Context(), transport.Operation{Server: srv.URL, Receive: &transport.Receive{}})

	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err %v", err)
	}
}

func TestRun_should_refuse_to_send(t *testing.T) {
	_, err := sse.New().Run(t.Context(), transport.Operation{Server: "http://x", Send: &transport.Message{Body: []byte("x")}})

	if err == nil {
		t.Fatal("SSE is one-way: sending must be an error")
	}
}

// An event ends at a blank line, as a browser's EventSource also requires:
// "data: x" followed by nothing is not delivered. (Typing a response by hand
// with nc and forgetting the empty line is the usual way to meet this.)
func TestRun_should_not_deliver_an_event_that_has_no_blank_line_after_it(t *testing.T) {
	url := stream(t, "data: unfinished\n", nil)

	res, err := sse.New().Run(t.Context(), transport.Operation{Server: url, Receive: &transport.Receive{Timeout: 200 * time.Millisecond}})

	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 0 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_should_keep_only_the_events_named_by_the_channel(t *testing.T) {
	body := "event: refresh\ndata: one\n\ndata: unnamed\n\nevent: other\ndata: two\n\nevent: refresh\ndata: three\n\n"
	tests := []struct {
		name, channel string
		want          []string
	}{
		{"empty_keeps_everything", "", []string{"one", "unnamed", "two", "three"}},
		{"a_name", "refresh", []string{"one", "three"}},
		{"unnamed_events_are_message", "message", []string{"unnamed"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := stream(t, body, nil)

			res, err := sse.New().Run(t.Context(), transport.Operation{
				Server: url, Channel: tt.channel, Receive: &transport.Receive{Count: len(tt.want), Timeout: 5 * time.Second},
			})

			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, m := range res.Received {
				got = append(got, string(m.Body))
			}
			if strings.Join(got, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRun_should_not_add_the_channel_to_the_url(t *testing.T) {
	seen := make(chan *http.Request, 1)
	url := stream(t, "event: refresh\ndata: x\n\n", seen)

	_, err := sse.New().Run(t.Context(), transport.Operation{Server: url + "/stream", Channel: "refresh", Receive: &transport.Receive{Timeout: 5 * time.Second}})

	if err != nil {
		t.Fatal(err)
	}
	if r := <-seen; r.URL.Path != "/stream" {
		t.Fatalf("requested %q, want /stream: the channel is an event name", r.URL.Path)
	}
}
