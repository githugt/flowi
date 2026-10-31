// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// fakeKeys is an in-memory KeyStore.
type fakeKeys struct {
	mu   sync.Mutex
	keys map[string]string
}

func newFakeKeys() *fakeKeys { return &fakeKeys{keys: map[string]string{}} }

func (f *fakeKeys) Get(service, account string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	v, ok := f.keys[service+"/"+account]
	if !ok {
		return "", ErrKeyNotFound
	}
	return v, nil
}

func (f *fakeKeys) Set(service, account, secret string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys[service+"/"+account] = secret
	return nil
}

// failingKeys fails every call, proving a code path never touches the keychain.
type failingKeys struct{ t *testing.T }

func (f failingKeys) Get(string, string) (string, error) {
	f.t.Error("keychain Get called")
	return "", errors.New("no keychain")
}

func (f failingKeys) Set(string, string, string) error {
	f.t.Error("keychain Set called")
	return errors.New("no keychain")
}

const ws model.ID = "01JA00000000000000000000W1"

func testEntry() Entry {
	return Entry{Collection: "01JA0000000000000000000001", Environment: "01JA0000000000000000000002", Name: "password"}
}

func options(t *testing.T, keys KeyStore) Options {
	t.Helper()
	return Options{File: filepath.Join(t.TempDir(), "vault.enc"), WorkspaceID: ws, Keys: keys}
}

func TestVault_should_round_trip_entries_through_file(t *testing.T) {
	o := options(t, newFakeKeys())
	if err := New(o).Set(testEntry(), "hunter2"); err != nil {
		t.Fatal(err)
	}

	got, ok, err := New(o).Get(testEntry(), "staging")

	if err != nil || !ok || got != "hunter2" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
	raw, _ := os.ReadFile(o.File)
	if bytes.Contains(raw, []byte("hunter2")) {
		t.Fatal("plaintext secret in vault file")
	}
}

func TestVault_Given_no_file_Then_Get_reports_not_set_without_unlocking(t *testing.T) {
	o := options(t, failingKeys{t})

	_, ok, err := New(o).Get(testEntry(), "staging")

	if err != nil || ok {
		t.Fatalf("got %v %v", ok, err)
	}
}

func TestVault_Given_wrong_workspace_id_Then_open_fails(t *testing.T) {
	keys := newFakeKeys()
	o := options(t, keys)
	if err := New(o).Set(testEntry(), "x"); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(t.TempDir(), "vault.enc")
	data, _ := os.ReadFile(o.File)
	if err := os.WriteFile(moved, data, 0o600); err != nil {
		t.Fatal(err)
	}
	// Same keychain testEntry(), different workspace: the additional data no longer matches.
	other := Options{File: moved, WorkspaceID: "01JA00000000000000000000W2", Keys: keys}
	key, _ := keys.Get(keychainService, keychainAccount(ws))
	_ = keys.Set(keychainService, keychainAccount(other.WorkspaceID), key)

	_, _, err := New(other).Get(testEntry(), "staging")

	var le *LockedError
	if !errors.As(err, &le) {
		t.Fatalf("got %v", err)
	}
}

func TestVault_Given_FLOWI_VAULT_KEY_Then_passphrase_source_and_reopens_with_same_key(t *testing.T) {
	o := options(t, failingKeys{t})
	o.Environ = []string{"FLOWI_VAULT_KEY=correct horse"}
	if err := New(o).Set(testEntry(), "s3cret"); err != nil {
		t.Fatal(err)
	}

	got, ok, err := New(o).Get(testEntry(), "staging")
	wrong := o
	wrong.Environ = []string{"FLOWI_VAULT_KEY=battery staple"}
	_, _, wrongErr := New(wrong).Get(testEntry(), "staging")

	if err != nil || !ok || got != "s3cret" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
	if source := fileSource(t, o.File); source != "passphrase" {
		t.Fatalf("key_source %q", source)
	}
	var le *LockedError
	if !errors.As(wrongErr, &le) {
		t.Fatalf("wrong key: %v", wrongErr)
	}
}

func fileSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var f struct {
		KeySource string `json:"key_source"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	return f.KeySource
}

func TestVault_Given_keychain_Then_keychain_source(t *testing.T) {
	o := options(t, newFakeKeys())

	if err := New(o).Set(testEntry(), "x"); err != nil {
		t.Fatal(err)
	}

	if source := fileSource(t, o.File); source != "keychain" {
		t.Fatalf("key_source %q", source)
	}
}

func TestVault_Given_no_keychain_Then_prompts_for_passphrase(t *testing.T) {
	o := options(t, nil)
	prompts := 0
	o.Passphrase = func() (string, error) { prompts++; return "typed", nil }
	if err := New(o).Set(testEntry(), "x"); err != nil {
		t.Fatal(err)
	}

	got, ok, err := New(o).Get(testEntry(), "staging")

	if err != nil || !ok || got != "x" || prompts != 2 {
		t.Fatalf("got %q %v %v prompts=%d", got, ok, err, prompts)
	}
}

func TestVault_Given_no_keychain_and_no_passphrase_Then_LockedError(t *testing.T) {
	err := New(options(t, nil)).Set(testEntry(), "x")

	var le *LockedError
	if !errors.As(err, &le) {
		t.Fatalf("got %v", err)
	}
}

func TestVault_Given_keychain_file_and_missing_keychain_entry_Then_LockedError_not_new_vault(t *testing.T) {
	o := options(t, newFakeKeys())
	if err := New(o).Set(testEntry(), "keep me"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(o.File)
	o.Keys = newFakeKeys() // keychain testEntry() gone

	_, _, getErr := New(o).Get(testEntry(), "staging")
	setErr := New(o).Set(testEntry(), "overwrite")

	var le *LockedError
	if !errors.As(getErr, &le) || !errors.As(setErr, &le) {
		t.Fatalf("get %v, set %v", getErr, setErr)
	}
	after, _ := os.ReadFile(o.File)
	if !bytes.Equal(before, after) {
		t.Fatal("vault file was replaced")
	}
}

func TestVault_Get_Given_FLOWI_SECRET_override_Then_wins_without_unlocking(t *testing.T) {
	o := options(t, failingKeys{t})
	o.Environ = []string{"FLOWI_SECRET_STAGING_EU_PASSWORD=from-env"}

	got, ok, err := New(o).Get(testEntry(), "staging-eu")

	if err != nil || !ok || got != "from-env" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
}

func TestOverrideName_should_upper_case_and_replace_non_alphanumerics(t *testing.T) {
	if got := OverrideName("staging.eu", "api-key"); got != "FLOWI_SECRET_STAGING_EU_API_KEY" {
		t.Fatalf("got %s", got)
	}
}

func TestVault_should_write_mode_0600_and_fresh_nonce_each_write(t *testing.T) {
	o := options(t, newFakeKeys())
	v := New(o)
	if err := v.Set(testEntry(), "a"); err != nil {
		t.Fatal(err)
	}
	first := fileField(t, o.File, "nonce")
	if err := v.Set(testEntry(), "a"); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(o.File)

	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX modes: it reports 0666 whatever was asked for.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
	if fileField(t, o.File, "nonce") == first {
		t.Fatal("nonce reused")
	}
}

func fileField(t *testing.T, path, field string) string {
	t.Helper()
	data, _ := os.ReadFile(path)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatal(err)
	}
	s, _ := m[field].(string)
	return s
}

func TestVault_Unset_should_remove_entry(t *testing.T) {
	o := options(t, newFakeKeys())
	v := New(o)
	if err := v.Set(testEntry(), "a"); err != nil {
		t.Fatal(err)
	}

	err := v.Unset(testEntry())
	_, ok, getErr := New(o).Get(testEntry(), "staging")

	if err != nil || getErr != nil || ok {
		t.Fatalf("unset %v, get ok=%v err=%v", err, ok, getErr)
	}
}

func TestVault_Names_should_not_return_values(t *testing.T) {
	o := options(t, newFakeKeys())
	v := New(o)
	other := testEntry()
	other.Collection = "01JA0000000000000000000009"
	for _, e := range []Entry{testEntry(), other} {
		if err := v.Set(e, "value-"+string(e.Collection)); err != nil {
			t.Fatal(err)
		}
	}

	names, err := New(o).Names(testEntry().Collection)

	if err != nil || len(names) != 1 || names[0] != testEntry() {
		t.Fatalf("got %+v %v", names, err)
	}
	if strings.Contains(stringsOf(names), "value-") {
		t.Fatal("value leaked")
	}
}

func stringsOf(es []Entry) string {
	var b strings.Builder
	for _, e := range es {
		b.WriteString(string(e.Collection) + string(e.Environment) + e.Name)
	}
	return b.String()
}

func TestVault_Get_Given_empty_override_Then_ignored(t *testing.T) {
	o := options(t, newFakeKeys())
	if err := New(o).Set(testEntry(), "from-vault"); err != nil {
		t.Fatal(err)
	}
	o.Environ = []string{"FLOWI_SECRET_STAGING_PASSWORD="}

	got, ok, err := New(o).Get(testEntry(), "staging")

	if err != nil || !ok || got != "from-vault" {
		t.Fatalf("got %q %v %v", got, ok, err)
	}
}

func TestOverridden_should_report_only_non_empty_values(t *testing.T) {
	if _, ok := Overridden([]string{"FLOWI_SECRET_LOCAL_PW="}, "local", "pw"); ok {
		t.Fatal("empty value counted as an override")
	}
	if v, ok := Overridden([]string{"FLOWI_SECRET_LOCAL_PW=x"}, "local", "pw"); !ok || v != "x" {
		t.Fatalf("got %q %v", v, ok)
	}
}
