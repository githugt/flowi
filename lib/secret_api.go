// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"slices"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

// SecretInfo describes one secret variable; it never carries the value.
type SecretInfo struct {
	Environment string
	Name        string
	Set         bool // stored in the vault
	Override    bool // FLOWI_SECRET_<ENV>_<NAME> is set
}

// Secrets lists every secret variable declared in the workspace's
// environments. Environments are global, so a secret belongs to no
// collection.
func (w *Workspace) Secrets() ([]SecretInfo, error) {
	stored, err := w.secrets().Names(vault.WorkspaceScope)
	if err != nil {
		return nil, vaultErr(err)
	}
	var out []SecretInfo
	for _, e := range w.Environments() {
		for _, v := range e.Vars {
			if !v.Secret {
				continue
			}
			entry := vault.Entry{Collection: vault.WorkspaceScope, Environment: e.ID, Name: v.Name}
			out = append(out, SecretInfo{
				Environment: e.Name, Name: v.Name,
				Set: slices.Contains(stored, entry), Override: overridden(w.opts.Environ, e.Name, v.Name),
			})
		}
	}
	return out, nil
}

func overridden(environ []string, env, name string) bool {
	_, ok := vault.Overridden(environ, env, name)
	return ok
}

// CheckSecret reports whether a secret variable can be set, before its
// value is asked for: the environment and the declaration must exist.
func (w *Workspace) CheckSecret(env, name string) error {
	_, err := w.secretEntry(env, name)
	return err
}

// SetSecret stores a secret's value in the vault. The variable must be
// declared secret in the environment ({ secret: true }).
func (w *Workspace) SetSecret(env, name, value string) error {
	entry, err := w.secretEntry(env, name)
	if err != nil {
		return err
	}
	return vaultErr(w.secrets().Set(entry, value))
}

// UnsetSecret removes a secret's value from the vault.
func (w *Workspace) UnsetSecret(env, name string) error {
	entry, err := w.secretEntry(env, name)
	if err != nil {
		return err
	}
	return vaultErr(w.secrets().Unset(entry))
}

// DiscardSecret removes a stored value whose variable is no longer a secret of the environment:
// it was deleted, or turned into a plain variable. UnsetSecret cannot do that, because it requires the
// declaration. Without this the value would stay in the vault with nothing left to show or remove it.
// The environment must exist (FLW-REQ-004); a name with no stored value is not an error.
func (w *Workspace) DiscardSecret(env, name string) error {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == env })
	if i < 0 {
		return diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": env})
	}
	return vaultErr(w.secrets().Unset(vault.Entry{Collection: vault.WorkspaceScope, Environment: envs[i].ID, Name: name}))
}

// RevealSecret returns the stored value of one secret variable, for the person who owns it to read
// again. It is the only call that returns a value: Secrets lists none. A secret provided by a
// FLOWI_SECRET_<ENV>_<NAME> variable is not revealed (FLW-SEC-004): it is not stored here, and the
// variable is already in the reader's environment. A secret with no value is FLW-SEC-001.
// Callers decide when to show it and for how long; this does not log.
func (w *Workspace) RevealSecret(env, name string) (string, error) {
	entry, err := w.secretEntry(env, name)
	if err != nil {
		return "", err
	}
	args := map[string]string{"name": name, "env": env}
	if overridden(w.opts.Environ, env, name) {
		return "", diagErr(diag.CodeSecretFromEnvironment, args)
	}
	value, ok, err := w.secrets().Get(entry, env)
	if err != nil {
		return "", vaultErr(err)
	}
	if !ok {
		return "", diagErr(diag.CodeSecretMissing, args)
	}
	return value, nil
}

// vaultErr reports a locked vault as a user problem (FLW-SEC-002): the fix
// is FLOWI_VAULT_KEY or the right passphrase, not a system repair.
func vaultErr(err error) error {
	var locked *vault.LockedError
	if errors.As(err, &locked) {
		return diagErr(diag.CodeSecretLocked, map[string]string{"detail": locked.Reason})
	}
	return err
}

// secretEntry resolves a declared secret variable to its vault entry.
func (w *Workspace) secretEntry(env, name string) (vault.Entry, error) {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == env })
	if i < 0 {
		return vault.Entry{}, diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": env})
	}
	e := envs[i]
	if !slices.ContainsFunc(e.Vars, func(v model.EnvVar) bool { return v.Name == name && v.Secret }) {
		return vault.Entry{}, diagErr(diag.CodeSecretUndeclared, map[string]string{"name": name, "env": env})
	}
	return vault.Entry{Collection: vault.WorkspaceScope, Environment: e.ID, Name: name}, nil
}
