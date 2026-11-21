// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/serve/tokens"
	"github.com/githugt/flowi/lib/vault"
	"github.com/githugt/flowi/lib/workspace"
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

// Scoped, named trigger tokens. The single token above stays: it has full power and no expiry, and keeps working
// for callers that have it (it is listed as "default"). A named token is limited to the flows and environments it
// was made for and may expire; only its hash is kept, so its value is shown once, when it is made.

// legacyTokenName is how the single token of earlier releases is listed, and named to revoke or rotate it.
const legacyTokenName = "default"

// TriggerTokenInfo describes a trigger token. The secret is never part of it.
type TriggerTokenInfo struct {
	ID      string
	Name    string
	Legacy  bool     // the single full-power token of earlier releases
	Flows   []string // flow patterns it may start; empty for any
	Envs    []string // environments it may use; empty for any
	Created time.Time
	Expires time.Time // zero for never
	Expired bool
}

func (w *Workspace) triggerTokens() *tokens.Store {
	w.tokensOnce.Do(func() {
		w.tokens = tokens.Open(filepath.Join(workspace.Dir(w.opts.Paths, w.opts.Name), "trigger_tokens.json"), w.opts.Now)
	})
	return w.tokens
}

func tokenInfo(t tokens.Token, now time.Time) TriggerTokenInfo {
	return TriggerTokenInfo{ID: t.ID, Name: t.Name, Flows: slices.Clone(t.Scopes.Flows), Envs: slices.Clone(t.Scopes.Envs), Created: t.Created, Expires: t.Expires, Expired: t.Expired(now)}
}

func tokenErr(err error) error {
	if errors.Is(err, tokens.ErrInvalid) || errors.Is(err, tokens.ErrNotFound) || errors.Is(err, tokens.ErrExists) {
		return diagErr(diag.CodeServeToken, map[string]string{"detail": err.Error()})
	}
	return err
}

func (w *Workspace) clock() time.Time {
	if w.opts.Now != nil {
		return w.opts.Now()
	}
	return time.Now()
}

// NewScopedTriggerToken makes a named token limited to the flow patterns and environments given (empty: any) that
// expires after lifetime (0: never) and returns its secret, which is not kept and cannot be shown again. A flow
// pattern is a path such as smoke/* where * matches any characters; a call that names no environment is refused for
// a token limited to some.
func (w *Workspace) NewScopedTriggerToken(name string, flows, envs []string, lifetime time.Duration) (string, TriggerTokenInfo, error) {
	if name == legacyTokenName {
		return "", TriggerTokenInfo{}, diagErr(diag.CodeServeToken, map[string]string{"detail": `"default" names the single token of earlier releases; pick another name`})
	}
	t, secret, err := w.triggerTokens().New(name, tokens.Scopes{Flows: flows, Envs: envs}, lifetime)
	if err != nil {
		return "", TriggerTokenInfo{}, tokenErr(err)
	}
	return secret, tokenInfo(t, w.clock()), nil
}

// TriggerTokens lists every trigger token, the single full-power one of earlier releases first when it exists.
func (w *Workspace) TriggerTokens() ([]TriggerTokenInfo, error) {
	var out []TriggerTokenInfo
	if has, err := w.HasTriggerToken(); err != nil {
		return nil, err
	} else if has {
		out = append(out, TriggerTokenInfo{Name: legacyTokenName, Legacy: true})
	}
	list, err := w.triggerTokens().List()
	if err != nil {
		return nil, err
	}
	now := w.clock()
	for _, t := range list {
		out = append(out, tokenInfo(t, now))
	}
	return out, nil
}

// RevokeTriggerToken removes a token by name or id; it stops working at once, also in a running `flowi serve`.
// "default" removes the single token of earlier releases (unless FLOWI_SECRET_SERVE_TRIGGER_TOKEN supplies it).
func (w *Workspace) RevokeTriggerToken(ref string) error {
	if ref == legacyTokenName {
		if has, err := w.HasTriggerToken(); err != nil {
			return err
		} else if !has {
			return diagErr(diag.CodeServeToken, map[string]string{"detail": "there is no default token"})
		}
		return vaultErr(w.secrets().Unset(w.triggerTokenEntry()))
	}
	return tokenErr(w.triggerTokens().Revoke(ref))
}

// RotateTriggerToken gives the token ref (a name or id) a new secret, keeping its scopes and starting its lifetime
// again, and returns the secret. The old secret stops working at once. "default" replaces the single token.
func (w *Workspace) RotateTriggerToken(ref string) (string, TriggerTokenInfo, error) {
	if ref == legacyTokenName {
		secret, err := w.NewTriggerToken()
		return secret, TriggerTokenInfo{Name: legacyTokenName, Legacy: true}, err
	}
	t, secret, err := w.triggerTokens().Rotate(ref)
	if err != nil {
		return "", TriggerTokenInfo{}, tokenErr(err)
	}
	return secret, tokenInfo(t, w.clock()), nil
}

// anyTriggerToken says whether triggers have a token to be called with: the single one or a named one.
func (w *Workspace) anyTriggerToken() (bool, error) {
	if has, err := w.HasTriggerToken(); err != nil || has {
		return has, err
	}
	list, err := w.triggerTokens().List()
	return len(list) > 0, err
}
