// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
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

// Before: an environment's group was not a thing, so nothing said that Compare would list a base apart from the
// environments that build on it.
func TestValidate_Given_an_environment_whose_base_is_in_another_group_Then_it_is_warned_about(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "checkout-prod"}, Groups: []string{"checkout"}, Extends: "base", File: "checkout-prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeRequestEnvGroupBase)
	if d.File != "checkout-prod.yaml" || d.Args["group"] != `the group "checkout"` || d.Args["baseGroup"] != "the default group" {
		t.Fatalf("diagnostic %+v", d)
	}
}

func TestValidate_Given_an_environment_and_its_base_in_the_same_group_Then_nothing_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, Groups: []string{"checkout"}, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Groups: []string{"checkout"}, Extends: "base", File: "prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

func TestValidate_Given_a_group_name_that_cannot_be_used_Then_the_group_line_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{{Meta: model.Meta{ID: f.ids.New(), Name: "a"}, Groups: []string{"Default"}, File: "a.yaml"}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeRequestEnvGroup)
	if d.File != "a.yaml" || d.Path != "$.groups" {
		t.Fatalf("location %+v", d.Location)
	}
}

// Before: an environment had one group, so a base in just one of two groups was never checked against the other.
func TestValidate_Given_an_environment_in_two_groups_and_a_base_in_only_one_Then_the_missing_group_is_named(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, Groups: []string{"checkout"}, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Groups: []string{"checkout", "payments"}, Extends: "base", File: "prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeRequestEnvGroupBase)
	if d.Args["group"] != `the group "payments"` || d.Args["baseGroup"] != `the group "checkout"` {
		t.Fatalf("diagnostic %+v", d)
	}
}

func TestValidate_Given_a_base_in_every_group_of_the_environment_and_more_Then_nothing_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, Groups: []string{"a", "b", "c"}, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Groups: []string{"a", "b"}, Extends: "base", File: "prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}

// Before: the default group could not be one of an environment's groups beside a named one, so a base in the default
// group alone could not be told apart from one that was in both.
func TestValidate_Given_an_environment_in_default_and_a_group_and_a_base_in_the_group_only_Then_the_default_group_is_named(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, Groups: []string{"checkout"}, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Groups: []string{"checkout", model.DefaultGroupToken}, Extends: "base", File: "prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeRequestEnvGroupBase)
	if d.Args["group"] != "the default group" || d.Args["baseGroup"] != `the group "checkout"` {
		t.Fatalf("diagnostic %+v", d)
	}
}

func TestValidate_Given_the_default_token_beside_a_name_and_a_base_that_has_both_Then_nothing_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	both := []string{"checkout", model.DefaultGroupToken}
	c.Environments = []model.Environment{
		{Meta: model.Meta{ID: f.ids.New(), Name: "base"}, Groups: both, File: "base.yaml"},
		{Meta: model.Meta{ID: f.ids.New(), Name: "prod"}, Groups: both, Extends: "base", File: "prod.yaml"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	if len(got) != 0 {
		t.Fatalf("got %v", codesOf(got))
	}
}
