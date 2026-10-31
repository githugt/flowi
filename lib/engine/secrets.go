// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"sync"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

// secretTracker reads secrets for a run, remembers every value read (for
// masking) and every declared secret that has no value (reported all at
// once, so the user can set them in one go).
//
// It is safe for concurrent use: mu guards masker, missing and lockErr
// (nodes in different lanes read secrets at once). Lookup holds mu while
// asking the vault, so a passphrase prompt is never shown twice at once.
type secretTracker struct {
	vault   *vault.Vault
	mu      sync.Mutex
	masker  vault.Masker
	missing []missingSecret
	environ []string
	// lockErr is why the vault could not be unlocked. Once set, the vault
	// is not asked again during the run: a wrong passphrase would otherwise
	// be asked for on every render.
	lockErr error
}

type missingSecret struct{ env, name string }

// namespace is the secret.* scope for one environment.
func (t *secretTracker) namespace(collection model.ID, env environment) secretNamespace {
	return secretNamespace{t: t, collection: collection, env: env}
}

type secretNamespace struct {
	t          *secretTracker
	collection model.ID
	env        environment
}

// scope is where this environment's secrets are stored: the workspace for
// a global environment, the collection for a legacy collection one.
func (n secretNamespace) scope() model.ID {
	if n.env.scope != "" {
		return n.env.scope
	}
	return n.collection
}

// Lookup implements template.Namespace. Names that are not declared
// secrets are simply undefined.
func (n secretNamespace) Lookup(name string) (any, bool) {
	if !n.env.secrets[name] {
		return nil, false
	}
	n.t.mu.Lock()
	defer n.t.mu.Unlock()
	if n.t.lockErr != nil {
		if value, ok := vault.Overridden(n.t.environ, n.env.name, name); ok {
			n.t.masker = n.t.masker.With(value)
			return value, true
		}
		return nil, false
	}
	value, ok, err := n.t.vault.Get(vault.Entry{Collection: n.scope(), Environment: n.env.id, Name: name}, n.env.name)
	if err != nil {
		n.t.lockErr = err
		return nil, false
	}
	if !ok {
		n.t.addMissingLocked(n.env.name, name)
		return nil, false
	}
	n.t.masker = n.t.masker.With(value)
	return value, true
}

// addMissingLocked records an unset secret; callers hold mu.
func (t *secretTracker) addMissingLocked(env, name string) {
	for _, m := range t.missing {
		if m == (missingSecret{env, name}) {
			return
		}
	}
	t.missing = append(t.missing, missingSecret{env, name})
}

// isMissing reports declared secrets that have no value, or that cannot
// be read because the vault is locked (reported as FLW-SEC-002, not as
// undefined variables).
func (t *secretTracker) isMissing(env string) func(string) bool {
	return func(name string) bool {
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.lockErr != nil {
			return true
		}
		for _, m := range t.missing {
			if m == (missingSecret{env, name}) {
				return true
			}
		}
		return false
	}
}

// problems reports the secrets a render needed but could not read:
// FLW-SEC-001 for each unset one, or one FLW-SEC-002 when the vault is
// locked.
func (t *secretTracker) problems(env string, needed []string) diag.List {
	if len(needed) == 0 {
		return nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lockErr != nil {
		// Whether the secrets are set cannot be known: report the one cause.
		return diag.List{diag.New(diag.CodeSecretLocked, diag.Location{}, map[string]string{"detail": lockDetail(t.lockErr)})}
	}
	out := make(diag.List, 0, len(needed))
	for _, name := range needed {
		out = append(out, diag.New(diag.CodeSecretMissing, diag.Location{}, map[string]string{"name": name, "env": env}))
	}
	return out
}

func lockDetail(err error) string {
	var le *vault.LockedError
	if errors.As(err, &le) {
		return le.Reason
	}
	return err.Error()
}

// derived adds a value computed from a secret (a Basic credential, a token
// a hook built) to the masker: every string in it.
// Values a server returned are never secret material themselves, so only
// template results that read secret.* come here.
func (t *secretTracker) derived(v any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.deriveLocked(v)
}

func (t *secretTracker) deriveLocked(v any) {
	switch x := v.(type) {
	case string:
		t.masker = t.masker.With(x)
	case map[string]any:
		for _, e := range x {
			t.deriveLocked(e)
		}
	case []any:
		for _, e := range x {
			t.deriveLocked(e)
		}
	}
	// Numbers and booleans are not registered: masking every "8" because a
	// secret is 8 characters long would hide unrelated output.
}

// String masks every secret read so far in s. The tracker is the run's
// live masker: values read later are masked in everything written later.
func (t *secretTracker) String(s string) string {
	return t.current().String(s)
}

// current is a snapshot of the masker, for code that masks many values.
func (t *secretTracker) current() vault.Masker {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.masker
}
