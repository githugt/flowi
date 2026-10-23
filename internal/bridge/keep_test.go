// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitKept_should_report_edits_that_may_be_lost(t *testing.T) {
	t.Parallel()
	answered := func(kept bool) chan bool {
		c := make(chan bool, 1)
		c <- kept
		return c
	}
	tests := []struct {
		name    string
		pending func() []chan bool
		want    bool
	}{
		{name: "no open requests", pending: func() []chan bool { return nil }, want: false},
		{name: "every edit kept", pending: func() []chan bool { return []chan bool{answered(true), answered(true)} }, want: false},
		{name: "a slot refused an edit", pending: func() []chan bool { return []chan bool{answered(true), answered(false)} }, want: true},
		{name: "an edit never answered", pending: func() []chan bool { return []chan bool{answered(true), make(chan bool)} }, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				got := waitKept(tt.pending(), time.Second)

				if got != tt.want {
					t.Fatalf("lost %v, want %v", got, tt.want)
				}
			})
		})
	}
}

// Before, each request got its own timeout in turn, so three stuck requests
// held a quit for three timeouts.
func TestWaitKept_Given_several_stuck_edits_Then_it_waits_one_timeout_in_all(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		stuck := []chan bool{make(chan bool), make(chan bool), make(chan bool)}
		start := time.Now()

		lost := waitKept(stuck, keepEditsTimeout)

		if waited := time.Since(start); !lost || waited != keepEditsTimeout {
			t.Fatalf("lost %v after %s, want lost after %s", lost, waited, keepEditsTimeout)
		}
	})
}
