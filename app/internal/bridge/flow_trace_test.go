// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"
)

func TestTrace_should_name_the_steps_that_make_and_read_a_value(t *testing.T) {
	// Arrange
	flows, v, _ := openTwoUnits(t)
	_ = flows.ConnectData(v.Node, "login", "status", "verify", "with.code")

	// Act
	got := flows.Trace(v.Node, "login.status")

	// Assert
	if got.Kind != "output" || !slices.Equal(got.Producers, []string{"login"}) || !slices.Equal(got.Readers, []string{"verify"}) {
		t.Fatalf("trace %+v", got)
	}
	if !slices.Equal(got.Lit, []string{"login", "verify"}) {
		t.Fatalf("lit %v: the steps the canvas keeps bright are the makers and the readers", got.Lit)
	}
}

func TestTrace_Given_an_unknown_flow_Then_an_empty_trace(t *testing.T) {
	flows, _, _ := openTwoUnits(t)

	got := flows.Trace("no-such-flow", "x")

	if len(got.Lit) != 0 || got.Producers == nil || got.Readers == nil {
		t.Fatalf("trace %+v: slices must be empty, not null, for the frontend", got)
	}
}
