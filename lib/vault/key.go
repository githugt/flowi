// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/crypto/argon2"

	"github.com/githugt/flowi/lib/model"
)

// KeyStore is the OS keychain.
type KeyStore interface {
	Get(service, account string) (string, error)
	Set(service, account, secret string) error
}

// ErrKeyNotFound is returned by KeyStore.Get when there is no such entry.
var ErrKeyNotFound = errors.New("vault key not found")

// errKeychainTimeout means the keychain did not answer in keychainTimeout.
var errKeychainTimeout = errors.New("keychain did not respond")

const (
	keychainService = "flowi"
	keychainTimeout = 5 * time.Second

	saltSize      = 16
	argonTime     = 3
	argonMemoryKB = 64 * 1024
	argonThreads  = 4
)

func keychainAccount(workspace model.ID) string { return "vault:" + string(workspace) }

// LockedError means the vault cannot be opened or created: no key source
// is available, the key is gone, or the passphrase is wrong.
type LockedError struct{ Reason string }

func (e *LockedError) Error() string { return "vault locked: " + e.Reason }

// OSKeyStore returns the operating system keychain (macOS Keychain,
// Secret Service on Linux, Windows Credential Manager).
func OSKeyStore() KeyStore { return osKeyStore{timeout: keychainTimeout} }

type osKeyStore struct{ timeout time.Duration }

func (k osKeyStore) Get(service, account string) (string, error) {
	v, err := withTimeout(k.timeout, func() (string, error) { return keyring.Get(service, account) })
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrKeyNotFound
	}
	return v, err
}

func (k osKeyStore) Set(service, account, secret string) error {
	_, err := withTimeout(k.timeout, func() (string, error) { return "", keyring.Set(service, account, secret) })
	return err
}

// withTimeout runs a keychain call with a deadline. go-keyring has no
// cancellation, so on timeout the call's goroutine is left to finish on
// its own; it holds nothing but its result channel (buffered, so it never
// blocks).
func withTimeout(d time.Duration, call func() (string, error)) (string, error) {
	type result struct {
		v   string
		err error
	}
	done := make(chan result, 1)
	go func() {
		v, err := call()
		done <- result{v, err}
	}()
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.v, r.err
	case <-timer.C:
		return "", fmt.Errorf("%w after %s", errKeychainTimeout, d)
	}
}

// createKey picks a key source for a new vault: FLOWI_VAULT_KEY, then the
// keychain, then a passphrase prompt.
func (v *Vault) createKey() (key []byte, env envelope, err error) {
	if phrase, ok := v.envKey(); ok {
		return newPassphraseKey(phrase)
	}
	if v.opts.Keys != nil {
		if key, err := newKeychainKey(v.opts.Keys, v.opts.WorkspaceID); err == nil {
			return key, envelope{Version: fileVersion, KeySource: sourceChain}, nil
		}
	}
	if v.opts.Passphrase != nil {
		phrase, err := v.opts.Passphrase()
		if err != nil {
			return nil, envelope{}, &LockedError{Reason: "no passphrase: " + err.Error()}
		}
		return newPassphraseKey(phrase)
	}
	return nil, envelope{}, &LockedError{Reason: "no keychain available; set FLOWI_VAULT_KEY or run interactively to enter a passphrase"}
}

func newKeychainKey(keys KeyStore, workspace model.ID) ([]byte, error) {
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generating vault key: %w", err)
	}
	if err := keys.Set(keychainService, keychainAccount(workspace), base64.StdEncoding.EncodeToString(key)); err != nil {
		return nil, fmt.Errorf("storing vault key in keychain: %w", err)
	}
	return key, nil
}

func newPassphraseKey(phrase string) ([]byte, envelope, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, envelope{}, fmt.Errorf("generating salt: %w", err)
	}
	return derive(phrase, salt), envelope{Version: fileVersion, KeySource: sourcePhrase, Salt: salt}, nil
}

func derive(phrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(phrase), salt, argonTime, argonMemoryKB, argonThreads, keySize)
}

// existingKey recovers the key of an existing vault from the source it
// was created with.
func (v *Vault) existingKey(env *envelope) ([]byte, error) {
	if env.KeySource == sourceChain {
		return v.keychainKey()
	}
	if env.KeySource != sourcePhrase {
		return nil, fmt.Errorf("vault has unknown key source %q", env.KeySource)
	}
	phrase, ok := v.envKey()
	if !ok && v.opts.Passphrase != nil {
		p, err := v.opts.Passphrase()
		if err != nil {
			return nil, &LockedError{Reason: "no passphrase: " + err.Error()}
		}
		phrase, ok = p, true
	}
	if !ok {
		return nil, &LockedError{Reason: "the vault needs its passphrase; set FLOWI_VAULT_KEY or run interactively"}
	}
	return derive(phrase, env.Salt), nil
}

func (v *Vault) keychainKey() ([]byte, error) {
	if v.opts.Keys == nil {
		return nil, &LockedError{Reason: "the vault key is in the OS keychain, which is not available"}
	}
	encoded, err := v.opts.Keys.Get(keychainService, keychainAccount(v.opts.WorkspaceID))
	if errors.Is(err, ErrKeyNotFound) {
		return nil, &LockedError{Reason: "the vault key is missing from the OS keychain"}
	}
	if err != nil {
		return nil, &LockedError{Reason: "keychain: " + err.Error()}
	}
	key, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(key) != keySize {
		return nil, &LockedError{Reason: "the keychain entry for the vault is not a valid key"}
	}
	return key, nil
}

func (v *Vault) envKey() (string, bool) {
	phrase := lookup(v.opts.Environ, envVaultKey)
	return phrase, phrase != ""
}
