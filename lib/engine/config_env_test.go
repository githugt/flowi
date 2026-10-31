// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestSelectEnvironment_Given_an_environment_that_extends_Then_its_own_values_win_and_the_base_fills_the_rest(t *testing.T) {
	// Arrange
	c := model.Collection{Environments: []model.Environment{
		{Meta: model.Meta{Name: "base"}, Vars: []model.EnvVar{{Name: "host", Value: "base.test"}, {Name: "region", Value: "eu"}}},
		{Meta: model.Meta{Name: "staging"}, Extends: "base", Vars: []model.EnvVar{{Name: "host", Value: "staging.test"}}},
	}}

	// Act
	env, diags := selectEnvironment(c, "staging", "")

	// Assert
	if len(diags) != 0 || env.name != "staging" || env.values["host"] != "staging.test" || env.values["region"] != "eu" {
		t.Fatalf("env %+v diags %v", env, diags)
	}
}

func TestSelectEnvironment_Given_a_base_secret_Then_it_is_not_inherited(t *testing.T) {
	// Arrange: a secret's value lives in the vault under the environment that declares it, so a child that
	// wants one declares its own.
	c := model.Collection{Environments: []model.Environment{
		{Meta: model.Meta{Name: "base"}, Vars: []model.EnvVar{{Name: "token", Secret: true}}},
		{Meta: model.Meta{Name: "staging"}, Extends: "base"},
	}}

	// Act
	env, _ := selectEnvironment(c, "staging", "")

	// Assert
	if env.secrets["token"] {
		t.Fatalf("a base secret must not be inherited: %+v", env.secrets)
	}
}

func TestSelectEnvironment_Given_a_missing_base_Then_it_reports_the_base(t *testing.T) {
	// Arrange
	c := model.Collection{Environments: []model.Environment{{Meta: model.Meta{Name: "staging"}, Extends: "nope"}}}

	// Act
	_, diags := selectEnvironment(c, "staging", "")

	// Assert
	if len(diags) != 1 || diags[0].Code != "FLW-REQ-008" {
		t.Fatalf("diags %v", diags)
	}
}

func TestSelectEnvironment_Given_typed_variables_Then_the_values_keep_their_types(t *testing.T) {
	// Arrange
	c := model.Collection{Environments: []model.Environment{{Meta: model.Meta{Name: "staging"}, Vars: []model.EnvVar{
		{Name: "count", Value: "5", Kind: model.VarNumber},
		{Name: "on", Value: "true", Kind: model.VarBoolean},
		{Name: "limits", Value: `{"max":3}`, Kind: model.VarJSON},
		{Name: "text", Value: "5"},
	}}}}

	// Act
	env, diags := selectEnvironment(c, "staging", "")

	// Assert
	limits, _ := env.values["limits"].(map[string]any)
	if len(diags) != 0 || env.values["count"] != int64(5) || env.values["on"] != true || limits["max"] != float64(3) || env.values["text"] != "5" {
		t.Fatalf("values %#v diags %v", env.values, diags)
	}
}

func TestSelectEnvironment_Given_a_value_that_does_not_fit_its_kind_Then_the_run_is_refused_naming_the_variable(t *testing.T) {
	// Arrange
	c := model.Collection{Environments: []model.Environment{{Meta: model.Meta{Name: "staging"}, Vars: []model.EnvVar{{Name: "count", Value: "five", Kind: model.VarNumber}}}}}

	// Act
	_, diags := selectEnvironment(c, "staging", "")

	// Assert
	if len(diags) != 1 || diags[0].Code != "FLW-REQ-008" {
		t.Fatalf("diags %v", diags)
	}
}
