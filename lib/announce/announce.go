// SPDX-License-Identifier: Apache-2.0

package announce

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Errors a fetch can report.
var (
	ErrBadSignature  = errors.New("announcements: the feed is not signed by a trusted key")
	ErrTooLarge      = errors.New("announcements: the feed is too large")
	ErrRollback      = errors.New("announcements: the feed is older than the one already held")
	ErrMalformed     = errors.New("announcements: the feed is malformed")
	ErrNotConfigured = errors.New("announcements: no feed is configured")
	ErrNoKeys        = errors.New("announcements: a feed is configured but no trusted key")
	ErrTurnedOff     = errors.New("announcements: turned off")
	ErrBadKey        = errors.New("announcements: not a valid trusted key")
)

const (
	maxFeedBytes   = 256 << 10
	maxItems       = 20
	maxIDLen       = 64
	maxTitleRunes  = 120
	maxBodyRunes   = 1000
	maxURLLen      = 300
	defaultTTL     = 6 * time.Hour
	defaultTimeout = 3 * time.Second
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// Item is one announcement, already cleaned for display.
type Item struct {
	ID    string    `json:"id"`
	Title string    `json:"title"`
	Body  string    `json:"body,omitempty"`
	URL   string    `json:"url,omitempty"`   // https only
	Level string    `json:"level,omitempty"` // info | warning
	Until time.Time `json:"until,omitempty"` // zero: no end
}

// Result is what a fetch found. Stale means the items come from the cache
// because a fresh feed could not be had.
type Result struct {
	Items  []Item
	Stale  bool
	Issued time.Time
}

// Config configures a Feed.
type Config struct {
	URL       string
	Keys      []ed25519.PublicKey
	CacheFile string
	TTL       time.Duration // how long a cached feed is used without asking again; default 6h
	Timeout   time.Duration // for the whole request; default 3s
	Client    *http.Client  // nil → a client with Timeout that does not follow redirects
	Now       func() time.Time
}

// Feed fetches and caches the announcements. It is safe for concurrent use
// in the sense that it holds no state beyond its cache file, which is
// replaced atomically.
type Feed struct {
	cfg    Config
	client *http.Client
}

// New checks the configuration. The URL must be https (plain http only for
// loopback, for tests and local mirrors) and at least one key is required.
func New(cfg Config) (*Feed, error) {
	if cfg.URL == "" {
		return nil, ErrNotConfigured
	}
	if len(cfg.Keys) == 0 {
		return nil, ErrNoKeys
	}
	if err := checkFeedURL(cfg.URL); err != nil {
		return nil, err
	}
	if cfg.CacheFile == "" {
		return nil, errors.New("announcements: no cache file")
	}
	if cfg.TTL <= 0 {
		cfg.TTL = defaultTTL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	client := cfg.Client
	if client == nil {
		client = &http.Client{
			Timeout:       cfg.Timeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return &Feed{cfg: cfg, client: client}, nil
}

func checkFeedURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return fmt.Errorf("announcements: %q is not a feed URL", raw)
	}
	switch u.Scheme {
	case "https":
		return nil
	case "http":
		if isLoopback(u.Hostname()) {
			return nil
		}
	}
	return fmt.Errorf("announcements: the feed URL must be https, got %q", u.Scheme+"://"+u.Host)
}

func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// ParseKey reads a base64 Ed25519 public key.
func ParseKey(s string) (ed25519.PublicKey, error) {
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil || len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("%w: a key is the base64 of a 32-byte Ed25519 public key", ErrBadKey)
	}
	return ed25519.PublicKey(raw), nil
}

// Fetch returns the current announcements. A fresh cache is used without
// touching the network. Otherwise the feed is fetched, verified and cached;
// if that fails, the cached feed (if it verifies) is returned as Stale
// together with the error, which callers normally log and ignore.
func (f *Feed) Fetch(ctx context.Context) (Result, error) {
	cached, fetched, haveCache := f.loadCache()
	if haveCache && f.fresh(fetched) {
		return f.result(cached, false), nil
	}
	doc, err := f.download(ctx)
	if err == nil {
		var fresh parsed
		if fresh, err = f.verify(doc); err == nil {
			if haveCache && fresh.issued.Before(cached.issued) {
				err = ErrRollback
			} else {
				f.saveCache(doc)
				return f.result(fresh, false), nil
			}
		}
	}
	if haveCache {
		return f.result(cached, true), err
	}
	return Result{}, err
}

func (f *Feed) fresh(fetched time.Time) bool {
	age := f.cfg.Now().Sub(fetched)
	return age >= 0 && age < f.cfg.TTL
}

func (f *Feed) download(ctx context.Context) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, f.cfg.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.cfg.URL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("announcements: building the request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("announcements: fetching the feed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // everything needed has been read
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("announcements: the server answered %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedBytes+1))
	if err != nil {
		return nil, fmt.Errorf("announcements: reading the feed: %w", err)
	}
	if len(body) > maxFeedBytes {
		return nil, ErrTooLarge
	}
	return body, nil
}

// parsed is a verified feed.
type parsed struct {
	issued time.Time
	items  []Item
}

type document struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

type feedPayload struct {
	Issued time.Time `json:"issued"`
	Items  []Item    `json:"items"`
}

// verify checks the signature over the decoded payload against every
// trusted key and decodes the payload.
func (f *Feed) verify(doc []byte) (parsed, error) {
	var d document
	if err := json.Unmarshal(doc, &d); err != nil {
		return parsed{}, ErrMalformed
	}
	payload, err1 := base64.StdEncoding.DecodeString(d.Payload)
	sig, err2 := base64.StdEncoding.DecodeString(d.Signature)
	if err1 != nil || err2 != nil {
		return parsed{}, ErrMalformed
	}
	if !f.signedByTrustedKey(payload, sig) {
		return parsed{}, ErrBadSignature
	}
	var p feedPayload
	if err := json.Unmarshal(payload, &p); err != nil {
		return parsed{}, ErrMalformed
	}
	return parsed{issued: p.Issued, items: p.Items}, nil
}

func (f *Feed) signedByTrustedKey(payload, sig []byte) bool {
	if len(sig) != ed25519.SignatureSize {
		return false
	}
	for _, k := range f.cfg.Keys {
		if ed25519.Verify(k, payload, sig) {
			return true
		}
	}
	return false
}

// result cleans the items for display and drops the ended ones.
func (f *Feed) result(p parsed, stale bool) Result {
	now := f.cfg.Now()
	var out []Item
	for _, it := range p.items {
		clean, ok := cleanItem(it)
		if !ok || (!clean.Until.IsZero() && !clean.Until.After(now)) {
			continue
		}
		if out = append(out, clean); len(out) == maxItems {
			break
		}
	}
	return Result{Items: out, Stale: stale, Issued: p.issued}
}

// cleanItem makes an item safe to show: no control or invisible characters,
// bounded lengths, https links only. Items without a usable id or title are
// dropped.
func cleanItem(it Item) (Item, bool) {
	it.ID = strings.TrimSpace(it.ID)
	if len(it.ID) > maxIDLen || !idPattern.MatchString(it.ID) {
		return Item{}, false
	}
	it.Title = truncate(printable(it.Title), maxTitleRunes)
	if it.Title == "" {
		return Item{}, false
	}
	it.Body = truncate(printable(it.Body), maxBodyRunes)
	if it.Level != "warning" {
		it.Level = "info"
	}
	it.URL = httpsURL(it.URL)
	return it, true
}

// printable removes control characters and invisible formatting characters
// (zero-width and bidirectional controls), which can disguise text.
func printable(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, s)
}

func truncate(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxRunes {
		return string(r[:maxRunes])
	}
	return s
}

func httpsURL(raw string) string {
	if raw == "" || len(raw) > maxURLLen {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return ""
	}
	return u.String()
}

// cacheFile is the cache document: the feed exactly as served, so it is
// verified again on every read, and when it was fetched.
type cacheFile struct {
	Fetched time.Time       `json:"fetched"`
	Feed    json.RawMessage `json:"feed"`
}

// loadCache returns the cached feed if the file exists and its feed still
// verifies. A missing, damaged or forged cache is simply no cache.
func (f *Feed) loadCache() (parsed, time.Time, bool) {
	data, err := os.ReadFile(f.cfg.CacheFile)
	if err != nil {
		return parsed{}, time.Time{}, false
	}
	var c cacheFile
	if err := json.Unmarshal(data, &c); err != nil {
		return parsed{}, time.Time{}, false
	}
	p, err := f.verify(c.Feed)
	if err != nil {
		return parsed{}, time.Time{}, false
	}
	return p, c.Fetched, true
}

// saveCache stores a verified feed. Failing to cache costs only a refetch,
// so the error is not surfaced.
func (f *Feed) saveCache(doc []byte) {
	data, err := json.Marshal(cacheFile{Fetched: f.cfg.Now().UTC(), Feed: doc})
	if err != nil {
		return
	}
	dir := filepath.Dir(f.cfg.CacheFile)
	if os.MkdirAll(dir, 0o700) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".announcements-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), f.cfg.CacheFile) != nil {
		_ = os.Remove(tmp.Name()) // best effort: a leftover temp file is harmless
	}
}

// Sign builds the feed document a publisher hosts: items issued at the
// given time, signed with priv. Publishing tools and tests use it.
func Sign(priv ed25519.PrivateKey, issued time.Time, items []Item) ([]byte, error) {
	payload, err := json.Marshal(feedPayload{Issued: issued.UTC(), Items: items})
	if err != nil {
		return nil, fmt.Errorf("announcements: encoding the payload: %w", err)
	}
	return json.Marshal(document{
		Payload:   base64.StdEncoding.EncodeToString(payload),
		Signature: base64.StdEncoding.EncodeToString(ed25519.Sign(priv, payload)),
	})
}
