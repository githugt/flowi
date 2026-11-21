// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

func envNamed(t *testing.T, w *Workspace, name string) model.Environment {
	t.Helper()
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == name })
	if i < 0 {
		t.Fatalf("no environment %q", name)
	}
	return envs[i]
}

func TestDeleteEnvironment_Given_it_declared_secrets_Then_their_values_leave_the_vault(t *testing.T) {
	// Before: deleting an environment left its secret values in the vault, with nothing to show or remove them.
	w := demoWorkspace(t)
	local := envNamed(t, w, "local")

	err := w.DeleteEnvironment(local.File)
	stored, storedErr := w.secrets().Stored()

	if err != nil || storedErr != nil {
		t.Fatalf("err %v %v", err, storedErr)
	}
	for _, e := range stored {
		if e.Environment == local.ID {
			t.Fatalf("the vault still holds %+v", e)
		}
	}
}

func TestDeleteEnvironment_Given_another_environments_secret_Then_it_stays(t *testing.T) {
	w := demoWorkspace(t)
	staging := envNamed(t, w, "staging")
	if err := w.SetSecret("staging", "password", "keep"); err != nil {
		t.Fatal(err)
	}

	err := w.DeleteEnvironment(envNamed(t, w, "local").File)
	got, revealErr := w.RevealSecret("staging", "password")

	if err != nil || revealErr != nil || got != "keep" || staging.ID == "" {
		t.Fatalf("err %v %v, value %q", err, revealErr, got)
	}
}

func TestOrphanSecrets_Given_a_variable_made_plain_or_an_environment_gone_Then_they_are_listed_without_values_and_pruned(t *testing.T) {
	w := demoWorkspace(t)
	local := envNamed(t, w, "local")
	local.Vars = slices.DeleteFunc(slices.Clone(local.Vars), func(v model.EnvVar) bool { return v.Name == "password" })
	if err := w.SaveEnvironment(local, SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}
	if err := w.secrets().Set(vault.Entry{Collection: vault.WorkspaceScope, Environment: "01JZ0000000000000000000009", Name: "token"}, "x"); err != nil {
		t.Fatal(err)
	}
	if err := w.SetSecret("staging", "password", "keep"); err != nil {
		t.Fatal(err)
	}

	orphans, err := w.OrphanSecrets()
	pruned, pruneErr := w.PruneSecrets()
	after, afterErr := w.OrphanSecrets()
	kept, keptErr := w.RevealSecret("staging", "password")

	if err != nil || len(orphans) != 2 {
		t.Fatalf("orphans %+v, err %v", orphans, err)
	}
	byName := map[string]OrphanSecret{}
	for _, o := range orphans {
		byName[o.Name] = o
	}
	if byName["password"].Environment != "local" || byName["password"].EnvironmentGone || !byName["token"].EnvironmentGone {
		t.Errorf("orphans %+v", orphans)
	}
	if pruneErr != nil || len(pruned) != 2 || afterErr != nil || len(after) != 0 {
		t.Errorf("pruned %+v (%v), left %+v (%v)", pruned, pruneErr, after, afterErr)
	}
	if keptErr != nil || kept != "keep" {
		t.Errorf("a declared secret was pruned: %q %v", kept, keptErr)
	}
}

func TestEnvironmentImpact_Given_secrets_and_a_child_Then_they_are_named(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := w.NewEnvironment("child"); err != nil {
		t.Fatal(err)
	}
	child := envNamed(t, w, "child")
	child.Extends = "local"
	if err := w.SaveEnvironment(child, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	got, err := w.EnvironmentImpact(envNamed(t, w, "local").File)
	_, unknown := w.EnvironmentImpact("/nowhere.yaml")

	if err != nil || !slices.Equal(got.Secrets, []string{"password"}) || !slices.Equal(got.Dependents, []string{"child"}) || unknown == nil {
		t.Fatalf("impact %+v, err %v, unknown %v", got, err, unknown)
	}
}
