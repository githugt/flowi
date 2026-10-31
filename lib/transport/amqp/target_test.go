// SPDX-License-Identifier: Apache-2.0

package amqp

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/transport"
)

func TestTargetOf_should_map_the_binding_to_an_exchange_key_and_queue(t *testing.T) {
	tests := []struct {
		name                           string
		channel                        string
		binding                        map[string]string
		exchange, key, queue, wantUser string
	}{
		{name: "channel_is_the_routing_key", channel: "orders.placed", binding: map[string]string{"exchange": "events"}, exchange: "events", key: "orders.placed"},
		{name: "routing_key_wins", channel: "orders.placed", binding: map[string]string{"exchange": "events", "routing_key": "orders.*"}, exchange: "events", key: "orders.*"},
		{name: "default_exchange_routes_to_the_queue_named_by_the_channel", channel: "jobs", key: "jobs"},
		{name: "queue_given", channel: "orders.placed", binding: map[string]string{"queue": "q1", "exchange": "events"}, exchange: "events", key: "orders.placed", queue: "q1"},
		{name: "credentials", channel: "k", binding: map[string]string{"username": "u", "password": "p"}, key: "k", wantUser: "u"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := targetOf(transport.Operation{Server: "amqp://broker:5672/vh", Channel: tt.channel, Binding: tt.binding})

			if err != nil {
				t.Fatal(err)
			}
			if got.exchange != tt.exchange || got.key != tt.key || got.queue != tt.queue {
				t.Fatalf("got %+v", got)
			}
			if tt.wantUser != "" && got.url.User.Username() != tt.wantUser {
				t.Fatalf("user %v", got.url.User)
			}
			if got.url.Path != "/vh" {
				t.Fatalf("vhost path %q", got.url.Path)
			}
		})
	}
}

func TestTargetOf_should_reject_a_server_that_is_not_amqp(t *testing.T) {
	_, err := targetOf(transport.Operation{Server: "http://h", Channel: "k"})

	if err == nil || !strings.Contains(err.Error(), "scheme") {
		t.Fatalf("err %v", err)
	}
}

func TestRedact_should_hide_the_password_in_an_error(t *testing.T) {
	tg, _ := targetOf(transport.Operation{Server: "amqp://u:hunter2@broker", Channel: "k"})

	got := tg.redact("dial tcp: amqp://u:hunter2@broker:5672 refused")

	if strings.Contains(got, "hunter2") {
		t.Fatalf("password leaked: %s", got)
	}
}

func TestReceiveNeedsAQueueOrExchange(t *testing.T) {
	tg, _ := targetOf(transport.Operation{Server: "amqp://broker", Channel: "k"})

	if err := tg.checkReceive(); err == nil {
		t.Fatal("consuming from the default exchange with no queue named has nothing to bind: want an error")
	}
}
