// SPDX-License-Identifier: Apache-2.0

//go:build keychain

package vault

import (
	"testing"
	"time"

	"github.com/zalando/go-keyring"
)

// TestOSKeyStore_smoke talks to the real OS keychain. Run it by hand:
//
//	go test -tags keychain -run OSKeyStore ./vault/
func TestOSKeyStore_smoke(t *testing.T) {
	keys := OSKeyStore()
	account := "smoke-test-" + time.Now().Format("150405.000000")

	if err := keys.Set(keychainService, account, "value"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	t.Cleanup(func() {
		if err := keyring.Delete(keychainService, account); err != nil {
			t.Errorf("removing test entry %s/%s: %v", keychainService, account, err)
		}
	})
	got, err := keys.Get(keychainService, account)
	_, missErr := keys.Get(keychainService, account+"-missing")

	if err != nil || got != "value" {
		t.Fatalf("Get: %q %v", got, err)
	}
	if missErr != ErrKeyNotFound { //nolint:errorlint // OSKeyStore returns the sentinel itself
		t.Fatalf("missing entry: %v", missErr)
	}
}
