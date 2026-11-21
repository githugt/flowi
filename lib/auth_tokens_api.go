// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/authx"
	"github.com/githugt/flowi/lib/vault"
)

// vaultTokens keeps the OAuth tokens a person approved in the workspace vault, encrypted like a secret, under a scope
// of their own (vault.TokenScope). A vault that cannot be opened reads as no token and writes as an error the cache
// ignores: the person is asked again, which is what happened before tokens were kept.
type vaultTokens struct{ w *Workspace }

func tokenEntry(key string) vault.Entry { return vault.Entry{Collection: vault.TokenScope, Name: key} }

type storedTokenJSON struct {
	Label   string    `json:"label"`
	Access  string    `json:"access"`
	Refresh string    `json:"refresh,omitempty"`
	Expires time.Time `json:"expires"`
}

func (s vaultTokens) Load(key string) (authx.StoredToken, bool) {
	v, ok, err := s.w.secrets().Get(tokenEntry(key), "")
	if err != nil || !ok {
		return authx.StoredToken{}, false
	}
	var t storedTokenJSON
	if json.Unmarshal([]byte(v), &t) != nil || t.Access == "" {
		return authx.StoredToken{}, false
	}
	return authx.StoredToken{Label: t.Label, Access: t.Access, Refresh: t.Refresh, Expires: t.Expires}, true
}

func (s vaultTokens) Save(key string, t authx.StoredToken) error {
	b, err := json.Marshal(storedTokenJSON{Label: t.Label, Access: t.Access, Refresh: t.Refresh, Expires: t.Expires})
	if err != nil {
		return err
	}
	return s.w.secrets().Set(tokenEntry(key), string(b))
}

func (s vaultTokens) Forget(key string) error { return s.w.secrets().Unset(tokenEntry(key)) }

func (s vaultTokens) Keys() ([]string, error) {
	entries, err := s.w.secrets().Names(vault.TokenScope)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name)
	}
	return out, nil
}

// AuthToken describes an OAuth token the workspace holds. It never carries the token.
type AuthToken struct {
	ID         string // the start of the token's key: enough to name it in ForgetAuthToken
	Label      string // what it is for, such as "device_code · auth.example.com · client tv · scope read"
	Expires    time.Time
	Expired    bool
	HasRefresh bool // an expired token with one is renewed without asking the person
	Stored     bool // kept in the vault, so it survives a restart
}

// idLen is how many characters of a token's key name it.
const idLen = 8

// AuthTokens lists the OAuth tokens this workspace holds: the ones in memory since it opened, and the ones a
// person approved earlier and the vault kept. Reading the vault may ask for its passphrase; when it cannot be
// opened only the tokens in memory are listed.
func (w *Workspace) AuthTokens() []AuthToken {
	now := time.Now()
	if w.opts.Now != nil {
		now = w.opts.Now()
	}
	var out []AuthToken
	for _, t := range w.authTokens.List() {
		out = append(out, AuthToken{ID: shortID(t.Key), Label: t.Label, Expires: t.Expires, Expired: !t.Expires.After(now), HasRefresh: t.HasRefresh, Stored: t.Stored})
	}
	return out
}

func shortID(key string) string { return key[:min(idLen, len(key))] }

// ForgetAuthToken drops the token whose ID (or the start of it) is given, from memory and from the vault, so the
// next send that needs it asks again. An id that matches nothing or several tokens is FLW-AUTH-004.
func (w *Workspace) ForgetAuthToken(id string) error {
	if id == "" {
		return diagErr(diag.CodeAuthToken, map[string]string{"detail": "give the id of a token"})
	}
	var match []string
	for _, t := range w.authTokens.List() {
		if strings.HasPrefix(t.Key, id) {
			match = append(match, t.Key)
		}
	}
	switch len(match) {
	case 0:
		return diagErr(diag.CodeAuthToken, map[string]string{"detail": fmt.Sprintf("no saved token has the id %q", id)})
	case 1:
		return w.authTokens.Forget(match[0])
	}
	return diagErr(diag.CodeAuthToken, map[string]string{"detail": fmt.Sprintf("%d saved tokens start with %q", len(match), id)})
}
