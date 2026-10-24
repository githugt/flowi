// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newEnvVM(t *testing.T) (*vm.Environment, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := sampleBackend()
	b.SecretList = []flowi.SecretInfo{{Environment: "staging", Name: "password", Set: true}}
	w := vmtest.SyncWorker()
	env := vm.NewEnvironment(b, w, b.Cols[0].Environments, "/c/environments/staging.yaml")
	w.Wait()
	return env, b, w
}

func TestEnvVM_should_list_variables_with_secret_state(t *testing.T) {
	env, _, _ := newEnvVM(t)

	rows := env.Rows()

	if len(rows) != 2 || rows[0] != (vm.EnvVarRow{Name: "base_url", Value: "https://s"}) || !rows[1].Secret || !rows[1].SecretSet || rows[1].Value != "" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestEnvVM_toggle_secret_moves_value_to_vault_and_never_into_the_file(t *testing.T) {
	env, b, w := newEnvVM(t)
	rows := env.Rows()
	rows = append(rows, vm.EnvVarRow{Name: "api_key", Value: "k-123", Secret: true})

	env.SetRows(rows)
	env.Save(false)
	w.Wait()

	saved := b.SavedEnvs[0]
	i := slices.IndexFunc(saved.Vars, func(v model.EnvVar) bool { return v.Name == "api_key" })
	if i < 0 || !saved.Vars[i].Secret || saved.Vars[i].Value != "" || !b.Called("SetSecret staging/api_key") {
		t.Fatalf("saved %+v calls %v", saved.Vars, b.Calls)
	}
	if after := env.Rows(); slices.ContainsFunc(after, func(r vm.EnvVarRow) bool { return r.Value == "k-123" }) || env.Dirty() {
		t.Fatalf("value kept after save: %+v dirty %v", after, env.Dirty())
	}
}

func TestEnvVM_turning_a_secret_off_unsets_the_vault_value(t *testing.T) {
	env, b, w := newEnvVM(t)
	rows := env.Rows()
	rows[1].Secret, rows[1].Value = false, "plain-now"

	env.SetRows(rows)
	env.Save(false)
	w.Wait()

	if !b.Called("DiscardSecret staging/password") || b.SavedEnvs[0].Vars[1].Value != "plain-now" {
		t.Fatalf("calls %v saved %+v", b.Calls, b.SavedEnvs)
	}
}

// UnsetSecret needs the variable to be declared secret, which it no longer is once the file is saved;
// Discard does not, so the value cannot be left behind in the vault.
func TestEnvVM_removing_a_secret_variable_discards_its_vault_value(t *testing.T) {
	env, b, w := newEnvVM(t)
	rows := env.Rows()

	env.SetRows(rows[:1])
	env.Save(false)
	w.Wait()

	if !b.Called("DiscardSecret staging/password") || b.Called("UnsetSecret staging/password") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestEnvVM_save_keeps_existing_secret_when_no_new_value(t *testing.T) {
	env, b, w := newEnvVM(t)

	env.SetRows(env.Rows())
	env.Save(false)
	w.Wait()

	if b.Called("SetSecret staging/password") || b.Called("DiscardSecret staging/password") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestEnvVM_drop_confirm_asks_once_per_save(t *testing.T) {
	env, b, w := newEnvVM(t)
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "x"})}.Err()
	asked := 0
	env.OnDropConfirm(func() { asked++ })

	env.SetRows(env.Rows())
	env.Save(false)
	w.Wait()
	env.SetRows(env.Rows())
	env.DeclineDrop()

	if asked != 1 || env.NeedsDropConfirm() {
		t.Fatalf("asked %d pending %v", asked, env.NeedsDropConfirm())
	}
}

// Before: the worker read e.coll while a reload replaced it on the UI
// goroutine (go test -race reported it).
func TestEnvVM_reload_during_secret_load_is_race_free(t *testing.T) {
	b := sampleBackend()
	b.SecretsGate = make(chan struct{})
	w := vm.NewWorker(func(f func()) { f() }) // a real goroutine: the call blocks on the gate
	env := vm.NewEnvironment(b, w, b.Cols[0].Environments, "/c/environments/staging.yaml")

	env.Reloaded(b.Cols[0].Environments) // the UI goroutine, while Secrets is in flight
	close(b.SecretsGate)
	w.Wait()

	if env.Err() != "" {
		t.Fatalf("err %q", env.Err())
	}
}

func TestEnvVM_Given_save_fails_Then_edits_and_typed_secret_kept(t *testing.T) {
	env, b, w := newEnvVM(t)
	b.SaveErr = errors.New("disk full")
	rows := append(env.Rows(), vm.EnvVarRow{Name: "api_key", Value: "k-1", Secret: true})

	env.SetRows(rows)
	env.Save(false)
	w.Wait()

	if !env.Dirty() || env.Err() == "" || env.Rows()[2].Value != "k-1" {
		t.Fatalf("dirty %v err %q rows %+v", env.Dirty(), env.Err(), env.Rows())
	}
}

func TestEnvVM_Given_a_colour_is_chosen_When_saved_Then_the_file_carries_it(t *testing.T) {
	env, b, w := newEnvVM(t)

	env.SetColour("red")
	dirty := env.Dirty()
	env.Save(false)
	w.Wait()

	if !dirty || env.Colour() != "red" || b.SavedEnvs[0].Colour != "red" {
		t.Fatalf("dirty %v colour %q saved %+v", dirty, env.Colour(), b.SavedEnvs)
	}
}

func TestEnvVM_should_ignore_a_colour_outside_the_palette(t *testing.T) {
	env, _, _ := newEnvVM(t)

	env.SetColour("chartreuse")

	if env.Colour() != "" || env.Dirty() {
		t.Fatalf("colour %q dirty %v", env.Colour(), env.Dirty())
	}
}
