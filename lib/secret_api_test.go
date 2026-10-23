// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
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
