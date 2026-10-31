// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	flowi "github.com/githugt/flowi-lib"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func secretsBackend() *vmtest.Backend {
	b := sampleBackend()
	b.SecretList = []flowi.SecretInfo{
		{Environment: "local", Name: "password", Set: true},
		{Environment: "local", Name: "token"},
		{Environment: "prod", Name: "password", Override: true},
	}
	return b
}

func newSecrets(t *testing.T, b *vmtest.Backend) (*vm.Secrets, *vm.Worker) {
	t.Helper()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	s := vm.NewSecrets(b, w, ws)
	s.Load()
	w.Wait()
	return s, w
}

func TestSecrets_Load_should_group_the_secrets_by_collection_and_environment(t *testing.T) {
	s, _ := newSecrets(t, secretsBackend())

	rows := s.Rows()

	// header (collection · local), password, token, header (collection · prod), password
	if len(rows) != 5 || rows[0].Header == "" || rows[1].Info.Name != "password" || rows[2].Info.Name != "token" || rows[3].Header == "" || rows[4].Info.Environment != "prod" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestSecrets_State_should_say_stored_missing_or_overridden(t *testing.T) {
	s, _ := newSecrets(t, secretsBackend())
	rows := s.Rows()

	got := []vm.SecretState{rows[1].State(), rows[2].State(), rows[4].State()}

	if got[0] != vm.SecretStored || got[1] != vm.SecretMissing || got[2] != vm.SecretOverridden {
		t.Fatalf("states %v", got)
	}
}

func TestSecrets_Set_should_store_the_value_and_show_it_as_stored(t *testing.T) {
	b := secretsBackend()
	s, w := newSecrets(t, b)
	got := errors.New("not called")

	s.Set(s.Rows()[2].Info, "hunter2", func(err error) { got = err })
	w.Wait()

	if got != nil || !b.Called("SetSecret local/token") || s.Rows()[2].State() != vm.SecretStored {
		t.Fatalf("err %v, state %v", got, s.Rows()[2].State())
	}
}

func TestSecrets_Set_Given_an_empty_value_Then_refuses(t *testing.T) {
	b := secretsBackend()
	s, w := newSecrets(t, b)
	var got error

	s.Set(s.Rows()[2].Info, "", func(err error) { got = err })
	w.Wait()

	if got == nil || b.Called("SetSecret local/token") {
		t.Fatalf("err %v", got)
	}
}

func TestSecrets_Unset_should_remove_the_stored_value(t *testing.T) {
	b := secretsBackend()
	s, w := newSecrets(t, b)

	s.Unset(s.Rows()[1].Info, func(error) {})
	w.Wait()

	if !b.Called("UnsetSecret local/password") || s.Rows()[1].State() != vm.SecretMissing {
		t.Fatalf("state %v", s.Rows()[1].State())
	}
}

func TestSecrets_Set_Given_the_vault_fails_Then_reports_it(t *testing.T) {
	b := secretsBackend()
	b.SecretErr = errors.New("keychain locked")
	s, w := newSecrets(t, b)
	var got error

	s.Set(s.Rows()[2].Info, "x", func(err error) { got = err })
	w.Wait()

	if got == nil {
		t.Fatal("the failure was not reported")
	}
}

func TestSecrets_Find_should_return_a_secret_by_its_key(t *testing.T) {
	s, _ := newSecrets(t, secretsBackend())
	want := s.Rows()[4].Info

	got, ok := s.Find(vm.SecretKey(want))
	_, none := s.Find("nope")

	if !ok || got != want || none {
		t.Fatalf("found %v %+v, none %v", ok, got, none)
	}
}
