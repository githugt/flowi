// SPDX-License-Identifier: Apache-2.0

package announce

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

var now = time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC) //nolint:gochecknoglobals // a fixed instant shared by read-only tests

// testKey is a fixed signing key, so failures reproduce.
func testKey(seed byte) (ed25519.PublicKey, ed25519.PrivateKey) {
	priv := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
	return priv.Public().(ed25519.PublicKey), priv
}

type payload struct {
	Issued time.Time `json:"issued"`
	Items  []Item    `json:"items"`
}

// signed builds the feed document the publisher would host.
func signed(t *testing.T, priv ed25519.PrivateKey, p payload) []byte {
	t.Helper()
	body, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := json.Marshal(map[string]string{
		"payload":   base64.StdEncoding.EncodeToString(body),
		"signature": base64.StdEncoding.EncodeToString(ed25519.Sign(priv, body)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func sampleItems() []Item {
	return []Item{{ID: "welcome", Title: "Welcome to 1.0", Body: "Thanks for trying Flowi.", URL: "https://example.com/news/welcome", Level: "info"}}
}

// feedServer serves doc and counts requests.
type feedServer struct {
	srv  *httptest.Server
	hits atomic.Int32
	doc  atomic.Pointer[[]byte]
}

func newFeedServer(t *testing.T, doc []byte) *feedServer {
	t.Helper()
	f := &feedServer{}
	f.doc.Store(&doc)
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f.hits.Add(1)
		_, _ = w.Write(*f.doc.Load())
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newFeed(t *testing.T, url string, keys ...ed25519.PublicKey) (*Feed, string) {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "announcements.cache.json")
	f, err := New(Config{URL: url, Keys: keys, CacheFile: cache, TTL: time.Hour, Timeout: 2 * time.Second, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	return f, cache
}

func TestFetch_should_return_verified_items_and_cache_them(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, cache := newFeed(t, srv.srv.URL, pub)

	res, err := feed.Fetch(context.Background())

	if err != nil || len(res.Items) != 1 || res.Items[0].Title != "Welcome to 1.0" || res.Stale {
		t.Fatalf("res %+v err %v", res, err)
	}
	if _, err := os.Stat(cache); err != nil {
		t.Fatalf("not cached: %v", err)
	}
}

func TestFetch_Given_a_fresh_cache_Then_the_network_is_not_used(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, _ := newFeed(t, srv.srv.URL, pub)
	if _, err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}

	res, err := feed.Fetch(context.Background())

	if err != nil || len(res.Items) != 1 || srv.hits.Load() != 1 {
		t.Fatalf("res %+v err %v hits %d", res, err, srv.hits.Load())
	}
}

func TestFetch_Given_a_stale_cache_Then_it_is_refreshed(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, _ := newFeed(t, srv.srv.URL, pub)
	if _, err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	later := now.Add(2 * time.Hour)
	feed.cfg.Now = func() time.Time { return later }
	updated := sampleItems()
	updated[0].Title = "Second post"
	next := signed(t, priv, payload{Issued: later, Items: updated})
	srv.doc.Store(&next)

	res, err := feed.Fetch(context.Background())

	if err != nil || res.Items[0].Title != "Second post" || srv.hits.Load() != 2 {
		t.Fatalf("res %+v err %v hits %d", res, err, srv.hits.Load())
	}
}

func TestFetch_should_ignore_feeds_not_signed_by_a_trusted_key(t *testing.T) {
	pub, _ := testKey(1)
	_, attacker := testKey(2)
	srv := newFeedServer(t, signed(t, attacker, payload{Issued: now, Items: sampleItems()}))
	feed, cache := newFeed(t, srv.srv.URL, pub)

	res, err := feed.Fetch(context.Background())

	if !errors.Is(err, ErrBadSignature) || len(res.Items) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if _, err := os.Stat(cache); err == nil {
		t.Fatal("an unverified feed was cached")
	}
}

func TestFetch_should_accept_any_of_several_trusted_keys(t *testing.T) {
	oldPub, _ := testKey(1)
	newPub, newPriv := testKey(2)
	srv := newFeedServer(t, signed(t, newPriv, payload{Issued: now, Items: sampleItems()}))
	feed, _ := newFeed(t, srv.srv.URL, oldPub, newPub)

	res, err := feed.Fetch(context.Background())

	if err != nil || len(res.Items) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_should_reject_a_payload_that_was_changed_after_signing(t *testing.T) {
	pub, priv := testKey(1)
	good := signed(t, priv, payload{Issued: now, Items: sampleItems()})
	var doc map[string]string
	if err := json.Unmarshal(good, &doc); err != nil {
		t.Fatal(err)
	}
	forged, _ := json.Marshal(payload{Issued: now, Items: []Item{{ID: "x", Title: "Download this installer", URL: "https://evil.example"}}})
	doc["payload"] = base64.StdEncoding.EncodeToString(forged)
	tampered, _ := json.Marshal(doc)
	feed, _ := newFeed(t, newFeedServer(t, tampered).srv.URL, pub)

	res, err := feed.Fetch(context.Background())

	if !errors.Is(err, ErrBadSignature) || len(res.Items) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_Given_offline_Then_stale_cached_items_are_returned_with_the_reason(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, _ := newFeed(t, srv.srv.URL, pub)
	if _, err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv.srv.Close()
	feed.cfg.Now = func() time.Time { return now.Add(48 * time.Hour) }

	res, err := feed.Fetch(context.Background())

	if err == nil || !res.Stale || len(res.Items) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_Given_offline_and_no_cache_Then_empty_with_the_reason(t *testing.T) {
	pub, _ := testKey(1)
	srv := newFeedServer(t, nil)
	url := srv.srv.URL
	srv.srv.Close()
	feed, _ := newFeed(t, url, pub)

	res, err := feed.Fetch(context.Background())

	if err == nil || len(res.Items) != 0 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_Given_a_server_that_never_answers_Then_it_gives_up_at_the_timeout(t *testing.T) {
	pub, _ := testKey(1)
	release := make(chan struct{})
	slow := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer slow.Close()
	defer close(release)
	feed, err := New(Config{URL: slow.URL, Keys: []ed25519.PublicKey{pub}, CacheFile: filepath.Join(t.TempDir(), "c.json"), Timeout: 150 * time.Millisecond, Now: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()

	_, fetchErr := feed.Fetch(context.Background())

	if fetchErr == nil || time.Since(start) > 3*time.Second {
		t.Fatalf("err %v after %v", fetchErr, time.Since(start))
	}
}

func TestFetch_should_refuse_an_oversized_feed(t *testing.T) {
	pub, _ := testKey(1)
	huge := bytes.Repeat([]byte("a"), maxFeedBytes+1)
	feed, _ := newFeed(t, newFeedServer(t, huge).srv.URL, pub)

	_, err := feed.Fetch(context.Background())

	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err %v", err)
	}
}

func TestFetch_should_not_follow_redirects(t *testing.T) {
	pub, priv := testKey(1)
	target := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	redirector := httptest.NewServer(http.RedirectHandler(target.srv.URL, http.StatusFound))
	defer redirector.Close()
	feed, _ := newFeed(t, redirector.URL, pub)

	res, err := feed.Fetch(context.Background())

	if err == nil || len(res.Items) != 0 || target.hits.Load() != 0 {
		t.Fatalf("res %+v err %v hits %d", res, err, target.hits.Load())
	}
}

func TestFetch_should_not_accept_an_older_feed_than_the_cached_one(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, _ := newFeed(t, srv.srv.URL, pub)
	if _, err := feed.Fetch(context.Background()); err != nil {
		t.Fatal(err)
	}
	// An attacker replays a genuinely signed but older feed.
	older := sampleItems()
	older[0].Title = "Old, withdrawn post"
	replay := signed(t, priv, payload{Issued: now.Add(-24 * time.Hour), Items: older})
	srv.doc.Store(&replay)
	feed.cfg.Now = func() time.Time { return now.Add(2 * time.Hour) }

	res, err := feed.Fetch(context.Background())

	if !errors.Is(err, ErrRollback) || res.Items[0].Title != "Welcome to 1.0" {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_should_ignore_a_cache_file_that_was_edited(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, cache := newFeed(t, srv.srv.URL, pub)
	_, forgedPriv := testKey(9)
	forged := signed(t, forgedPriv, payload{Issued: now, Items: []Item{{ID: "evil", Title: "Run this"}}})
	if err := os.WriteFile(cache, forged, 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := feed.Fetch(context.Background())

	if err != nil || len(res.Items) != 1 || res.Items[0].ID != "welcome" {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_Given_a_corrupt_cache_Then_it_is_replaced(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: sampleItems()}))
	feed, cache := newFeed(t, srv.srv.URL, pub)
	if err := os.WriteFile(cache, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := feed.Fetch(context.Background())

	if err != nil || len(res.Items) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
}

func TestFetch_should_clean_and_bound_what_it_shows(t *testing.T) {
	pub, priv := testKey(1)
	var items []Item
	for i := range 30 {
		items = append(items, Item{ID: "n" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Title: "T"})
	}
	items[0] = Item{ID: "first", Title: "Hello\x1b[31m red\x00 world", Body: strings.Repeat("b", 5000), URL: "http://insecure.example/x", Level: "alarm"}
	items[1] = Item{ID: "https", Title: "Linked", URL: "https://example.com/ok", Level: "warning"}
	items[2] = Item{ID: "expired", Title: "Old", Until: now.Add(-time.Hour)}
	items[3] = Item{ID: "", Title: "no id"}
	items[4] = Item{ID: "untitled", Title: ""}
	srv := newFeedServer(t, signed(t, priv, payload{Issued: now, Items: items}))
	feed, _ := newFeed(t, srv.srv.URL, pub)

	res, err := feed.Fetch(context.Background())

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Items) != maxItems {
		t.Fatalf("%d items", len(res.Items))
	}
	first := res.Items[0]
	if strings.ContainsAny(first.Title, "\x1b\x00") || len([]rune(first.Body)) > maxBodyRunes || first.URL != "" || first.Level != "info" {
		t.Fatalf("first %+v", first)
	}
	if res.Items[1].URL != "https://example.com/ok" || res.Items[1].Level != "warning" {
		t.Fatalf("second %+v", res.Items[1])
	}
	for _, it := range res.Items {
		if it.ID == "expired" || it.ID == "" || it.ID == "untitled" {
			t.Fatalf("kept %+v", it)
		}
	}
}

func TestNew_should_refuse_what_cannot_work(t *testing.T) {
	pub, _ := testKey(1)
	tests := map[string]Config{
		"no url":        {Keys: []ed25519.PublicKey{pub}, CacheFile: "c"},
		"no keys":       {URL: "https://example.com/feed.json", CacheFile: "c"},
		"plain http":    {URL: "http://example.com/feed.json", Keys: []ed25519.PublicKey{pub}, CacheFile: "c"},
		"not a url":     {URL: "feed.json", Keys: []ed25519.PublicKey{pub}, CacheFile: "c"},
		"no cache file": {URL: "https://example.com/feed.json", Keys: []ed25519.PublicKey{pub}},
	}
	for name, cfg := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := New(cfg); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestParseKey(t *testing.T) {
	pub, _ := testKey(1)
	good := base64.StdEncoding.EncodeToString(pub)

	got, err := ParseKey(good)
	_, shortErr := ParseKey(base64.StdEncoding.EncodeToString([]byte("short")))
	_, junkErr := ParseKey("!!!")

	if err != nil || !bytes.Equal(got, pub) || shortErr == nil || junkErr == nil {
		t.Fatalf("got %v err %v short %v junk %v", got, err, shortErr, junkErr)
	}
}

func TestConfigFrom_should_be_off_without_a_url_or_when_toggled_off(t *testing.T) {
	pub, _ := testKey(1)
	key := base64.StdEncoding.EncodeToString(pub)
	tests := map[string]model.Settings{
		"nothing set":       {},
		"url but toggle":    {"announcements_url": "https://example.com/feed.json", "announcements_keys": []string{key}, "announcements": false},
		"keys but no url":   {"announcements_keys": []string{key}},
		"toggle on, no url": {"announcements": true},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			_, on, err := ConfigFrom(resolved(values), "cache.json")

			if on || err != nil {
				t.Fatalf("on=%v err=%v", on, err)
			}
		})
	}
}

func TestConfigFrom_should_read_url_and_keys(t *testing.T) {
	pub, _ := testKey(1)
	key := base64.StdEncoding.EncodeToString(pub)

	cfg, on, err := ConfigFrom(resolved(model.Settings{"announcements_url": "https://example.com/feed.json", "announcements_keys": []string{key}}), "cache.json")

	if !on || err != nil || cfg.URL != "https://example.com/feed.json" || len(cfg.Keys) != 1 || cfg.CacheFile != "cache.json" {
		t.Fatalf("cfg %+v on=%v err=%v", cfg, on, err)
	}
}

func TestConfigFrom_Given_a_url_without_a_valid_key_Then_error(t *testing.T) {
	_, on, err := ConfigFrom(resolved(model.Settings{"announcements_url": "https://example.com/feed.json", "announcements_keys": []string{"nonsense"}}), "cache.json")

	if err == nil || on {
		t.Fatalf("on=%v err=%v", on, err)
	}
	_, on, err = ConfigFrom(resolved(model.Settings{"announcements_url": "https://example.com/feed.json"}), "cache.json")
	if err == nil || on {
		t.Fatalf("no keys: on=%v err=%v", on, err)
	}
}

func resolved(values model.Settings) settings.Resolved {
	return settings.Resolve(settings.Schema(), settings.LayerValues{Layer: settings.LayerUser, Source: "settings.yaml", Values: values})
}

func TestSign_should_produce_a_feed_that_Fetch_accepts(t *testing.T) {
	pub, priv := testKey(1)
	doc, err := Sign(priv, now, sampleItems())
	if err != nil {
		t.Fatal(err)
	}
	feed, _ := newFeed(t, newFeedServer(t, doc).srv.URL, pub)

	res, fetchErr := feed.Fetch(context.Background())

	if fetchErr != nil || len(res.Items) != 1 || !res.Issued.Equal(now) {
		t.Fatalf("res %+v err %v", res, fetchErr)
	}
}

func TestLoad_should_report_why_there_is_nothing_to_fetch(t *testing.T) {
	pub, _ := testKey(1)
	key := base64.StdEncoding.EncodeToString(pub)
	tests := []struct {
		name   string
		values model.Settings
		want   error
	}{
		{"toggled off", model.Settings{"announcements": false, "announcements_url": "https://example.com/f.json", "announcements_keys": []string{key}}, ErrTurnedOff},
		{"no url", model.Settings{}, ErrNotConfigured},
		{"url without keys", model.Settings{"announcements_url": "https://example.com/f.json"}, ErrNoKeys},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(context.Background(), resolved(tt.values), filepath.Join(t.TempDir(), "c.json"), false)

			if !errors.Is(err, tt.want) {
				t.Fatalf("err %v, want %v", err, tt.want)
			}
		})
	}
}

func TestLoad_should_fetch_with_the_configured_feed(t *testing.T) {
	pub, priv := testKey(1)
	srv := newFeedServer(t, signed(t, priv, payload{Issued: time.Now(), Items: sampleItems()}))
	values := model.Settings{"announcements_url": srv.srv.URL, "announcements_keys": []string{base64.StdEncoding.EncodeToString(pub)}}
	cache := filepath.Join(t.TempDir(), "c.json")

	res, err := Load(context.Background(), resolved(values), cache, false)
	again, _ := Load(context.Background(), resolved(values), cache, false)
	_, _ = Load(context.Background(), resolved(values), cache, true)

	if err != nil || len(res.Items) != 1 || len(again.Items) != 1 {
		t.Fatalf("res %+v err %v", res, err)
	}
	if srv.hits.Load() != 2 {
		t.Fatalf("hits %d: the second call should use the cache and the refresh should not", srv.hits.Load())
	}
}
