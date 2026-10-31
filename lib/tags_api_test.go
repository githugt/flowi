// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

func TestFindFlowsTagged_Given_a_saved_tag_Then_the_flow_is_found_and_other_tags_find_nothing(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	f.Tags = []string{"smoke"}
	if err := w.SaveFlow(target, f, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Act
	found, err := w.FindFlowsTagged("smoke", "")
	_, missing := w.FindFlowsTagged("nightly", "")

	// Assert
	if err != nil || len(found) != 1 || found[0] != target {
		t.Fatalf("found %v err %v", found, err)
	}
	var le *diag.ListError
	if !errors.As(missing, &le) || le.List[0].Code != diag.CodeFlowTagEmpty {
		t.Fatalf("a tag nothing carries should say so, got %v", missing)
	}
}
