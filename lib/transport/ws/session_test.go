// SPDX-License-Identifier: Apache-2.0

package ws_test

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/ws"
)

// echoUpper answers every frame with its upper-case form, after greeting.
func echoServer(t *testing.T) string {
	return serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		_ = c.Write(ctx, websocket.MessageText, []byte("welcome"))
		for {
			_, b, err := c.Read(ctx)
			if err != nil {
				return
			}
			_ = c.Write(ctx, websocket.MessageText, append([]byte("re:"), b...))
		}
	})
}

func TestOpen_should_hold_what_arrives_between_steps_and_answer_a_later_send(t *testing.T) {
	// Arrange
	s, err := ws.New().Open(t.Context(), transport.Operation{Server: echoServer(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Act
	greeting, greetErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})
	sent, sendErr := s.Send(t.Context(), transport.Message{Body: []byte("ping")})
	reply, replyErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})

	// Assert
	if greetErr != nil || sendErr != nil || replyErr != nil {
		t.Fatalf("errors %v %v %v", greetErr, sendErr, replyErr)
	}
	if got := texts(greeting); len(got) != 1 || got[0] != "welcome" {
		t.Errorf("greeting %v", got)
	}
	if sent.Direction != "sent" || string(sent.Body) != "ping" {
		t.Errorf("sent %+v", sent)
	}
	if got := texts(reply); len(got) != 1 || got[0] != "re:ping" {
		t.Errorf("reply %v", got)
	}
}

func TestOpen_should_keep_the_connection_after_a_receive_times_out(t *testing.T) {
	// Arrange
	s, err := ws.New().Open(t.Context(), transport.Operation{Server: echoServer(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second}); err != nil {
		t.Fatal(err)
	}

	// Act
	_, timedOut := s.Receive(t.Context(), transport.Receive{Timeout: 50 * time.Millisecond})
	_, sendErr := s.Send(t.Context(), transport.Message{Body: []byte("again")})
	reply, replyErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})

	// Assert
	if !errors.Is(timedOut, transport.ErrTimeout) {
		t.Fatalf("first receive: %v", timedOut)
	}
	if sendErr != nil || replyErr != nil || len(reply) != 1 {
		t.Fatalf("after the timeout: send %v, receive %v %v", sendErr, replyErr, reply)
	}
}

func TestOpen_should_outlive_the_context_that_opened_it(t *testing.T) {
	// Arrange
	opening, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	s, err := ws.New().Open(opening, transport.Operation{Server: echoServer(t)})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Act
	cancel()
	got, recvErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})

	// Assert
	if recvErr != nil || len(got) != 1 {
		t.Fatalf("the step that opened the connection has ended; the connection must not: %v %v", got, recvErr)
	}
}

func TestOpen_should_report_a_server_that_hangs_up_after_the_messages_it_sent(t *testing.T) {
	// Arrange
	server := serve(t, func(ctx context.Context, c *websocket.Conn, _ *http.Request) {
		_ = c.Write(ctx, websocket.MessageText, []byte("bye"))
		_ = c.Close(websocket.StatusGoingAway, "")
	})
	s, err := ws.New().Open(t.Context(), transport.Operation{Server: server})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// Act
	first, firstErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})
	_, secondErr := s.Receive(t.Context(), transport.Receive{Timeout: 2 * time.Second})

	// Assert
	if firstErr != nil || len(first) != 1 || secondErr == nil || errors.Is(secondErr, transport.ErrTimeout) {
		t.Fatalf("first %v %v, second %v", first, firstErr, secondErr)
	}
}

func TestSession_Close_should_be_safe_twice(t *testing.T) {
	s, err := ws.New().Open(t.Context(), transport.Operation{Server: echoServer(t)})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}
