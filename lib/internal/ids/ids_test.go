// SPDX-License-Identifier: Apache-2.0

package ids

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/model"
)

func TestSource_New_should_return_valid_increasing_ids_for_fixed_clock(t *testing.T) {
	fixed := time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)
	src := NewSource(func() time.Time { return fixed }, rand.NewChaCha8([32]byte{1, 2, 3}))
	var prev model.ID

	for range 10 {
		id := src.New()

		if !Valid(id) {
			t.Fatalf("invalid id %q", id)
		}
		if id <= prev {
			t.Fatalf("id %q not greater than %q", id, prev)
		}
		prev = id
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		id   model.ID
		want bool
	}{
		{"01J9Z8K3M4N5P6Q7R8S9T0VWXY", true},
		{"01j9z8k3m4n5p6q7r8s9t0vwxy", false},
		{"01J9Z8K3M4N5P6Q7R8S9T0VWX", false},
		{"01J9Z8K3M4N5P6Q7R8S9T0VWXI", false},
		{"01J9Z8K3M4N5P6Q7R8S9T0VWXL", false},
		{"01J9Z8K3M4N5P6Q7R8S9T0VWXO", false},
		{"01J9Z8K3M4N5P6Q7R8S9T0VWXU", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			got := Valid(tt.id)

			if got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}
