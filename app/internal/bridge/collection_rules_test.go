// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

func TestCollection_SpecRules_Then_each_rule_comes_with_its_level_and_whether_the_collection_chose_it(t *testing.T) {
	// Arrange
	app, b := checksApp(t)
	b.Check.Rules = []flowi.SpecRule{
		{Name: "path-case", About: "Path segments are lowercase.", Default: "off", Level: "error", Chosen: true},
		{Name: "operation-id", About: "Every operation has an operationId.", Default: "warning", Level: "warning"},
	}

	// Act
	got, err := app.Collection().SpecRules("C1")

	// Assert
	if err != nil || len(got) != 2 || got[0].Name != "path-case" || got[0].Level != "error" || !got[0].Chosen || got[1].Chosen {
		t.Fatalf("rules %+v err %v", got, err)
	}
}

func TestCollection_SetSpecRule_Then_the_backend_is_told(t *testing.T) {
	// Arrange
	app, b := checksApp(t)

	// Act
	err := app.Collection().SetSpecRule("C1", "path-case", "error")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(b.Calls, "SetSpecLintRule C1 path-case error") {
		t.Fatalf("calls %v", b.Calls)
	}
}
