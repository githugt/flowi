// SPDX-License-Identifier: AGPL-3.0-or-later

package card_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/card"
)

func request(open, over string, closeIt bool) model.Node {
	return model.Node{ID: "r", Type: "request", Request: &model.RequestCall{Open: open, Over: over, Close: closeIt}}
}

// rowsOf renders the rows as "key: summary" lines for comparing.
func rowsOf(rows []card.Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key + ": " + r.Summary
	}
	return out
}

func moreOf(rows []card.Row) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.Key
	}
	return out
}

func TestSummarize_Given_each_request_kind_Then_badge_subline_and_rows_follow_the_design_note(t *testing.T) {
	tests := []struct {
		name    string
		node    model.Node
		facts   card.Facts
		badge   string
		subline string
		rows    []string
		more    []string
	}{
		{
			name:    "http_get_with_auth",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "http", Method: "get", Target: "/pokemon/ditto", Auth: "Bearer", Query: 2, Headers: 3, Body: "JSON", PreHooks: 1, PostHooks: 2, Retry: 2, Timeout: "10s"},
			badge:   "GET",
			subline: "/pokemon/ditto",
			rows:    []string{"inputs: Bearer · 2 query · 3 headers · JSON", "hooks: 1 before · 2 after", "settings: retry 2 · timeout 10s"},
		},
		{
			name:    "http_get_bare_hides_empty_parts",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "http", Method: "GET", Target: "/health"},
			badge:   "GET",
			subline: "/health",
			more:    []string{"inputs", "hooks", "settings"},
		},
		{
			name:    "graphql",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "graphql", Method: "query", Target: "getUser", Variables: 2, Headers: 1},
			badge:   "QUERY",
			subline: "getUser",
			rows:    []string{"inputs: 2 variables · 1 header"},
			more:    []string{"hooks", "settings"},
		},
		{
			name:    "grpc_unary",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "grpc", Method: "unary", Target: "Service.Method", Body: "JSON", Props: 2, Auth: "Bearer", Timeout: "5s"},
			badge:   "UNARY",
			subline: "Service.Method",
			rows:    []string{"send: Bearer · JSON · 2 metadata", "settings: timeout 5s"},
			more:    []string{"hooks"},
		},
		{
			name:    "websocket_with_connection",
			node:    request("chat", "", false),
			facts:   card.Facts{Kind: "ws", Sends: true, Receives: true, Target: "ws://host/path", Headers: 2, Body: "1 message", ReceiveCount: 3, ReceiveWait: "10s"},
			badge:   "SEND",
			subline: "ws://host/path",
			rows:    []string{"inputs: 2 headers", "send: 1 message", "receive: 3 messages · 10s", "connection: opens chat"},
			more:    []string{"hooks"},
		},
		{
			name:    "websocket_uses_then_closes",
			node:    request("", "chat", true),
			facts:   card.Facts{Kind: "ws", Sends: true, Target: "ws://host/path", Body: "1 message"},
			badge:   "SEND",
			subline: "ws://host/path",
			rows:    []string{"send: 1 message", "connection: uses chat, then closes it"},
			more:    []string{"inputs", "receive", "hooks"},
		},
		{
			name:    "sse_listens",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "sse", Receives: true, Target: "https://host/events", Auth: "Bearer", ReceiveCount: 5, ReceiveWait: "30s"},
			badge:   "LISTEN",
			subline: "https://host/events",
			rows:    []string{"inputs: Bearer", "receive: 5 messages · 30s"},
			more:    []string{"hooks", "settings"},
		},
		{
			name:    "mqtt_publish",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "mqtt", Sends: true, Target: "orders/placed", Body: "JSON", Props: 2, SendExtras: []string{"QoS 1", "retain"}},
			badge:   "PUBLISH",
			subline: "orders/placed",
			rows:    []string{"send: JSON · 2 properties · QoS 1 · retain"},
			more:    []string{"receive", "connection", "hooks", "settings"},
		},
		{
			name:    "mqtt_subscribe",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "mqtt", Receives: true, Target: "orders/#", ReceiveCount: 2, ReceiveWait: "10s"},
			badge:   "SUBSCRIBE",
			subline: "orders/#",
			rows:    []string{"receive: 2 messages · 10s"},
			more:    []string{"send", "connection", "hooks", "settings"},
		},
		{
			name:    "nats_request",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "nats", Method: "request", Target: "svc.echo", Sends: true, Receives: true, Body: "text", ReceiveCount: 1, ReceiveWait: "5s"},
			badge:   "REQUEST",
			subline: "svc.echo",
			rows:    []string{"send: text", "receive: 1 message · 5s"},
			more:    []string{"connection", "hooks"},
		},
		{
			name:    "kafka_produce",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "kafka", Sends: true, Target: "events", Body: "JSON", Props: 2, SendExtras: []string{"key"}},
			badge:   "PRODUCE",
			subline: "events",
			rows:    []string{"send: JSON · 2 headers · key"},
			more:    []string{"receive", "hooks", "settings"},
		},
		{
			name:    "kafka_consume",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "kafka", Receives: true, Target: "events", ReceiveCount: 5, ReceiveExtras: []string{"group g1", "from latest"}},
			badge:   "CONSUME",
			subline: "events",
			rows:    []string{"receive: 5 messages · group g1 · from latest"},
			more:    []string{"send", "hooks", "settings"},
		},
		{
			name:    "amqp_publish",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "amqp", Sends: true, Target: "orders/placed", Body: "JSON"},
			badge:   "PUBLISH",
			subline: "orders/placed",
			rows:    []string{"send: JSON"},
			more:    []string{"receive", "hooks", "settings"},
		},
		{
			name:    "amqp10_consume",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "amqp10", Receives: true, Target: "queue-1", ReceiveExtras: []string{"auto-ack"}},
			badge:   "CONSUME",
			subline: "queue-1",
			rows:    []string{"receive: auto-ack"},
			more:    []string{"send", "hooks", "settings"},
		},
		{
			name:    "redis_add",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "redis", Method: "add", Sends: true, Target: "stream:orders", Body: "JSON"},
			badge:   "ADD",
			subline: "stream:orders",
			rows:    []string{"send: JSON"},
			more:    []string{"receive", "hooks"},
		},
		{
			name:    "sns_publish_short_topic",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "sns", Sends: true, Target: "orders", Body: "JSON", Props: 2, Region: "eu-west-1"},
			badge:   "PUBLISH",
			subline: "orders",
			rows:    []string{"send: JSON · 2 attributes", "settings: region eu-west-1"},
			more:    []string{"hooks"},
		},
		{
			name:    "sqs_receive",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "sqs", Receives: true, Target: "jobs", ReceiveCount: 10, ReceiveWait: "5s", ReceiveExtras: []string{"delete after read"}},
			badge:   "RECEIVE",
			subline: "jobs",
			rows:    []string{"receive: 10 messages · 5s · delete after read"},
			more:    []string{"send", "hooks", "settings"},
		},
		{
			name:    "pubsub_pull",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "pubsub", Receives: true, Target: "sub-orders", ReceiveCount: 1},
			badge:   "PULL",
			subline: "sub-orders",
			rows:    []string{"receive: 1 message"},
			more:    []string{"send", "hooks"},
		},
		{
			name:    "servicebus_send",
			node:    request("", "", false),
			facts:   card.Facts{Kind: "servicebus", Sends: true, Target: "orders", Body: "JSON", Props: 2, SendExtras: []string{"session s1"}},
			badge:   "SEND",
			subline: "orders",
			rows:    []string{"send: JSON · 2 properties · session s1"},
			more:    []string{"receive", "hooks", "settings"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := card.Summarize(tt.node, tt.facts)

			if got.Badge != tt.badge || got.Subline != tt.subline {
				t.Fatalf("badge %q subline %q, want %q %q", got.Badge, got.Subline, tt.badge, tt.subline)
			}
			if r := rowsOf(got.Rows); strings.Join(r, "|") != strings.Join(tt.rows, "|") {
				t.Fatalf("rows %q, want %q", r, tt.rows)
			}
			if m := moreOf(got.More); strings.Join(m, "|") != strings.Join(tt.more, "|") {
				t.Fatalf("more %v, want %v", m, tt.more)
			}
			wantPorts := []string{"done", "fail"}
			if ports := portIDs(got.Ports); strings.Join(ports, "|") != strings.Join(wantPorts, "|") {
				t.Fatalf("ports %v, want %v", ports, wantPorts)
			}
		})
	}
}

func portIDs(ports []card.Port) []string {
	out := make([]string, len(ports))
	for i, p := range ports {
		out[i] = p.ID
	}
	return out
}

func TestSummarize_Given_a_default_part_Then_no_row_is_made_for_it(t *testing.T) {
	facts := card.Facts{Kind: "http", Method: "GET", Target: "/x", Retry: 0, Timeout: ""}

	got := card.Summarize(request("", "", false), facts)

	for _, r := range got.Rows {
		t.Fatalf("unexpected row %+v", r)
	}
}

func TestSummarize_Given_a_very_long_target_Then_the_summary_is_cut_with_an_ellipsis_at_40_runes(t *testing.T) {
	long := "/" + strings.Repeat("é", 80)
	facts := card.Facts{Kind: "http", Method: "GET", Target: long}

	got := card.Summarize(request("", "", false), facts)

	runes := []rune(got.Subline)
	if len(runes) != 40 || runes[39] != '…' {
		t.Fatalf("subline %q has %d runes, want 40 ending in an ellipsis", got.Subline, len(runes))
	}
}

func TestSummarize_Given_more_than_four_filled_parts_Then_four_rows_show_and_the_rest_wait_under_more(t *testing.T) {
	facts := card.Facts{Kind: "ws", Sends: true, Receives: true, Target: "ws://h", Headers: 1, Body: "1 message", ReceiveCount: 1, PreHooks: 1}

	got := card.Summarize(request("c", "", false), facts)

	if len(got.Rows) != 4 || len(got.More) != 1 || got.More[0].Key != "hooks" {
		t.Fatalf("rows %q more %v", rowsOf(got.Rows), moreOf(got.More))
	}
}
