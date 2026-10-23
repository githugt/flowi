// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"testing"
)

func TestNormalizeServer_should_write_out_the_scheme_nats_and_kafka_may_leave_off(t *testing.T) {
	tests := []struct{ protocol, server, want string }{
		{ProtocolNATS, "localhost:4222", "nats://localhost:4222"},
		{ProtocolKafka, "a:9092,b:9092", "kafka://a:9092,b:9092"},
		{ProtocolNATS, "nats://h:4222", "nats://h:4222"},
		{ProtocolNATS, "tls://h:4222", "tls://h:4222"},
		{ProtocolNATS, "{{env.nats}}", "{{env.nats}}"},
		{ProtocolNATS, "", ""},
		{ProtocolMQTT, "localhost:1883", "localhost:1883"}, // MQTT has several schemes: nothing to guess
		{ProtocolServiceBus, "myns.servicebus.windows.net", "sb://myns.servicebus.windows.net"},
		{ProtocolServiceBus, "sb://myns.servicebus.windows.net", "sb://myns.servicebus.windows.net"},
		{ProtocolSQS, "", ""},
	}
	for _, tt := range tests {
		if got := NormalizeServer(tt.protocol, tt.server); got != tt.want {
			t.Errorf("NormalizeServer(%s, %q) = %q, want %q", tt.protocol, tt.server, got, tt.want)
		}
	}
}

func TestTargetURL_should_join_the_channel_except_for_sse_where_it_is_an_event_name(t *testing.T) {
	tests := []struct{ protocol, server, channel, want string }{
		{ProtocolWS, "ws://h:8080", "chat", "ws://h:8080/chat"},
		{ProtocolNATS, "nats://h:4222", "orders.new", "nats://h:4222/orders.new"},
		{ProtocolSSE, "http://h:4000/stream", "custom-trigger", "http://h:4000/stream"},
		{ProtocolSSE, "http://h:4000/stream", "", "http://h:4000/stream"},
		{ProtocolMQTT, "mqtt://h", "", "mqtt://h"},
		{ProtocolSQS, "", "jobs", "jobs"}, // no server: AWS itself, so just the queue
		{ProtocolSQS, "http://localhost:4566", "jobs", "http://localhost:4566/jobs"},
		{ProtocolPubSub, "pubsub://demo", "feed", "pubsub://demo/feed"},
	}
	for _, tt := range tests {
		if got := TargetURL(tt.protocol, tt.server, tt.channel); got != tt.want {
			t.Errorf("TargetURL(%s, %q, %q) = %q, want %q", tt.protocol, tt.server, tt.channel, got, tt.want)
		}
	}
}

func TestAsyncProtocols_should_list_the_managed_messaging_protocols_after_the_open_ones(t *testing.T) {
	want := []string{"ws", "sse", "mqtt", "kafka", "amqp", "nats", "redis", "sqs", "sns", "servicebus", "pubsub"}

	got := AsyncProtocols()

	if !slices.Equal(got, want) {
		t.Fatalf("AsyncProtocols() = %v, want %v", got, want)
	}
}

func TestAsyncSchemesAndBindings_should_describe_each_new_protocol(t *testing.T) {
	tests := []struct {
		protocol string
		schemes  []string
		binding  string // one name it must understand
	}{
		{ProtocolRedis, []string{"redis", "rediss"}, "mode"},
		{ProtocolSQS, []string{"http", "https"}, "visibility_timeout"},
		{ProtocolSNS, []string{"http", "https"}, "subject"},
		{ProtocolServiceBus, []string{"sb"}, "connection_string"},
		{ProtocolPubSub, []string{"pubsub"}, "ordering_key"},
	}
	for _, tt := range tests {
		if got := AsyncSchemes(tt.protocol); !slices.Equal(got, tt.schemes) {
			t.Errorf("AsyncSchemes(%s) = %v, want %v", tt.protocol, got, tt.schemes)
		}
		if !slices.Contains(AsyncBindings(tt.protocol), tt.binding) {
			t.Errorf("AsyncBindings(%s) lacks %q", tt.protocol, tt.binding)
		}
	}
}

func TestAsyncCredentials_should_name_the_binding_entries_that_are_secrets(t *testing.T) {
	tests := []struct {
		protocol string
		want     []string
	}{
		{ProtocolRedis, []string{"username", "password"}},
		{ProtocolSQS, []string{"access_key_id", "secret_access_key", "session_token"}},
		{ProtocolSNS, []string{"access_key_id", "secret_access_key", "session_token"}},
		{ProtocolServiceBus, []string{"connection_string"}},
		{ProtocolPubSub, []string{"credentials_json"}},
		{ProtocolMQTT, []string{"username", "password", "token"}},
		{ProtocolWS, nil},
	}
	for _, tt := range tests {
		if got := AsyncCredentials(tt.protocol); !slices.Equal(got, tt.want) {
			t.Errorf("AsyncCredentials(%s) = %v, want %v", tt.protocol, got, tt.want)
		}
	}
}

func TestCloudProtocol_should_be_true_only_for_the_providers_with_a_login_mode(t *testing.T) {
	for _, p := range AsyncProtocols() {
		want := p == ProtocolSQS || p == ProtocolSNS || p == ProtocolServiceBus || p == ProtocolPubSub
		if got := CloudProtocol(p); got != want {
			t.Errorf("CloudProtocol(%s) = %v, want %v", p, got, want)
		}
	}
}

func TestRedactedTarget_should_hide_a_password_written_in_the_server(t *testing.T) {
	tests := []struct{ in, want string }{
		{"redis://app:s3cret@cache:6379/events", "redis://app:***@cache:6379/events"},
		{"redis://:s3cret@cache:6379", "redis://:***@cache:6379"},
		{"amqp://guest@h:5672/q", "amqp://guest@h:5672/q"}, // a user name alone is not secret
		{"nats://h:4222/orders", "nats://h:4222/orders"},
		{"kafka://u:p@a:9092,b:9092/t", "kafka://u:***@a:9092,b:9092/t"},
		{"orders", "orders"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := RedactedTarget(tt.in); got != tt.want {
			t.Errorf("RedactedTarget(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestProtocolVerb_should_be_a_short_upper_case_name_that_maps_back(t *testing.T) {
	want := map[string]string{ProtocolMQTT: "MQTT", ProtocolServiceBus: "BUS", ProtocolPubSub: "PUBSUB", ProtocolSQS: "SQS"}
	for protocol, verb := range want {
		if got := ProtocolVerb(protocol); got != verb {
			t.Errorf("ProtocolVerb(%s) = %q, want %q", protocol, got, verb)
		}
	}
	for _, p := range AsyncProtocols() {
		if back := ProtocolOfVerb(ProtocolVerb(p)); back != p {
			t.Errorf("ProtocolOfVerb(ProtocolVerb(%s)) = %q", p, back)
		}
	}
	if ProtocolOfVerb("GET") != "" {
		t.Error("an HTTP method is not a protocol verb")
	}
}
