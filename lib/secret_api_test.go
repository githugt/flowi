// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

func TestSecrets_should_list_declared_secrets_with_set_and_override_state(t *testing.T) {
	w := demoWorkspace(t)
	w.opts.Environ = []string{"FLOWI_SECRET_STAGING_PASSWORD=x"}

	got, err := w.Secrets()

	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v %v", got, err)
	}
	byEnv := map[string]SecretInfo{}
	for _, s := range got {
		byEnv[s.Environment] = s
	}
	if !byEnv["local"].Set || byEnv["local"].Override || byEnv["staging"].Set || !byEnv["staging"].Override || byEnv["local"].Name != "password" {
		t.Fatalf("got %+v", got)
	}
}

func TestSetSecret_Given_undeclared_name_Then_FLW_SEC_003(t *testing.T) {
	w := demoWorkspace(t)

	err := w.SetSecret("local", "base_url", "x")

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeSecretUndeclared {
		t.Fatalf("got %v", err)
	}
}

func TestUnsetSecret_should_mark_secret_missing(t *testing.T) {
	w := demoWorkspace(t)

	err := w.UnsetSecret("local", "password")
	got, _ := w.Secrets()

	for _, s := range got {
		if s.Environment == "local" && s.Set {
			t.Fatalf("still set: %+v", s)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetSecret_Given_no_keychain_no_passphrase_no_FLOWI_VAULT_KEY_Then_FLW_SEC_002(t *testing.T) {
	e := newEnv(t)
	dir := copyDir(t, filepath.Join("examples", "user-creation"))
	moveEnvironmentsToWorkspace(t, e, dir)
	w := e.open(t)
	mustMount(t, w, dir)

	err := w.SetSecret("local", "password", "x")

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeSecretLocked || !strings.Contains(le.List[0].Args["detail"], "FLOWI_VAULT_KEY") {
		t.Fatalf("got %v", err)
	}
}

func TestCheckSecret_should_accept_declared_and_reject_unknown_env(t *testing.T) {
	w := demoWorkspace(t)

	ok := w.CheckSecret("local", "password")
	bad := w.CheckSecret("prod", "password")

	var le *diag.ListError
	if ok != nil || !errors.As(bad, &le) || le.List[0].Code != diag.CodeRequestEnvUnknown {
		t.Fatalf("ok %v bad %v", ok, bad)
	}
}

func TestRevealSecret_should_return_the_stored_value_of_that_secret_only(t *testing.T) {
	w := demoWorkspace(t)

	got, err := w.RevealSecret("local", "password")

	if err != nil || got != "demo" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestRevealSecret_Given_the_secret_is_not_set_Then_FLW_SEC_001(t *testing.T) {
	w := demoWorkspace(t)
	if err := w.UnsetSecret("local", "password"); err != nil {
		t.Fatal(err)
	}

	_, err := w.RevealSecret("local", "password")

	requireCode(t, err, diag.CodeSecretMissing)
}

func TestRevealSecret_Given_an_undeclared_name_Then_FLW_SEC_003(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.RevealSecret("local", "base_url")

	requireCode(t, err, diag.CodeSecretUndeclared)
}

func TestRevealSecret_Given_an_environment_variable_provides_it_Then_it_is_not_revealed(t *testing.T) {
	w := demoWorkspace(t)
	w.opts.Environ = []string{"FLOWI_SECRET_LOCAL_PASSWORD=from-env"}

	got, err := w.RevealSecret("local", "password")

	requireCode(t, err, diag.CodeSecretFromEnvironment)
	if got != "" {
		t.Fatalf("value leaked: %q", got)
	}
}

func requireCode(t *testing.T, err error, want diag.Code) {
	t.Helper()
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}

func TestDiscardSecret_should_remove_the_value_of_a_variable_no_longer_declared_secret(t *testing.T) {
	w := demoWorkspace(t)
	env := w.Environments()[slices.IndexFunc(w.Environments(), func(e model.Environment) bool { return e.Name == "local" })]
	env.Vars = slices.DeleteFunc(slices.Clone(env.Vars), func(v model.EnvVar) bool { return v.Name == "password" })
	if err := w.SaveEnvironment(env, SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}
	unsetErr := w.UnsetSecret("local", "password")

	err := w.DiscardSecret("local", "password")
	names, namesErr := w.secrets().Names(vault.WorkspaceScope)

	requireCode(t, unsetErr, diag.CodeSecretUndeclared) // why UnsetSecret cannot do this once the declaration is gone
	if err != nil || namesErr != nil || len(names) != 0 {
		t.Fatalf("err %v %v, vault still holds %v", err, namesErr, names)
	}
}

func TestDiscardSecret_Given_an_unknown_environment_Then_FLW_REQ_004(t *testing.T) {
	w := demoWorkspace(t)

	err := w.DiscardSecret("nowhere", "password")

	requireCode(t, err, diag.CodeRequestEnvUnknown)
}
