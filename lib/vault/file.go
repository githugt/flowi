// SPDX-License-Identifier: Apache-2.0

package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/githugt/flowi-lib/model"
)

const (
	fileVersion  = 1
	fileMode     = 0o600
	keySize      = 32 // AES-256
	aadPrefix    = "flowi-vault-v1:"
	sourceChain  = "keychain"
	sourcePhrase = "passphrase"
)

// envelope is the on-disk format of vault.enc.
type envelope struct {
	Version   int    `json:"version"`
	KeySource string `json:"key_source"`
	Salt      []byte `json:"salt,omitempty"`
	Nonce     []byte `json:"nonce"`
	Data      []byte `json:"data"`
}

// contents is the decrypted payload.
type contents struct {
	Entries []storedEntry `json:"entries"`
}

type storedEntry struct {
	Collection  model.ID `json:"c"`
	Environment model.ID `json:"e"`
	Name        string   `json:"n"`
	Value       string   `json:"v"`
}

// readEnvelope loads vault.enc; a missing file returns (nil, nil).
func readEnvelope(path string) (*envelope, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading vault: %w", err)
	}
	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("reading vault %s: %w", path, err)
	}
	if env.Version != fileVersion {
		return nil, fmt.Errorf("vault %s has version %d, want %d", path, env.Version, fileVersion)
	}
	return &env, nil
}

// seal encrypts c with a fresh nonce. The workspace id is bound in as
// additional data, so a vault copied to another workspace does not open.
func seal(key []byte, workspace model.ID, c contents) (nonce, data []byte, err error) {
	gcm, err := newGCM(key)
	if err != nil {
		return nil, nil, err
	}
	plain, err := json.Marshal(c)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding vault: %w", err)
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, fmt.Errorf("generating nonce: %w", err)
	}
	return nonce, gcm.Seal(nil, nonce, plain, []byte(aadPrefix+workspace)), nil
}

// errWrongKey means decryption failed: the key or the workspace is wrong.
var errWrongKey = errors.New("wrong key or vault belongs to another workspace")

func open(key []byte, workspace model.ID, env *envelope) (contents, error) {
	gcm, err := newGCM(key)
	if err != nil {
		return contents{}, err
	}
	plain, err := gcm.Open(nil, env.Nonce, env.Data, []byte(aadPrefix+workspace))
	if err != nil {
		return contents{}, errWrongKey
	}
	var c contents
	if err := json.Unmarshal(plain, &c); err != nil {
		return contents{}, fmt.Errorf("decoding vault: %w", err)
	}
	return c, nil
}

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("vault cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("vault cipher: %w", err)
	}
	return gcm, nil
}

// writeEnvelope writes vault.enc atomically with mode 0600.
func writeEnvelope(path string, env envelope) error {
	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("encoding vault: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating vault directory: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".vault-*.tmp")
	if err != nil {
		return fmt.Errorf("writing vault: %w", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // cleanup of a temp file that is gone after a successful rename
	if err := writeAndClose(tmp, data); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("writing vault: %w", err)
	}
	return nil
}

func writeAndClose(f *os.File, data []byte) error {
	if err := f.Chmod(fileMode); err != nil {
		_ = f.Close() // already failing; the chmod error says more
		return fmt.Errorf("writing vault: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close() // already failing; the write error says more
		return fmt.Errorf("writing vault: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close() // already failing; the sync error says more
		return fmt.Errorf("writing vault: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("writing vault: %w", err)
	}
	return nil
}
