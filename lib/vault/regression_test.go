// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"strings"
	"testing"
)

// A secret that shares a value or a name
// with something else lost its value. Here: two secrets where one contains
// the other must both be masked whole.
func TestMasker_Given_secret_containing_another_When_masking_Then_longer_is_masked_whole(t *testing.T) {
	m := NewMasker("abc", "abcdef")

	got := m.String("token=abcdef")

	if strings.Contains(got, "def") {
		t.Fatalf("partial leak: %q", got)
	}
}

// A one- or two-character secret masks every occurrence of that text in
// logs and reports, which makes them unreadable. Documents the behaviour.
func TestMasker_Given_very_short_secret_When_masking_Then_behaviour_is_recorded(t *testing.T) {
	m := NewMasker("1")

	got := m.String("status 200 in 15ms")

	t.Logf("short secret masks unrelated text: %q", got)
}
