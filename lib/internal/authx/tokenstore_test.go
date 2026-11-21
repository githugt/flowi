// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// memStore is a TokenStore in a map, counting what is asked of it.
type memStore struct {
	mu     sync.Mutex
	tokens map[string]StoredToken
	saves  int
}

func newMemStore() *memStore { return &memStore{tokens: map[string]StoredToken{}} }

func (m *memStore) Load(k string) (StoredToken, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.tokens[k]
	return t, ok
}

func (m *memStore) Save(k string, t StoredToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tokens[k] = t
	m.saves++
	return nil
}

func (m *memStore) Forget(k string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tokens, k)
	return nil
}

func (m *memStore) Keys() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []string
	for k := range m.tokens {
		out = append(out, k)
	}
	return out, nil
}

func deviceAuth(p *idp) []string {
	return []string{"grant", "device_code", "device_url", p.srv.URL + "/device", "token_url", p.srv.URL + "/token", "client_id", "tv", "client_secret", "s3cr3t", "scope", "read"}
}

func apply(t *testing.T, kv []string, env Env) (string, error) {
	t.Helper()
	s, err := New(auth("oauth2", kv...), env)
	if err != nil {
		t.Fatal(err)
	}
	r := request("GET", "http://api/x")
	_, err = s.Apply(context.Background(), r)
	return r.Header.Get("Authorization"), err
}

// Before: an OAuth token lived in memory, so every restart sent the person back to the browser.
func TestOAuth2_Given_a_persistent_cache_Then_a_token_a_person_approved_survives_a_restart(t *testing.T) {
	p := newIDP(t)
	store := newMemStore()
	prompts := 0
	first := Env{After: immediately, Tokens: NewPersistentTokenCache(store), Prompt: func(Prompt) { prompts++ }}
	_, err := apply(t, deviceAuth(p), first)
	if err != nil || prompts != 1 || store.saves != 1 {
		t.Fatalf("first run: err %v, prompts %d, saves %d", err, prompts, store.saves)
	}
	requests := len(p.tokenRequests())

	again := Env{After: immediately, Tokens: NewPersistentTokenCache(store), Prompt: func(Prompt) { t.Error("the person was asked again after a restart") }}
	header, err := apply(t, deviceAuth(p), again)

	if err != nil || header != "Bearer tok-1" || len(p.tokenRequests()) != requests {
		t.Fatalf("after restart: header %q err %v, token requests %d (was %d)", header, err, len(p.tokenRequests()), requests)
	}
}

func TestOAuth2_Given_a_stored_token_that_ran_out_Then_its_refresh_token_renews_it_and_is_kept_when_the_server_sends_no_new_one(t *testing.T) {
	var mu sync.Mutex
	var forms []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		mu.Lock()
		forms = append(forms, r.PostForm.Get("grant_type")+":"+r.PostForm.Get("refresh_token"))
		mu.Unlock()
		_, _ = w.Write([]byte(`{"access_token":"fresh","expires_in":3600}`))
	}))
	t.Cleanup(srv.Close)
	kv := []string{"grant", "device_code", "device_url", srv.URL + "/device", "token_url", srv.URL + "/token", "client_id", "tv"}
	store := newMemStore()
	key := newCacheKey(t, kv)
	store.tokens[key] = StoredToken{Label: "x", Access: "old", Refresh: "ref-old", Expires: time.Unix(1000, 0)}
	env := Env{Now: func() time.Time { return time.Unix(5000, 0) }, Tokens: NewPersistentTokenCache(store), Prompt: func(Prompt) { t.Error("asked a person") }}

	header, err := apply(t, kv, env)

	if err != nil || header != "Bearer fresh" || !slices.Equal(forms, []string{"refresh_token:ref-old"}) {
		t.Fatalf("header %q err %v forms %v", header, err, forms)
	}
	if got := store.tokens[key]; got.Access != "fresh" || got.Refresh != "ref-old" {
		t.Errorf("stored %+v: the refresh token must survive a response that has none", got)
	}
}

func TestOAuth2_Given_a_grant_that_needs_nobody_Then_nothing_is_stored(t *testing.T) {
	p := newIDP(t)
	store := newMemStore()
	env := Env{Tokens: NewPersistentTokenCache(store)}

	_, err := apply(t, []string{"token_url", p.srv.URL + "/token", "client_id", "svc", "client_secret", "s3cr3t"}, env)

	if err != nil || store.saves != 0 || len(env.Tokens.List()) != 1 || env.Tokens.List()[0].Stored {
		t.Fatalf("err %v saves %d list %+v", err, store.saves, env.Tokens.List())
	}
}

func TestTokenCache_should_list_what_it_holds_without_secrets_and_forget_it_everywhere(t *testing.T) {
	p := newIDP(t)
	store := newMemStore()
	cache := NewPersistentTokenCache(store)
	if _, err := apply(t, deviceAuth(p), Env{After: immediately, Tokens: cache, Prompt: func(Prompt) {}}); err != nil {
		t.Fatal(err)
	}

	list := cache.List()
	fresh := NewPersistentTokenCache(store).List() // another process: only the store knows it

	if len(list) != 1 || !list[0].Stored || !list[0].HasRefresh || !strings.HasPrefix(list[0].Label, "device_code · 127.0.0.1:") ||
		!strings.Contains(list[0].Label, "client tv") || !strings.HasSuffix(list[0].Label, "scope read") {
		t.Fatalf("list %+v", list)
	}
	for _, secret := range []string{"s3cr3t", "tok-1", "ref-1"} {
		if strings.Contains(list[0].Label, secret) || strings.Contains(list[0].Key, secret) {
			t.Errorf("the listing shows %q", secret)
		}
	}
	if len(fresh) != 1 || fresh[0].Key != list[0].Key || fresh[0].Label != list[0].Label {
		t.Fatalf("a restarted process lists %+v", fresh)
	}

	if err := cache.Forget(list[0].Key); err != nil {
		t.Fatal(err)
	}
	prompts := 0
	_, err := apply(t, deviceAuth(p), Env{After: immediately, Tokens: cache, Prompt: func(Prompt) { prompts++ }})

	if len(NewPersistentTokenCache(store).List()) != 1 || err != nil || prompts != 1 {
		t.Fatalf("after Forget the next send must ask again once (prompts %d, err %v)", prompts, err)
	}
	if err := cache.Forget("no-such-key"); err != nil {
		t.Errorf("forgetting an unknown key: %v", err)
	}
}

func TestTokenCache_Given_a_nil_cache_Then_listing_and_forgetting_do_nothing(t *testing.T) {
	var c *TokenCache

	list := c.List()
	err := c.Forget("k")

	if list != nil || err != nil {
		t.Fatalf("%v %v", list, err)
	}
}

// newCacheKey is the key the signer built from kv uses, found by asking it.
func newCacheKey(t *testing.T, kv []string) string {
	t.Helper()
	s, err := New(auth("oauth2", kv...), Env{})
	if err != nil {
		t.Fatal(err)
	}
	return s.(*oauth2).key()
}
