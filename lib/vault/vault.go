// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/githugt/flowi-lib/model"
)

// Entry names one secret: a variable of an environment of a collection.
// Environments are identified by id, so renaming an environment file
// keeps its secrets.
type Entry struct {
	Collection  model.ID
	Environment model.ID
	Name        string
}

// WorkspaceScope is the Entry.Collection of secrets that belong to a global
// environment. Environments are workspace-wide, so their secrets are not
// tied to any collection. It is not a valid collection id.
const WorkspaceScope model.ID = "workspace"

// Options configures a vault.
type Options struct {
	File        string // workspaces/<name>/vault.enc
	WorkspaceID model.ID
	Keys        KeyStore               // nil when there is no keychain
	Environ     []string               // for FLOWI_VAULT_KEY and FLOWI_SECRET_*
	Passphrase  func() (string, error) // nil when no prompt is possible
}

// Vault is the encrypted secrets file. It is unlocked on first use and the
// key kept for the vault's lifetime. Safe for concurrent use.
type Vault struct {
	opts Options

	mu  sync.Mutex // guards key and env
	key []byte
	env *envelope // nil until the file has been read or created
}

// New returns a vault; nothing is read or unlocked until it is used.
func New(o Options) *Vault { return &Vault{opts: o} }

// Get returns a secret's value. FLOWI_SECRET_<ENV>_<NAME> wins over the
// vault and needs no unlock; envName is the environment's name, as users
// write it in the variable. ok is false when the secret is not set.
func (v *Vault) Get(e Entry, envName string) (value string, ok bool, err error) {
	if value, ok := Overridden(v.opts.Environ, envName, e.Name); ok {
		return value, true, nil
	}
	v.mu.Lock()
	defer v.mu.Unlock()
	c, exists, err := v.read()
	if err != nil || !exists {
		return "", false, err
	}
	i := c.index(e)
	if i < 0 {
		return "", false, nil
	}
	return c.Entries[i].Value, true, nil
}

// Unlock opens the vault now (asking for the passphrase if it needs one),
// so callers can do it before showing live output. Without a vault file
// there is nothing to unlock.
func (v *Vault) Unlock() error {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, _, err := v.read()
	return err
}

// Set stores a secret, creating the vault on first use.
func (v *Vault) Set(e Entry, value string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.update(func(c *contents) {
		stored := storedEntry{Collection: e.Collection, Environment: e.Environment, Name: e.Name, Value: value}
		if i := c.index(e); i >= 0 {
			c.Entries[i] = stored
			return
		}
		c.Entries = append(c.Entries, stored)
	})
}

// Unset removes a secret. Removing one that is not set is not an error.
func (v *Vault) Unset(e Entry) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	_, exists, err := v.read()
	if err != nil || !exists {
		return err
	}
	return v.update(func(c *contents) {
		if i := c.index(e); i >= 0 {
			c.Entries = slices.Delete(c.Entries, i, i+1)
		}
	})
}

// Names lists the secrets set for a collection, never their values.
func (v *Vault) Names(collection model.ID) ([]Entry, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	c, exists, err := v.read()
	if err != nil || !exists {
		return nil, err
	}
	var out []Entry
	for _, s := range c.Entries {
		if s.Collection == collection {
			out = append(out, Entry{Collection: s.Collection, Environment: s.Environment, Name: s.Name})
		}
	}
	return out, nil
}

// read decrypts the vault file, unlocking it if needed. exists is false
// when there is no vault file yet. Callers hold mu.
func (v *Vault) read() (contents, bool, error) {
	if v.env == nil {
		env, err := readEnvelope(v.opts.File)
		if err != nil || env == nil {
			return contents{}, false, err
		}
		v.env = env
	}
	if v.key == nil {
		key, err := v.existingKey(v.env)
		if err != nil {
			v.env = nil
			return contents{}, false, err
		}
		v.key = key
	}
	c, err := open(v.key, v.opts.WorkspaceID, v.env)
	if errors.Is(err, errWrongKey) {
		v.key, v.env = nil, nil
		return contents{}, false, &LockedError{Reason: err.Error()}
	}
	return c, err == nil, err
}

// update reads (or creates) the vault, applies change and writes it back
// with a fresh nonce. Callers hold mu.
func (v *Vault) update(change func(*contents)) error {
	c, exists, err := v.read()
	if err != nil {
		return err
	}
	if !exists {
		if err := v.create(); err != nil {
			return err
		}
	}
	change(&c)
	nonce, data, err := seal(v.key, v.opts.WorkspaceID, c)
	if err != nil {
		return err
	}
	next := *v.env
	next.Nonce, next.Data = nonce, data
	if err := writeEnvelope(v.opts.File, next); err != nil {
		return fmt.Errorf("saving vault: %w", err)
	}
	v.env = &next
	return nil
}

func (v *Vault) create() error {
	key, env, err := v.createKey()
	if err != nil {
		return err
	}
	v.key, v.env = key, &env
	return nil
}

func (c contents) index(e Entry) int {
	return slices.IndexFunc(c.Entries, func(s storedEntry) bool {
		return s.Collection == e.Collection && s.Environment == e.Environment && s.Name == e.Name
	})
}
