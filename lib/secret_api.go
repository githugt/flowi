// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
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

// DeclareSecret makes name a secret variable of environment env (when it is not one yet, the environment file
// gains the declaration) and stores value for it in the vault. A name that is a plain variable there is refused
// (FLW-SEC-003 would be misleading): turn it into a secret in the environment first.
func (w *Workspace) DeclareSecret(env, name, value string) error {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.Name == env })
	if i < 0 {
		return w.envNotInWorkspace(env)
	}
	e := envs[i]
	switch j := slices.IndexFunc(e.Vars, func(v model.EnvVar) bool { return v.Name == name }); {
	case j < 0:
		e.Vars = append(slices.Clone(e.Vars), model.EnvVar{Name: name, Secret: true})
		if err := w.SaveEnvironment(e, SaveOptions{}); err != nil {
			return err
		}
	case !e.Vars[j].Secret:
		return fmt.Errorf("%q is a plain variable of the environment %q: pick another name, or make it a secret in the environment", name, env)
	}
	return w.SetSecret(env, name, value)
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
		return w.envNotInWorkspace(env)
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
		return vault.Entry{}, w.envNotInWorkspace(env)
	}
	e := envs[i]
	if !slices.ContainsFunc(e.Vars, func(v model.EnvVar) bool { return v.Name == name && v.Secret }) {
		return vault.Entry{}, diagErr(diag.CodeSecretUndeclared, map[string]string{"name": name, "env": env})
	}
	return vault.Entry{Collection: vault.WorkspaceScope, Environment: e.ID, Name: name}, nil
}

// OrphanSecret is a value in the vault that no environment declares as a secret any more: its environment was deleted,
// or the variable was removed or made plain. It carries no value.
type OrphanSecret struct {
	// Environment is the environment's name, or its id when the environment itself is gone.
	Environment string
	Name        string
	// EnvironmentGone says the whole environment is gone, not only the variable.
	EnvironmentGone bool
}

// declaresSecret says whether some environment still declares the entry as a secret variable.
func declaresSecret(envs []model.Environment, e vault.Entry) bool {
	return slices.ContainsFunc(envs, func(env model.Environment) bool {
		return env.ID == e.Environment && slices.ContainsFunc(env.Vars, func(v model.EnvVar) bool { return v.Name == e.Name && v.Secret })
	})
}

// keepSecret is the rule of Prune: secrets of other scopes are not judged here, and a workspace secret stays while its
// environment declares it.
func keepSecret(envs []model.Environment) func(vault.Entry) bool {
	return func(e vault.Entry) bool { return e.Collection != vault.WorkspaceScope || declaresSecret(envs, e) }
}

func orphanOf(envs []model.Environment, e vault.Entry) OrphanSecret {
	if i := slices.IndexFunc(envs, func(x model.Environment) bool { return x.ID == e.Environment }); i >= 0 {
		return OrphanSecret{Environment: envs[i].Name, Name: e.Name}
	}
	return OrphanSecret{Environment: string(e.Environment), Name: e.Name, EnvironmentGone: true}
}

// OrphanSecrets lists the stored secrets nothing declares any more, so they can be looked at before PruneSecrets
// removes them. Only the workspace's own secrets are judged.
func (w *Workspace) OrphanSecrets() ([]OrphanSecret, error) {
	stored, err := w.secrets().Stored()
	if err != nil {
		return nil, vaultErr(err)
	}
	envs, keep := w.Environments(), keepSecret(w.Environments())
	var out []OrphanSecret
	for _, e := range stored {
		if !keep(e) {
			out = append(out, orphanOf(envs, e))
		}
	}
	return out, nil
}

// PruneSecrets removes the stored secrets nothing declares any more and returns what went. It never touches a secret
// an environment still declares.
func (w *Workspace) PruneSecrets() ([]OrphanSecret, error) {
	envs := w.Environments()
	gone, err := w.secrets().Prune(keepSecret(envs))
	if err != nil {
		return nil, vaultErr(err)
	}
	out := make([]OrphanSecret, 0, len(gone))
	for _, e := range gone {
		out = append(out, orphanOf(envs, e))
	}
	return out, nil
}

// EnvImpact is what deleting an environment takes with it or leaves hanging.
type EnvImpact struct {
	// Secrets are the names of the secret variables the environment declares: their stored values go with it.
	Secrets []string
	// Dependents are the environments that build on it (extends); they would lose what it gives them.
	Dependents []string
}

// EnvironmentImpact says what deleting the environment file would affect. An unknown file is FLW-REQ-004.
func (w *Workspace) EnvironmentImpact(file string) (EnvImpact, error) {
	envs := w.Environments()
	i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.File == file })
	if i < 0 {
		return EnvImpact{}, diagErr(diag.CodeRequestEnvUnknown, map[string]string{"env": file})
	}
	out := EnvImpact{Secrets: []string{}, Dependents: []string{}}
	for _, v := range envs[i].Vars {
		if v.Secret {
			out.Secrets = append(out.Secrets, v.Name)
		}
	}
	for _, e := range envs {
		if e.Extends != "" && e.Extends == envs[i].Name {
			out.Dependents = append(out.Dependents, e.Name)
		}
	}
	return out, nil
}
