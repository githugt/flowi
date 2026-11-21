// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
	flowi "github.com/githugt/flowi/lib"
)

// Before: a missing variable or secret was found halfway through a run.
func TestPreflightLines_should_say_what_each_need_is_in_plain_words(t *testing.T) {
	needs := []flowi.Need{{Kind: flowi.NeedVariable, Name: "region"}, {Kind: flowi.NeedSecret, Name: "key"}, {Kind: flowi.NeedSecretValue, Name: "token"}}

	got := vm.PreflightLines("staging", needs)

	want := []string{
		"The variable region is read but staging does not define it.",
		"The secret key is read but staging does not declare it.",
		"The secret token has no value in staging.",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

// Before: a run in a protected environment asked a generic question that named no step.
func TestWriteLines_should_name_each_step_how_it_writes_and_the_subflow_it_is_in(t *testing.T) {
	writes := []flowi.FlowWrite{
		{Step: "login", Request: "auth/login", What: "POST"},
		{Step: "make", Request: "users/create", What: "PUT", Via: "shared/setup"},
	}

	got := vm.WriteLines(writes)

	want := []string{"login: POST (auth/login)", "make: PUT (users/create) in shared/setup"}
	if !slices.Equal(got, want) || len(vm.WriteLines(nil)) != 0 {
		t.Fatalf("got %q", got)
	}
}
