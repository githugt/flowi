// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"sort"
	"time"
)

// StoredToken is an OAuth token as a TokenStore keeps it.
type StoredToken struct {
	Label   string
	Access  string
	Refresh string
	Expires time.Time
}

// TokenStore keeps the tokens a person approved between runs of the program. Implementations encrypt them; a
// failure to read is the same as no token, and a failure to write only means the person is asked again.
type TokenStore interface {
	Load(key string) (StoredToken, bool)
	Save(key string, t StoredToken) error
	Forget(key string) error
	// Keys lists what is stored, without reading any token.
	Keys() ([]string, error)
}

// TokenInfo describes a cached token for a person to look at: never the token itself.
type TokenInfo struct {
	Key        string // the cache key: a hash of what the token is for, enough to tell two apart
	Label      string
	Expires    time.Time
	HasRefresh bool
	Stored     bool // kept across restarts
}

// List says what the cache holds, in memory and in the store, ordered by label and then key.
func (c *TokenCache) List() []TokenInfo {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	store := c.store
	seen := map[string]TokenInfo{}
	for k, t := range c.tokens {
		seen[k] = TokenInfo{Key: k, Label: t.label, Expires: t.expires, HasRefresh: t.refresh != "", Stored: store != nil && t.persist}
	}
	c.mu.Unlock()
	if store != nil {
		keys, _ := store.Keys()
		for _, k := range keys {
			if _, ok := seen[k]; ok {
				continue
			}
			if t, ok := store.Load(k); ok {
				seen[k] = TokenInfo{Key: k, Label: t.Label, Expires: t.Expires, HasRefresh: t.Refresh != "", Stored: true}
			}
		}
	}
	out := make([]TokenInfo, 0, len(seen))
	for _, v := range seen {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Label != out[j].Label {
			return out[i].Label < out[j].Label
		}
		return out[i].Key < out[j].Key
	})
	return out
}

// Forget drops a token from memory and from the store, so the next send asks for a new one. A key that is not
// there is not an error.
func (c *TokenCache) Forget(key string) error {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	delete(c.tokens, key)
	store := c.store
	c.mu.Unlock()
	if store == nil {
		return nil
	}
	return store.Forget(key)
}
