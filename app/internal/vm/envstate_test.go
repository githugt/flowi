// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// Before: n/a (new). The sidebar must tell the selected environment from the rest and count what is unresolved in each.
func TestEnvironmentStates_Given_an_unset_secret_and_a_missing_base_When_listed_Then_each_is_an_issue_and_one_is_active(t *testing.T) {
	envs := []model.Environment{
		{Meta: model.Meta{Name: "local"}, File: "/e/local.yaml"},
		{Meta: model.Meta{Name: "staging"}, File: "/e/staging.yaml", Extends: "gone", Vars: []model.EnvVar{{Name: "token", Secret: true}, {Name: "key", Secret: true}}},
	}
	infos := []flowi.SecretInfo{{Environment: "staging", Name: "key", Set: true}}

	got := vm.EnvironmentStates(envs, infos, "local", true)

	if !got[0].Active || got[1].Active || len(got[0].Issues) != 0 {
		t.Fatalf("local: %+v", got[0])
	}
	if len(got[1].Issues) != 2 || got[1].Issues[0].Hint == "" {
		t.Fatalf("staging: %+v", got[1])
	}
}

func TestEnvironmentStates_Given_secrets_that_could_not_be_read_When_listed_Then_they_are_not_judged(t *testing.T) {
	envs := []model.Environment{{Meta: model.Meta{Name: "a"}, File: "/e/a.yaml", Vars: []model.EnvVar{{Name: "t", Secret: true}}}}

	got := vm.EnvironmentStates(envs, nil, "", false)

	if len(got[0].Issues) != 0 {
		t.Fatalf("%+v", got[0])
	}
}
