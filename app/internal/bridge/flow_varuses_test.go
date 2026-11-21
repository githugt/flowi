// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: the variables bar listed names only.
func TestVarUsesOf_should_list_each_variable_with_its_writers_and_readers_by_name(t *testing.T) {
	f := vm.FlowVars{
		Writers: map[string][]string{"token": {"login"}},
		Readers: map[string][]string{"token": {"call"}, "region": {"call"}},
	}

	got := varUsesOf(f)

	if len(got) != 2 || got[0].Name != "region" || got[1].Name != "token" {
		t.Fatalf("got %+v", got)
	}
	if got[0].Writers == nil || len(got[0].Writers) != 0 || !slices.Equal(got[1].Writers, []string{"login"}) || !slices.Equal(got[1].Readers, []string{"call"}) {
		t.Fatalf("got %+v", got)
	}
}
