// SPDX-License-Identifier: Apache-2.0

package mqtt_test

import (
	"testing"
	"time"

	mochi "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/mqtt"
)

func TestOpen_should_hold_messages_published_before_the_step_that_reads_them(t *testing.T) {
	// Arrange
	srv, url := broker(t, nil)
	s, err := mqtt.New().Open(t.Context(), transport.Operation{Server: url, Channel: "alerts"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	_ = srv.Publish("alerts", []byte("one"), false, 0)
	_ = srv.Publish("alerts", []byte("two"), false, 0)

	// Act
	got, err := s.Receive(t.Context(), transport.Receive{Count: 2, Timeout: 2 * time.Second})

	// Assert
	if err != nil || len(got) != 2 || string(got[0].Body) != "one" || string(got[1].Body) != "two" {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestOpen_should_publish_over_the_open_connection(t *testing.T) {
	// Arrange
	srv, url := broker(t, nil)
	seen := make(chan string, 1)
	if err := srv.Subscribe("alerts", 1, func(_ *mochi.Client, _ packets.Subscription, pk packets.Packet) { seen <- string(pk.Payload) }); err != nil {
		t.Fatal(err)
	}
	s, err := mqtt.New().Open(t.Context(), transport.Operation{Server: url, Channel: "alerts"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Act
	sent, err := s.Send(t.Context(), transport.Message{Body: []byte("hello")})

	// Assert
	if err != nil || sent.Direction != "sent" {
		t.Fatalf("sent %+v, err %v", sent, err)
	}
	select {
	case got := <-seen:
		if got != "hello" {
			t.Fatalf("broker saw %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the broker never saw the message")
	}
}

func TestOpen_Given_MQTT_3_1_1_Then_says_only_version_5_can_be_kept_open(t *testing.T) {
	_, url := broker(t, nil)

	_, err := mqtt.New().Open(t.Context(), transport.Operation{Server: url, Channel: "x", Binding: map[string]string{"version": "3.1.1"}})

	if err == nil {
		t.Fatal("a 3.1.1 session is not supported and must say so")
	}
}
