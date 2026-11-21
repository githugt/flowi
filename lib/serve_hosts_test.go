// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"
)

// Before: a listener at localhost:PORT got no Host check at all, so a page reaching it through a rebound name could
// read the replayer, while 127.0.0.1:PORT was guarded.
func TestLoopbackHosts_should_guard_a_listener_named_localhost_and_leave_a_network_one_open(t *testing.T) {
	tests := []struct {
		addr    string
		guarded bool
	}{
		{"127.0.0.1:8525", true},
		{"[::1]:8525", true},
		{"localhost:8525", true},
		{"LOCALHOST:8525", true},
		{"0.0.0.0:8525", false},
		{":8525", false},
		{"192.168.1.5:8525", false},
		{"example.com:8525", false},
		{"not an address", false},
	}
	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			got := loopbackHosts(tt.addr)

			if tt.guarded != (len(got) > 0) {
				t.Fatalf("loopbackHosts(%q) = %v, guarded want %v", tt.addr, got, tt.guarded)
			}
			if tt.guarded && !slices.Contains(got, "localhost") {
				t.Fatalf("hosts %v miss localhost", got)
			}
		})
	}
}
