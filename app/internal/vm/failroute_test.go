// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import "testing"

func TestFailRoute_should_round_trip_every_mode(t *testing.T) {
	tests := []struct {
		onError string
		mode    FailMode
		target  string
	}{
		{"", FailStop, ""},
		{"fail", FailStop, ""},
		{"continue", FailContinue, ""},
		{"goto:cleanup", FailGoto, "cleanup"},
		{"goto: cleanup", FailGoto, "cleanup"},
		{"nonsense", FailStop, ""},
	}
	for _, tt := range tests {
		t.Run(tt.onError, func(t *testing.T) {
			mode, target := ParseFailRoute(tt.onError)

			if mode != tt.mode || target != tt.target {
				t.Fatalf("got %q %q", mode, target)
			}
		})
	}
	if FailRoute(FailStop, "") != "" || FailRoute(FailContinue, "") != "continue" || FailRoute(FailGoto, "x") != "goto:x" {
		t.Fatal("FailRoute")
	}
}
