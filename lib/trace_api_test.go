// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"
)

func TestTraceValue_should_name_the_step_that_makes_a_variable_and_the_steps_that_read_it(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.FindFlow("user-creation", "")
	if err != nil {
		t.Fatal(err)
	}

	got, err := w.TraceValue(target, "token")

	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.Producers, []string{"session"}) || !slices.Contains(got.Readers, "verify") {
		t.Fatalf("got %+v", got)
	}
}

func TestTraceValue_Given_an_unknown_flow_Then_errors(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.TraceValue(FlowTarget{Collection: "nope", Flow: "nope"}, "token")

	if err == nil {
		t.Fatal("want an error")
	}
}
