// SPDX-License-Identifier: Apache-2.0

package nats_test

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/nats"
)

func TestOpen_should_hold_messages_published_before_the_step_that_reads_them(t *testing.T) {
	// Arrange
	url := startServer(t, nil)
	s, err := nats.New().Open(t.Context(), transport.Operation{Server: url, Channel: "alerts"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	other := connect(t, url)
	_ = other.Publish("alerts", []byte("one"))
	_ = other.Publish("alerts", []byte("two"))
	_ = other.Flush()

	// Act
	got, err := s.Receive(t.Context(), transport.Receive{Count: 2, Timeout: 2 * time.Second})

	// Assert
	if err != nil || len(got) != 2 || string(got[0].Body) != "one" || string(got[1].Body) != "two" {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestOpen_should_publish_over_the_open_connection(t *testing.T) {
	// Arrange
	url := startServer(t, nil)
	other := connect(t, url)
	sub, err := other.SubscribeSync("alerts")
	if err != nil {
		t.Fatal(err)
	}
	_ = other.Flush()
	s, err := nats.New().Open(t.Context(), transport.Operation{Server: url, Channel: "alerts"})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Act
	_, sendErr := s.Send(t.Context(), transport.Message{Body: []byte("hello")})
	msg, nextErr := sub.NextMsg(2 * time.Second)

	// Assert
	if sendErr != nil || nextErr != nil || string(msg.Data) != "hello" {
		t.Fatalf("send %v, next %v, %v", sendErr, nextErr, msg)
	}
}

func TestOpen_Given_a_JetStream_stream_Then_says_it_cannot_be_kept_open(t *testing.T) {
	url := startServer(t, nil)

	_, err := nats.New().Open(t.Context(), transport.Operation{Server: url, Channel: "x", Binding: map[string]string{"stream": "S"}})

	if err == nil {
		t.Fatal("a JetStream session is not supported and must say so")
	}
}
