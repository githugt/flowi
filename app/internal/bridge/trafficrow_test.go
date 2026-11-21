// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/serve/replay"
)

func TestTrafficRowOf_Given_an_answer_from_a_saved_example_When_converted_Then_it_says_which_one(t *testing.T) {
	// Before: the row dropped the version, example, source and simulated delay the log holds, so the screen
	// could say that a call matched a request but not which saved answer it was given.
	e := replay.Entry{
		Seq: 3, At: time.Date(2026, 11, 3, 10, 0, 0, 0, time.UTC), Method: "GET", Path: "/pets", Status: 200, Match: replay.MatchOK,
		Request: "pets/list", Version: "v2", Example: "two pets", Source: "pinned", Latency: 250 * time.Millisecond, Duration: 260 * time.Millisecond,
	}

	row := trafficRowOf(e)

	if row.Version != "v2" || row.Example != "two pets" || row.Source != "pinned" || row.Delay != "250ms" {
		t.Fatalf("row = %+v, want the version, example, source and a 250ms delay", row)
	}
}

func TestTrafficRowOf_Given_no_simulated_delay_When_converted_Then_the_delay_is_empty(t *testing.T) {
	e := replay.Entry{Method: "GET", Path: "/x", Status: 404, Match: replay.MatchNone}

	row := trafficRowOf(e)

	if row.Delay != "" {
		t.Fatalf("Delay = %q, want empty", row.Delay)
	}
}

func TestCallViewOf_Given_a_captured_call_When_converted_Then_headers_are_listed_by_name_with_the_bodies(t *testing.T) {
	e := replay.Entry{Seq: 7, Exchange: &replay.Exchange{
		Sent: replay.Message{Header: http.Header{"X-B": {"2"}, "Accept": {"*/*"}}, Body: `{"a":1}`},
		Got:  replay.Message{Header: http.Header{"Content-Type": {"application/json", "x"}}, Body: "ok", Truncated: true},
	}}

	view := callViewOf(e)

	if !view.Captured || view.Seq != 7 {
		t.Fatalf("view = %+v", view)
	}
	if len(view.Sent.Headers) != 2 || view.Sent.Headers[0].Name != "Accept" || view.Sent.Headers[1].Name != "X-B" || view.Sent.Body != `{"a":1}` {
		t.Errorf("sent = %+v, want Accept then X-B", view.Sent)
	}
	if h := view.Got.Headers; len(h) != 2 || h[0].Value != "application/json" || h[1].Name != "Content-Type" || !view.Got.Truncated {
		t.Errorf("got = %+v, want a row for each value", view.Got)
	}
}

func TestCallViewOf_Given_a_call_without_a_capture_When_converted_Then_it_says_so(t *testing.T) {
	view := callViewOf(replay.Entry{Seq: 2})

	if view.Captured {
		t.Fatalf("Captured = true for an entry with no Exchange")
	}
}

func TestMessagesOf_Given_messages_of_each_body_kind_When_converted_Then_each_has_a_kind(t *testing.T) {
	// Before: every message looked alike, and a binary one was an empty row with nothing to say what it was.
	in := []engine.RecordMessage{
		{Direction: "received", Body: engine.Body{JSON: map[string]any{"a": 1}, ContentType: "application/json", Size: 7}},
		{Direction: "received", Body: engine.Body{Text: "hello", Size: 5}},
		{Direction: "received", Body: engine.Body{Base64: "AAEC", Size: 3}},
		{Direction: "sent", Body: engine.Body{}},
	}

	got := messagesOf(in)

	kinds := []string{got[0].Kind, got[1].Kind, got[2].Kind, got[3].Kind}
	want := []string{"json", "text", "binary", ""}
	for i := range want {
		if kinds[i] != want[i] {
			t.Errorf("message %d kind = %q, want %q", i, kinds[i], want[i])
		}
	}
	if got[2].Body != "(binary, 3 bytes)" {
		t.Errorf("binary body = %q", got[2].Body)
	}
}

func TestMessagesOf_Given_a_message_with_a_protocol_type_When_converted_Then_the_row_carries_it(t *testing.T) {
	// Before: an SSE event name or a WebSocket frame kind never reached the row, so a stream of different events
	// all looked alike.
	in := []engine.RecordMessage{{Direction: "received", Type: "order.paid", Body: engine.Body{Text: "x", Size: 1}}}

	got := messagesOf(in)

	if got[0].Type != "order.paid" {
		t.Fatalf("type = %q", got[0].Type)
	}
}

func TestMessagesOf_Given_broker_metadata_When_converted_Then_the_row_carries_it(t *testing.T) {
	// Before: the partition, offset, sequence and headers of a Kafka or JetStream message were dropped here.
	in := []engine.RecordMessage{{Direction: "received", Partition: 2, Offset: 10345, Sequence: 7,
		Headers: []engine.Header{{Name: "trace-id", Value: "abc"}}, Body: engine.Body{Text: "x", Size: 1}}}

	got := messagesOf(in)

	if got[0].Partition != 2 || got[0].Offset != 10345 || got[0].Sequence != 7 || len(got[0].Headers) != 1 || got[0].Headers[0].Key != "trace-id" {
		t.Fatalf("row %+v", got[0])
	}
}

func TestMessagesOf_Given_a_binary_message_When_converted_Then_the_row_carries_its_size_and_a_hex_dump(t *testing.T) {
	// Before: a binary message was only the note "(binary, 3 bytes)", so its bytes could not be looked at and its row
	// measured the note, not the message.
	in := []engine.RecordMessage{
		{Direction: "received", Body: engine.Body{Base64: "AAEC", Size: 3}},
		{Direction: "received", Body: engine.Body{Text: "hello", Size: 5}},
	}

	got := messagesOf(in)

	if got[0].Size != 3 || !strings.Contains(got[0].Hex, "00 01 02") {
		t.Errorf("binary row size = %d, hex = %q", got[0].Size, got[0].Hex)
	}
	if got[1].Hex != "" || got[1].Size != 5 {
		t.Errorf("text row size = %d, hex = %q", got[1].Size, got[1].Hex)
	}
}

func TestRecordedOf_Given_forwarded_and_saved_calls_When_counted_Then_each_new_request_counts_once(t *testing.T) {
	// Before: record mode ended with no result: it wrote request files and said nothing of how many or where.
	entries := []replay.Entry{
		{Match: replay.MatchRecorded, Request: "recorded/get-orders"},
		{Match: "request", Request: "auth/login"},
		{Match: replay.MatchRecorded, Request: "recorded/get-orders"},
		{Match: replay.MatchRecorded, Request: "recorded/post-pay"},
		{Match: replay.MatchRecorded, Request: ""},
	}
	targets := []Choice{{ID: "", Label: "Scratch (drafts)"}, {ID: "c1", Label: "Payments", Path: "Payments"}}

	got := recordedOf(entries, targets, "c1", " ")

	if got.Count != 2 || got.Requests[0] != "recorded/get-orders" || got.Requests[1] != "recorded/post-pay" {
		t.Fatalf("recorded %+v", got)
	}
	if got.Target != "Payments/recorded" {
		t.Fatalf("target %q", got.Target)
	}
}

func TestRecordedOf_Given_the_scratch_draft_and_a_folder_When_counted_Then_the_target_names_them(t *testing.T) {
	got := recordedOf(nil, []Choice{{ID: "", Label: "Scratch (drafts)"}}, "", "captured")

	if got.Count != 0 || got.Requests == nil || got.Target != "Scratch/captured" {
		t.Fatalf("recorded %+v", got)
	}
}
