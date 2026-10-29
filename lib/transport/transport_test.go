// SPDX-License-Identifier: Apache-2.0

package transport_test

import (
	"errors"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/transport"
)

func feed(bodies ...string) chan transport.Message {
	ch := make(chan transport.Message, len(bodies))
	for _, b := range bodies {
		ch <- transport.Message{Body: []byte(b)}
	}
	return ch
}

func TestWait_should_stop_with_the_size_limit_and_keep_what_it_had(t *testing.T) {
	r := transport.Receive{Count: 3, Timeout: time.Second, MaxBytes: 5}

	got, err := transport.Wait(t.Context(), r, feed("abc", "def", "ghi"), nil)

	if !errors.Is(err, transport.ErrTooLarge) || len(got) != 2 {
		t.Fatalf("err %v kept %d", err, len(got))
	}
}

func TestWait_should_stop_when_the_match_fails(t *testing.T) {
	boom := errors.New("boom")
	r := transport.Receive{Timeout: time.Second, Match: func(transport.Message) (bool, error) { return false, boom }}

	_, err := transport.Wait(t.Context(), r, feed("a"), nil)

	if !errors.Is(err, boom) {
		t.Fatalf("err %v", err)
	}
}

func TestJoinURL_should_put_one_slash_between(t *testing.T) {
	tests := []struct{ server, channel, want string }{
		{"wss://h", "events", "wss://h/events"},
		{"wss://h/", "/events", "wss://h/events"},
		{"wss://h", "", "wss://h"},
	}
	for _, tt := range tests {
		if got := transport.JoinURL(tt.server, tt.channel); got != tt.want {
			t.Errorf("JoinURL(%q,%q) = %q, want %q", tt.server, tt.channel, got, tt.want)
		}
	}
}

func TestWait_should_report_each_message_as_it_is_kept(t *testing.T) {
	var seen []string
	r := transport.Receive{Count: 2, Timeout: time.Second, OnMessage: func(m transport.Message) { seen = append(seen, string(m.Body)) },
		Match: func(m transport.Message) (bool, error) { return string(m.Body) != "skip", nil }}

	got, err := transport.Wait(t.Context(), r, feed("a", "skip", "b"), nil)

	if err != nil || len(got) != 2 || len(seen) != 2 || seen[0] != "a" || seen[1] != "b" {
		t.Fatalf("err %v kept %d seen %v", err, len(got), seen)
	}
}

func TestWait_should_stop_at_the_message_the_keeper_refuses_and_return_what_it_had(t *testing.T) {
	// Arrange
	stop := errors.New("stop here")
	var seen []string
	r := transport.Receive{Count: 5, Timeout: time.Second, OnKept: func(m transport.Message) error {
		seen = append(seen, string(m.Body))
		if string(m.Body) == "b" {
			return stop
		}
		return nil
	}}

	// Act
	got, err := transport.Wait(t.Context(), r, feed("a", "b", "c"), nil)

	// Assert
	if !errors.Is(err, stop) || len(got) != 2 || len(seen) != 2 {
		t.Fatalf("err %v kept %d seen %v (the refused message is kept, the later one is not read)", err, len(got), seen)
	}
}
