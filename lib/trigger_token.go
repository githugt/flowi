// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"slices"

	"github.com/githugt/flowi-lib/vault"
)

// The trigger token is a workspace-level secret kept in the vault like any
// other: an entry under the workspace's own id (no collection has it), in
// the reserved environment "serve". FLOWI_SECRET_SERVE_TRIGGER_TOKEN
// overrides the vault, which suits containers.
const (
	triggerTokenEnv  = "serve"
	triggerTokenName = "trigger_token"
	triggerTokenSize = 32 // random bytes
)

func (w *Workspace) triggerTokenEntry() vault.Entry {
	return vault.Entry{Collection: w.Model().ID, Environment: triggerTokenEnv, Name: triggerTokenName}
}

// TriggerToken returns the bearer token that authorises HTTP flow triggers.
// ok is false when none is set, in which case triggers stay off.
func (w *Workspace) TriggerToken() (token string, ok bool, err error) {
	token, ok, err = w.secrets().Get(w.triggerTokenEntry(), triggerTokenEnv)
	return token, ok, vaultErr(err)
}

// HasTriggerToken reports whether a trigger token exists, in the vault or as
// an environment override, without reading its value.
func (w *Workspace) HasTriggerToken() (bool, error) {
	if _, ok := vault.Overridden(w.opts.Environ, triggerTokenEnv, triggerTokenName); ok {
		return true, nil
	}
	entry := w.triggerTokenEntry()
	names, err := w.secrets().Names(entry.Collection)
	if err != nil {
		return false, vaultErr(err)
	}
	return slices.Contains(names, entry), nil
}

// NewTriggerToken stores a fresh random token, replacing any earlier one,
// and returns it. This is the only time the value is shown to a user.
func (w *Workspace) NewTriggerToken() (string, error) {
	raw := make([]byte, triggerTokenSize)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generating a token: %w", err)
	}
	token := hex.EncodeToString(raw)
	if err := w.secrets().Set(w.triggerTokenEntry(), token); err != nil {
		return "", vaultErr(err)
	}
	return token, nil
}
