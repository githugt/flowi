// SPDX-License-Identifier: Apache-2.0

package servicebus

import (
	"strings"
	"testing"
	"time"
)

func TestSendTarget_should_send_to_the_queue_or_to_the_topic_of_a_topic_path(t *testing.T) {
	tests := []struct{ channel, want string }{
		{"orders", "orders"},
		{"events/billing", "events"}, // a topic/subscription path: the sender goes to the topic
		{"events", "events"},
	}
	for _, tt := range tests {
		got, err := sendTarget(tt.channel)
		if err != nil || got != tt.want {
			t.Errorf("sendTarget(%q) = %q, %v; want %q", tt.channel, got, err, tt.want)
		}
	}
}

func TestReceiveTarget_should_read_a_queue_or_a_topic_subscription(t *testing.T) {
	tests := []struct {
		channel string
		want    entity
	}{
		{"orders", entity{queue: "orders"}},
		{"events/billing", entity{topic: "events", subscription: "billing"}},
	}
	for _, tt := range tests {
		got, err := receiveTarget(tt.channel)
		if err != nil || got != tt.want {
			t.Errorf("receiveTarget(%q) = %+v, %v; want %+v", tt.channel, got, err, tt.want)
		}
	}
}

func TestTargets_should_refuse_an_empty_or_malformed_channel(t *testing.T) {
	for _, channel := range []string{"", "/", "events/", "/billing", "a/b/c"} {
		if _, err := sendTarget(channel); err == nil {
			t.Errorf("sendTarget(%q) accepted", channel)
		}
		if _, err := receiveTarget(channel); err == nil {
			t.Errorf("receiveTarget(%q) accepted", channel)
		}
	}
}

func TestNamespaceOf_should_accept_the_forms_a_server_is_written_in(t *testing.T) {
	tests := []struct{ server, want string }{
		{"myns.servicebus.windows.net", "myns.servicebus.windows.net"},
		{"sb://myns.servicebus.windows.net", "myns.servicebus.windows.net"},
		{"sb://myns.servicebus.windows.net/", "myns.servicebus.windows.net"},
		{"myns", "myns.servicebus.windows.net"},
		{"sb://MyNs", "MyNs.servicebus.windows.net"},
	}
	for _, tt := range tests {
		got, err := namespaceOf(tt.server)
		if err != nil || got != tt.want {
			t.Errorf("namespaceOf(%q) = %q, %v; want %q", tt.server, got, err, tt.want)
		}
	}
	if _, err := namespaceOf(""); err == nil {
		t.Error("namespaceOf(\"\") accepted")
	}
	if _, err := namespaceOf("https://myns.servicebus.windows.net"); err == nil {
		t.Error("an https server was accepted")
	}
}

func TestExplicitConnection_should_need_a_connection_string_and_never_echo_it(t *testing.T) {
	_, err := explicitConnection(map[string]string{"auth": "explicit"})
	if err == nil || !strings.Contains(err.Error(), "connection_string") {
		t.Fatalf("err %v, want one naming connection_string", err)
	}

	_, err = explicitConnection(map[string]string{"auth": "explicit", "connection_string": "SharedAccessKey=top-s3cret-value"})
	if err == nil || !strings.Contains(err.Error(), "connection_string") || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("err %v, want a malformed-string error that does not echo the value", err)
	}

	cs := "Endpoint=sb://myns.servicebus.windows.net/;SharedAccessKeyName=k;SharedAccessKey=v"
	if got, err := explicitConnection(map[string]string{"auth": "explicit", "connection_string": cs}); err != nil || got != cs {
		t.Fatalf("got %q, %v; want the string back", got, err)
	}
}

func TestSendProperties_should_map_the_binding_onto_the_message(t *testing.T) {
	binding := map[string]string{"session_id": "s1", "subject": "New order", "message_id": "m-1", "ttl": "90s"}

	p, err := sendPropertiesOf(binding)

	if err != nil {
		t.Fatal(err)
	}
	if p.sessionID != "s1" || p.subject != "New order" || p.messageID != "m-1" || p.ttl != 90*time.Second {
		t.Fatalf("properties %+v", p)
	}
	if _, err := sendPropertiesOf(map[string]string{"ttl": "soon"}); err == nil || !strings.Contains(err.Error(), "ttl") {
		t.Fatalf("err %v, want one naming ttl", err)
	}
}

func TestSendPropertiesOf_should_generate_a_message_id_when_none_is_given(t *testing.T) {
	a, err := sendPropertiesOf(nil)
	b, _ := sendPropertiesOf(nil)

	if err != nil || a.messageID == "" || a.messageID == b.messageID {
		t.Fatalf("ids %q and %q; want two different generated ids", a.messageID, b.messageID)
	}
}

func TestReadModeOf_should_default_to_lock_and_accept_peek(t *testing.T) {
	cases := map[string]string{"": "lock", "lock": "lock", "peek": "peek"}
	for in, want := range cases {
		got, err := readModeOf(map[string]string{"read": in})

		if err != nil || got != want {
			t.Errorf("read %q: got %q, %v, want %q", in, got, err, want)
		}
	}
}

func TestReadModeOf_Given_another_value_Then_says_what_is_allowed(t *testing.T) {
	_, err := readModeOf(map[string]string{"read": "consume"})

	if err == nil || !strings.Contains(err.Error(), "peek or lock") {
		t.Fatalf("got %v", err)
	}
}
