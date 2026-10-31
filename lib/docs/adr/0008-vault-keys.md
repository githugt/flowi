# 0008: Where the vault key comes from

- **Status:** Accepted
- **Date:** 2026-10-06

## Context

Secret values live only in the workspace's `vault.enc` (AES-256-GCM). The key must be
available on a developer's machine without typing a passphrase each time, and on CI or
headless Linux where there is no OS keychain.

## Decision

- **New vault:** with `FLOWI_VAULT_KEY` set, the key is derived from it with Argon2id
  (time 3, 64 MiB, 4 threads, 16-byte salt kept in the file). Otherwise a random 32-byte
  key is stored in the OS keychain (service `flowi`, account `vault:<workspace-id>`).
  Without a keychain, the user is asked for a passphrase (Argon2id as above); without a
  terminal either, the command fails with `FLW-SEC-002` naming `FLOWI_VAULT_KEY`.
- **Existing vault:** the file records its key source, and it is always reopened that
  way. A keychain vault whose key has gone is an error, never a silent new vault.
- The workspace id is bound into the encryption as additional data, so a copied vault
  does not open in another workspace.
- Entries are keyed by collection id, **environment id** and name, so renaming an
  environment file keeps its secrets.
- `FLOWI_SECRET_<ENV>_<NAME>` overrides an entry without unlocking the vault (CI). An
  empty value counts as unset.
- Keychain calls time out after 5 seconds.

## Consequences

- CI needs only `FLOWI_SECRET_*` variables, or `FLOWI_VAULT_KEY` plus a vault.
- A vault is tied to one machine's keychain unless it was created with a passphrase.
