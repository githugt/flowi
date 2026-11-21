// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"testing"

	"github.com/githugt/flowi/lib/serve/replay"
)

// Before: the replayer compared host strings, so localhost:8080 was not seen as 127.0.0.1:8080 and a forward to it
// looped back into the replayer.
func TestSameHost(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"127.0.0.1:8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "localhost:8080", true},
		{"LOCALHOST:8080", "[::1]:8080", true},
		{"127.0.0.2:8080", "127.0.0.1:8080", true},
		{"[::]:8080", "localhost:8080", true},
		{"0.0.0.0:8080", "127.0.0.1:8080", true},
		{"127.0.0.1:8080", "127.0.0.1:8081", false},
		{"127.0.0.1:8080", "localhost", false},
		{"api.example.com:8080", "127.0.0.1:8080", false},
		{"Api.Example.com:80", "api.example.com:80", true},
		{"10.0.0.5:8080", "127.0.0.1:8080", false},
	}
	for _, tt := range tests {
		if got := replay.SameHost(tt.a, tt.b); got != tt.want {
			t.Errorf("SameHost(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}
