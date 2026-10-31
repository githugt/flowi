// SPDX-License-Identifier: Apache-2.0

package model_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

// A step could only be given a colour; "no colour" meant the category's own, which is blue for a request,
// so there was no way to make a step plain.
func TestNodeColors_should_include_a_neutral_black(t *testing.T) {
	got := model.NodeColors()

	if !slices.Contains(got, "black") {
		t.Fatalf("colours %v have no black", got)
	}
}

// An outcome starts green or red (success, failure) and a named one aqua blue, which the step colours did
// not have: a second "blue" was the only blue there was.
func TestNodeColors_should_include_red_and_an_aqua_blue(t *testing.T) {
	got := model.NodeColors()

	if !slices.Contains(got, "red") || !slices.Contains(got, "sky") {
		t.Fatalf("colours %v", got)
	}
}
