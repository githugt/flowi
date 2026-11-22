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

func TestCheckGroup(t *testing.T) {
	tests := []struct {
		name  string
		group string
		ok    bool
	}{
		{"the default group is no name", "", true},
		{"a plain name", "checkout services", true},
		{"letters of any script", "Café 日本", true},
		{"a space at the start", " checkout", false},
		{"a space at the end", "checkout ", false},
		{"the default's own name", "Default", false},
		{"the default's own name in lower case", "default", false},
		{"a control character", "check\nout", false},
		{"a bidi override", "check\u202eout", false},
		{"too long", strings.Repeat("x", MaxGroupRunes+1), false},
		{"as long as allowed", strings.Repeat("x", MaxGroupRunes), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckGroup(tt.group)

			if (err == nil) != tt.ok {
				t.Fatalf("CheckGroup(%q) = %v, want ok=%v", tt.group, err, tt.ok)
			}
		})
	}
}

func TestEnvironmentGroups_Given_environments_in_groups_Then_the_named_groups_are_listed_once_and_sorted(t *testing.T) {
	envs := []Environment{{Meta: Meta{Name: "a"}, Groups: []string{"payments"}}, {Meta: Meta{Name: "b"}}, {Meta: Meta{Name: "c"}, Groups: []string{"checkout"}}, {Meta: Meta{Name: "d"}, Groups: []string{"payments"}}}

	got := EnvironmentGroups(envs)

	if !reflect.DeepEqual(got, []string{"checkout", "payments"}) {
		t.Fatalf("got %v", got)
	}
	if in := EnvironmentsInGroup(envs, ""); len(in) != 1 || in[0].Name != "b" {
		t.Fatalf("default group %v", in)
	}
	if in := EnvironmentsInGroup(envs, "payments"); len(in) != 2 {
		t.Fatalf("payments %v", in)
	}
}

// Before: an environment had one group, so a service shared by two sets of environments could be compared with only one.
func TestEnvironment_Given_several_groups_Then_it_is_in_each_and_the_default_group_only_when_it_has_none(t *testing.T) {
	envs := []Environment{{Meta: Meta{Name: "shared"}, Groups: []string{"checkout", "payments"}}, {Meta: Meta{Name: "plain"}}}

	if !envs[0].InGroup("checkout") || !envs[0].InGroup("payments") || envs[0].InGroup("") || envs[0].InGroup("other") {
		t.Errorf("shared: %+v", envs[0])
	}
	if !envs[1].InGroup("") || envs[1].InGroup("checkout") {
		t.Errorf("plain: %+v", envs[1])
	}
	if got := EnvironmentGroups(envs); !reflect.DeepEqual(got, []string{"checkout", "payments"}) {
		t.Errorf("groups %v", got)
	}
	if in := EnvironmentsInGroup(envs, "payments"); len(in) != 1 || in[0].Name != "shared" {
		t.Errorf("payments %v", in)
	}
}

func TestCheckGroups_should_refuse_an_empty_name_a_repeat_and_a_name_CheckGroup_refuses(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		ok     bool
	}{
		{"none", nil, true},
		{"two", []string{"a", "b"}, true},
		{"an empty name", []string{"a", ""}, false},
		{"a repeat", []string{"a", "b", "a"}, false},
		{"the default's name", []string{"Default"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckGroups(tt.groups); (err == nil) != tt.ok {
				t.Fatalf("CheckGroups(%q) = %v, want ok=%v", tt.groups, err, tt.ok)
			}
		})
	}
}

func TestNormaliseGroups_should_sort_and_drop_repeats_and_empty_names(t *testing.T) {
	got := NormaliseGroups([]string{"b", "", "a", "b"})

	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("got %q", got)
	}
}

// Before: an environment was in the default group only when it had no named group, so it could not be in the default
// group and a named one at once.
func TestEnvironment_Given_the_default_token_Then_it_is_in_the_default_group_as_well_as_the_named_ones(t *testing.T) {
	e := Environment{Meta: Meta{Name: "shared"}, Groups: []string{"checkout", DefaultGroupToken}}

	if !e.InGroup("") || !e.InGroup("checkout") || e.InGroup("payments") {
		t.Errorf("membership of %+v", e)
	}
	if got := e.Memberships(); !reflect.DeepEqual(got, []string{"", "checkout"}) {
		t.Errorf("memberships %q", got)
	}
	if got := (Environment{Groups: []string{"b", "a"}}).Memberships(); !reflect.DeepEqual(got, []string{"b", "a"}) {
		t.Errorf("a named-only environment: %q", got)
	}
	if got := (Environment{}).Memberships(); !reflect.DeepEqual(got, []string{""}) {
		t.Errorf("an environment with none: %q", got)
	}
	if got := EnvironmentGroups([]Environment{e}); !reflect.DeepEqual(got, []string{"checkout"}) {
		t.Errorf("the token is not a group of its own: %q", got)
	}
}

func TestCheckGroups_should_accept_the_default_token_once_and_refuse_other_spellings_of_default(t *testing.T) {
	tests := []struct {
		name   string
		groups []string
		ok     bool
	}{
		{"the token beside a name", []string{"a", DefaultGroupToken}, true},
		{"the token twice", []string{DefaultGroupToken, DefaultGroupToken}, false},
		{"another case", []string{"a", "Default"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := CheckGroups(tt.groups); (err == nil) != tt.ok {
				t.Fatalf("CheckGroups(%q) = %v, want ok=%v", tt.groups, err, tt.ok)
			}
		})
	}
}

func TestNormaliseGroups_Given_only_the_default_token_Then_it_is_the_same_as_no_group(t *testing.T) {
	if got := NormaliseGroups([]string{DefaultGroupToken}); got != nil {
		t.Errorf("got %q", got)
	}
	if got := NormaliseGroups([]string{"b", DefaultGroupToken, "a"}); !reflect.DeepEqual(got, []string{"a", "b", DefaultGroupToken}) {
		t.Errorf("got %q", got)
	}
}
