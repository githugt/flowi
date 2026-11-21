// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestReadCancelable_Given_done_while_reading_Then_cancelled_and_terminal_restored(t *testing.T) {
	done := make(chan struct{})
	blocked := make(chan struct{})
	restored := false
	time.AfterFunc(50*time.Millisecond, func() { close(done) })

	_, err := readCancelable(func() ([]byte, error) { <-blocked; return nil, nil }, done, func() { restored = true })

	if !errors.Is(err, context.Canceled) || !restored {
		t.Fatalf("err %v restored %v", err, restored)
	}
	close(blocked)
}

func TestReadCancelable_Given_input_Then_returns_it(t *testing.T) {
	got, err := readCancelable(func() ([]byte, error) { return []byte("pw"), nil }, make(chan struct{}), func() { t.Error("restored without cancel") })

	if err != nil || got != "pw" {
		t.Fatalf("got %q %v", got, err)
	}
}

// Before: NO_COLOR= (present but empty) turned colour off.
func TestNewConsole_should_turn_colour_off_only_for_a_non_empty_NO_COLOR(t *testing.T) {
	tests := []struct {
		environ []string
		want    bool
	}{
		{nil, false},
		{[]string{"NO_COLOR="}, false},
		{[]string{"NO_COLOR=1"}, true},
		{[]string{"OTHER=1", "NO_COLOR=yes"}, true},
	}
	for _, tt := range tests {
		got := NewConsole(nil, nil, nil, tt.environ, nil).NoColor

		if got != tt.want {
			t.Errorf("environ %v: NoColor %v, want %v", tt.environ, got, tt.want)
		}
	}
}
