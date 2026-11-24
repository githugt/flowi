// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestCheckArrows_Given_a_step_with_arrows_When_validated_Then_only_square_and_curved_pass(t *testing.T) {
	// Before: a step had no say in how its arrows were drawn, so a file naming one was an unknown field.
	tests := []struct {
		name   string
		arrows string
		bad    bool
	}{
		{name: "square", arrows: "square"},
		{name: "curved", arrows: "curved"},
		{name: "left_out", arrows: ""},
		{name: "wavy", arrows: "wavy", bad: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := note("a")
			n.ArrowStyle = tt.arrows

			got := validateFlow(t, nil, n)

			if tt.bad != hasDiag(got, diag.CodeNodeParam, diag.SeverityError) || (!tt.bad && len(got) != 0) {
				t.Fatalf("got %v %v", codesOf(got), got)
			}
		})
	}
}
