// SPDX-License-Identifier: Apache-2.0

// Package vault stores secret values in an encrypted file in the
// workspace (vault.enc), so collections and environment files only ever
// hold references.
//
// The file is AES-256-GCM encrypted. Its key is a random data key kept in
// the OS keychain or, where there is no keychain (headless Linux, CI), a
// key derived with Argon2id from FLOWI_VAULT_KEY or a passphrase. The
// source is recorded in the file, so a vault is always reopened the way
// it was created.
//
// FLOWI_SECRET_<ENV>_<NAME> environment variables override vault entries
// without unlocking it. Masker hides secret values in anything shown or
// saved.
package vault
