// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"github.com/githugt/flowi-lib/vault"
)

// The Valkey password lives in the vault like the trigger token: an entry
// under the workspace id in the reserved environment "serve".
// FLOWI_SECRET_SERVE_VALKEY_PASSWORD overrides it.
const valkeyPasswordName = "valkey_password"

func (w *Workspace) valkeyPasswordEntry() vault.Entry {
	return vault.Entry{Collection: w.Model().ID, Environment: triggerTokenEnv, Name: valkeyPasswordName}
}

// ValkeyPassword returns the password for serve.valkey_addr; ok is false when
// none is set (a server without authentication).
func (w *Workspace) ValkeyPassword() (password string, ok bool, err error) {
	password, ok, err = w.secrets().Get(w.valkeyPasswordEntry(), triggerTokenEnv)
	return password, ok, vaultErr(err)
}

// SetValkeyPassword stores the password in the vault.
func (w *Workspace) SetValkeyPassword(password string) error {
	return vaultErr(w.secrets().Set(w.valkeyPasswordEntry(), password))
}
