// SPDX-License-Identifier: Apache-2.0

package mqtt_test

import (
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/auth"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/mqtt"
)

func v311(extra map[string]string) map[string]string {
	b := map[string]string{"version": "3.1.1"}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

func TestRun_Given_version_3_1_1_should_publish_with_qos_and_retain(t *testing.T) {
	// Arrange
	srv, server := broker(t, nil)
	got := make(chan packets.Packet, 1)
	if err := srv.Subscribe("orders/#", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) { got <- pk }); err != nil {
		t.Fatal(err)
	}

	// Act
	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders/placed", Send: &transport.Message{Body: []byte(`{"id":1}`)},
		Binding: v311(map[string]string{"qos": "1", "retain": "true"}),
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	pk := <-got
	if string(pk.Payload) != `{"id":1}` || pk.TopicName != "orders/placed" || !pk.FixedHeader.Retain {
		t.Fatalf("broker got %q on %q retain %v", pk.Payload, pk.TopicName, pk.FixedHeader.Retain)
	}
	if res.Sent == nil || res.Sent.Direction != "sent" {
		t.Fatalf("sent %+v", res.Sent)
	}
}

func TestRun_Given_version_3_1_1_should_receive_retained_messages_with_the_topic(t *testing.T) {
	// Arrange
	srv, server := broker(t, nil)
	if err := srv.Publish("orders/placed", []byte("one"), true, 1); err != nil {
		t.Fatal(err)
	}

	// Act
	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "orders/+", Receive: &transport.Receive{Timeout: 5 * time.Second}, Binding: v311(nil),
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || string(res.Received[0].Body) != "one" || res.Received[0].Headers[0].Value != "orders/placed" {
		t.Fatalf("received %+v", res.Received)
	}
}

func TestRun_Given_version_3_1_1_should_get_the_reply_but_not_its_own_message(t *testing.T) {
	// Arrange
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

	// Act
	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "rpc/ask", Send: &transport.Message{Body: []byte("question")},
		Receive: &transport.Receive{Timeout: 5 * time.Second}, Binding: v311(nil),
	})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Received) != 1 || string(res.Received[0].Body) != "answer" {
		t.Fatalf("received %+v (3.1.1 has no no-local, so the echo must be filtered out)", res.Received)
	}
}

func TestRun_Given_version_3_1_1_should_authenticate_with_username_and_password(t *testing.T) {
	// Arrange
	_, server := broker(t, auth.AuthRules{{Username: "u", Password: "p", Allow: true}})
	op := transport.Operation{Server: server, Channel: "t", Send: &transport.Message{Body: []byte("x")}, Binding: v311(nil)}

	// Act
	_, bad := mqtt.New().Run(t.Context(), op)
	op.Binding = v311(map[string]string{"username": "u", "password": "p"})
	_, good := mqtt.New().Run(t.Context(), op)

	// Assert
	if bad == nil {
		t.Fatal("a connection without credentials must be refused")
	}
	if good != nil {
		t.Fatalf("with credentials: %v", good)
	}
}

func TestRun_Given_version_3_1_1_should_time_out_with_the_partial_messages(t *testing.T) {
	// Arrange
	srv, server := broker(t, nil)
	_ = srv.Publish("t", []byte("only"), true, 0)

	// Act
	res, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "t", Receive: &transport.Receive{Count: 2, Timeout: 300 * time.Millisecond}, Binding: v311(nil),
	})

	// Assert
	if !errors.Is(err, transport.ErrTimeout) || len(res.Received) != 1 {
		t.Fatalf("err %v received %d", err, len(res.Received))
	}
}

func TestRun_Given_version_3_1_1_should_refuse_message_headers_instead_of_dropping_them(t *testing.T) {
	// Arrange
	_, server := broker(t, nil)

	// Act
	_, err := mqtt.New().Run(t.Context(), transport.Operation{
		Server: server, Channel: "t", Binding: v311(nil),
		Send: &transport.Message{Body: []byte("x"), Headers: []transport.Header{{Name: "type", Value: "order"}}},
	})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "no headers") {
		t.Fatalf("err %v", err)
	}
}

func TestRun_should_refuse_an_unknown_version(t *testing.T) {
	// Act
	_, err := mqtt.New().Run(t.Context(), transport.Operation{Server: "mqtt://127.0.0.1:1", Channel: "t", Binding: map[string]string{"version": "4"}})

	// Assert
	if err == nil || !strings.Contains(err.Error(), "version must be") {
		t.Fatalf("err %v", err)
	}
}
