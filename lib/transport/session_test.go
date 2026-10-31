// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func body(m Message) string { return string(m.Body) }

func TestInbox_Wait_should_return_messages_that_arrived_before_it_was_asked(t *testing.T) {
	// Arrange
	in := NewInbox(0)
	in.Push(Message{Body: []byte("a")})
	in.Push(Message{Body: []byte("b")})

	// Act
	got, err := in.Wait(t.Context(), Receive{Count: 2})

	// Assert
	if err != nil || len(got) != 2 || body(got[0]) != "a" || body(got[1]) != "b" {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestInbox_Wait_should_leave_later_messages_for_the_next_wait(t *testing.T) {
	// Arrange
	in := NewInbox(0)
	for _, s := range []string{"a", "b", "c"} {
		in.Push(Message{Body: []byte(s)})
	}

	// Act
	first, _ := in.Wait(t.Context(), Receive{Count: 1})
	rest, err := in.Wait(t.Context(), Receive{Count: 2})

	// Assert
	if err != nil || body(first[0]) != "a" || len(rest) != 2 || body(rest[0]) != "b" {
		t.Fatalf("first %v rest %v err %v", first, rest, err)
	}
}

func TestInbox_Wait_should_drop_a_message_that_does_not_match_and_keep_waiting(t *testing.T) {
	// Arrange
	in := NewInbox(0)
	in.Push(Message{Body: []byte("noise")})
	in.Push(Message{Body: []byte("answer")})
	r := Receive{Match: func(m Message) (bool, error) { return string(m.Body) == "answer", nil }}

	// Act
	got, err := in.Wait(t.Context(), r)

	// Assert
	if err != nil || len(got) != 1 || body(got[0]) != "answer" {
		t.Fatalf("got %v, err %v", got, err)
	}
}

func TestInbox_Wait_should_time_out_with_what_it_has_and_keep_the_connection_usable(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Arrange
		in := NewInbox(0)
		in.Push(Message{Body: []byte("a")})

		// Act
		got, err := in.Wait(t.Context(), Receive{Count: 2, Timeout: time.Second})
		in.Push(Message{Body: []byte("b")})
		later, laterErr := in.Wait(t.Context(), Receive{Count: 1})

		// Assert
		if !errors.Is(err, ErrTimeout) || len(got) != 1 {
			t.Fatalf("got %v, err %v", got, err)
		}
		if laterErr != nil || len(later) != 1 || body(later[0]) != "b" {
			t.Fatalf("later %v, err %v", later, laterErr)
		}
	})
}

func TestInbox_Wait_should_wake_when_a_message_arrives_while_waiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// Arrange
		in := NewInbox(0)
		go func() {
			time.Sleep(time.Second)
			in.Push(Message{Body: []byte("late")})
		}()

		// Act
		got, err := in.Wait(t.Context(), Receive{Timeout: time.Minute})

		// Assert
		if err != nil || len(got) != 1 || body(got[0]) != "late" {
			t.Fatalf("got %v, err %v", got, err)
		}
	})
}

func TestInbox_Wait_should_report_a_lost_connection_after_the_messages_it_held(t *testing.T) {
	// Arrange
	in := NewInbox(0)
	in.Push(Message{Body: []byte("a")})
	lost := errors.New("the broker hung up")
	in.Fail(lost)

	// Act
	first, firstErr := in.Wait(t.Context(), Receive{})
	_, secondErr := in.Wait(t.Context(), Receive{})

	// Assert
	if firstErr != nil || len(first) != 1 {
		t.Fatalf("first %v err %v", first, firstErr)
	}
	if !errors.Is(secondErr, lost) {
		t.Fatalf("second err %v", secondErr)
	}
}

func TestInbox_Push_should_refuse_to_hold_more_than_its_limit(t *testing.T) {
	// Arrange
	in := NewInbox(5)

	// Act
	in.Push(Message{Body: []byte("abc")})
	in.Push(Message{Body: []byte("def")})
	_, err := in.Wait(t.Context(), Receive{Count: 2})

	// Assert
	if !errors.Is(err, ErrInboxFull) {
		t.Fatalf("a connection nobody reads must not fill memory, got %v", err)
	}
}

func TestInbox_Wait_should_stop_when_the_context_ends(t *testing.T) {
	// Arrange
	in := NewInbox(0)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	// Act
	_, err := in.Wait(ctx, Receive{})

	// Assert
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
