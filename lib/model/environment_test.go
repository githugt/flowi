// SPDX-License-Identifier: Apache-2.0

package model

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDefaultEnvColour(t *testing.T) {
	tests := []struct{ name, want string }{
		{"prod", "red"},
		{"Production", "red"},
		{"PROD-eu", "red"},
		{"staging", ""},
		{"preprod", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DefaultEnvColour(tt.name)

			if got != tt.want {
				t.Fatalf("DefaultEnvColour(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestEnvironmentChain_Given_a_child_When_resolved_Then_the_base_comes_first(t *testing.T) {
	// Arrange
	envs := []Environment{{Meta: Meta{Name: "staging"}, Extends: "base"}, {Meta: Meta{Name: "base"}}}

	// Act
	chain, err := EnvironmentChain(envs, "staging")

	// Assert
	if err != nil || len(chain) != 2 || chain[0].Name != "base" || chain[1].Name != "staging" {
		t.Fatalf("chain %+v err %v", chain, err)
	}
}

func TestEnvironmentChain_Given_a_missing_base_Then_the_error_names_it(t *testing.T) {
	// Arrange
	envs := []Environment{{Meta: Meta{Name: "staging"}, Extends: "nope"}}

	// Act
	_, err := EnvironmentChain(envs, "staging")

	// Assert
	if !errors.Is(err, ErrEnvironmentBase) || !strings.Contains(err.Error(), `"nope"`) {
		t.Fatalf("err %v", err)
	}
}

func TestEnvironmentChain_Given_a_loop_Then_it_is_refused_instead_of_followed_forever(t *testing.T) {
	// Arrange
	envs := []Environment{{Meta: Meta{Name: "a"}, Extends: "b"}, {Meta: Meta{Name: "b"}, Extends: "a"}}

	// Act
	_, err := EnvironmentChain(envs, "a")

	// Assert
	if !errors.Is(err, ErrEnvironmentLoop) {
		t.Fatalf("err %v", err)
	}
}

func TestEnvVarTyped_Given_each_kind_Then_the_value_has_that_type(t *testing.T) {
	tests := []struct {
		name string
		v    EnvVar
		want any
	}{
		{"text", EnvVar{Value: "5"}, "5"},
		{"whole number", EnvVar{Value: "5", Kind: VarNumber}, int64(5)},
		{"fraction", EnvVar{Value: "1.5", Kind: VarNumber}, 1.5},
		{"boolean", EnvVar{Value: "true", Kind: VarBoolean}, true},
		{"object", EnvVar{Value: `{"a":[1,2]}`, Kind: VarJSON}, map[string]any{"a": []any{float64(1), float64(2)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.v.Typed()

			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v (%v), want %#v", got, err, tt.want)
			}
		})
	}
}

func TestEnvVarTyped_Given_a_value_that_does_not_fit_its_kind_Then_it_is_an_error_that_names_the_kind(t *testing.T) {
	for _, v := range []EnvVar{{Name: "n", Value: "five", Kind: VarNumber}, {Name: "b", Value: "yes", Kind: VarBoolean}, {Name: "o", Value: "{", Kind: VarJSON}} {
		_, err := v.Typed()

		if err == nil || !strings.Contains(err.Error(), v.Name) {
			t.Errorf("%s: err %v", v.Name, err)
		}
	}
}
