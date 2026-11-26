// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"testing"
)

func TestCallers_Given_flows_in_another_collection_When_asked_Then_every_step_that_runs_the_request_is_listed(t *testing.T) {
	cs := loadUsage(t)
	login := Target{Collection: identityID, Request: loginID}

	got := Callers(cs, []Target{login})[login]

	var signup, legacy bool
	for _, c := range got {
		signup = signup || (c.Flow == "signup" && c.Node == "login" && c.Collection == "onboarding")
		legacy = legacy || (c.Flow == "legacy" && c.Node == "login" && c.Collection == "onboarding")
	}
	if !signup || !legacy {
		t.Fatalf("got %+v, want the signup and legacy login steps of onboarding", got)
	}
}

func TestCallers_Given_a_request_nothing_runs_When_asked_Then_it_has_no_entry(t *testing.T) {
	// Before: nothing told a delete that a request was free to go, so the window could not tell it from one in use.
	cs := loadUsage(t)
	unused := Target{Collection: identityID, Request: "01JB00000000000000000000ZZ"}

	got := Callers(cs, []Target{unused})

	if len(got) != 0 {
		t.Fatalf("got %+v, want none for an unknown request", got)
	}
}
