// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestValidate_Given_an_environment_that_extends_a_missing_base_Then_the_file_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New(), Name: "staging"}, Extends: "nope", File: "staging.yaml"}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeRequestEnvExtends)
	if d.File != "staging.yaml" || d.Path != "$.extends" {
		t.Fatalf("location %+v", d.Location)
	}
}

func TestValidate_Given_environments_that_extend_each_other_Then_the_loop_is_reported_once_per_file(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "a"}, Extends: "b", File: "a.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "b"}, Extends: "a", File: "b.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	if n := len(got); n != 2 {
		t.Fatalf("want one per file, got %v", codesOf(got))
	}
}
