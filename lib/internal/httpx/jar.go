// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

// StoredCookie is a cookie with the URL that set it.
type StoredCookie struct {
	URL    string      `json:"url"`
	Cookie http.Cookie `json:"cookie"`
}

// Jar is a cookie jar that can be saved and loaded. The standard jar
// cannot list its cookies, so Jar keeps a log of what was set (the latest
// value of each cookie) and replays it into a fresh standard jar on load.
type Jar struct {
	now func() time.Time

	mu  sync.Mutex // guards log; jar is safe for concurrent use itself
	jar *cookiejar.Jar
	log []StoredCookie
}

// NewJar returns an empty in-memory jar.
func NewJar() *Jar {
	// cookiejar.New only fails for a nil error-returning option set, which
	// this is not.
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})
	return &Jar{now: time.Now, jar: jar}
}

// LoadJar reads a saved jar, dropping cookies expired at now. A missing
// file is an empty jar.
func LoadJar(file string, now time.Time) (*Jar, error) {
	j := NewJar()
	saved, err := ReadStored(file, now)
	if err != nil {
		return nil, err
	}
	for _, s := range saved {
		u, _ := url.Parse(s.URL) // ReadStored kept only entries whose URL parses
		c := s.Cookie
		j.SetCookies(u, []*http.Cookie{&c})
	}
	return j, nil
}

// ReadStored reads the cookies of a saved jar without building a jar,
// dropping expired and damaged entries. A missing file holds none.
func ReadStored(file string, now time.Time) ([]StoredCookie, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading cookie jar: %w", err)
	}
	var saved []StoredCookie
	if err := json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("reading cookie jar %s: %w", file, err)
	}
	return slices.DeleteFunc(saved, func(s StoredCookie) bool {
		_, err := url.Parse(s.URL)
		return err != nil || expired(s.Cookie, now) // a damaged entry or an expired cookie: skip it, keep the rest
	}), nil
}

// WriteStored saves cookies as a jar file, as Save does.
func WriteStored(file string, cookies []StoredCookie) error {
	data, err := json.MarshalIndent(cookies, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cookie jar: %w", err)
	}
	return writeFileAtomic(file, data, 0o600)
}

// StoredKey identifies a stored cookie by name, domain and path: the
// triple that makes two cookies one.
func StoredKey(s StoredCookie) string {
	u, err := url.Parse(s.URL)
	if err != nil {
		return ""
	}
	return cookieKey(u, s.Cookie)
}

// Clone returns an independent jar holding this jar's unexpired cookies:
// later changes to either do not reach the other.
func (j *Jar) Clone() *Jar {
	c := NewJar()
	c.now = j.now
	now := j.now()
	for _, s := range j.All() {
		u, err := url.Parse(s.URL)
		if err != nil || expired(s.Cookie, now) {
			continue // as on load: a damaged or expired entry is dropped
		}
		cookie := s.Cookie
		c.SetCookies(u, []*http.Cookie{&cookie})
	}
	return c
}

// SetCookies implements http.CookieJar.
func (j *Jar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.jar.SetCookies(u, cookies)
	now := j.now()
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, c := range cookies {
		j.record(u, *c, now)
	}
}

// record keeps the latest state of one cookie in the log. Callers hold mu.
// The cookie is one a server sent; its attributes are recorded as given
// (gosec G124 targets cookies a server creates).
func (j *Jar) record(u *url.URL, c http.Cookie, now time.Time) { //nolint:gosec // see above
	if c.MaxAge > 0 {
		c.Expires, c.MaxAge = now.Add(time.Duration(c.MaxAge)*time.Second), 0 // absolute, so it survives saving
	}
	key := cookieKey(u, c)
	j.log = slices.DeleteFunc(j.log, func(s StoredCookie) bool {
		su, err := url.Parse(s.URL)
		return err == nil && cookieKey(su, s.Cookie) == key
	})
	if c.MaxAge < 0 || expired(c, now) {
		return // a deletion
	}
	j.log = append(j.log, StoredCookie{URL: u.String(), Cookie: c})
}

func cookieKey(u *url.URL, c http.Cookie) string {
	domain := strings.ToLower(strings.TrimPrefix(c.Domain, "."))
	if domain == "" {
		domain = strings.ToLower(u.Hostname())
	}
	p := c.Path
	if p == "" || !strings.HasPrefix(p, "/") {
		p = defaultPath(u.Path)
	}
	return c.Name + "\x00" + domain + "\x00" + p
}

// defaultPath is RFC 6265 §5.1.4's default cookie path.
func defaultPath(p string) string {
	if !strings.HasPrefix(p, "/") || strings.Count(p, "/") == 1 {
		return "/"
	}
	return path.Dir(p)
}

func expired(c http.Cookie, now time.Time) bool {
	return !c.Expires.IsZero() && !c.Expires.After(now)
}

// Cookies implements http.CookieJar.
func (j *Jar) Cookies(u *url.URL) []*http.Cookie { return j.jar.Cookies(u) }

// All returns every cookie currently held, in the order first set.
func (j *Jar) All() []StoredCookie {
	j.mu.Lock()
	defer j.mu.Unlock()
	return slices.Clone(j.log)
}

// Save writes the jar atomically, dropping cookies that have expired.
// Session cookies (no expiry) are kept: a series of standalone sends acts
// as one long browser session until the jar is cleared.
func (j *Jar) Save(file string) error {
	now := j.now()
	live := slices.DeleteFunc(j.All(), func(s StoredCookie) bool { return expired(s.Cookie, now) })
	data, err := json.MarshalIndent(live, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding cookie jar: %w", err)
	}
	return writeFileAtomic(file, data, 0o600)
}

// writeFileAtomic writes through a temp file in the same directory and a
// rename, so readers never see a partial file.
func writeFileAtomic(file string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(file)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // removes the temp file only if the rename did not happen
	_, err = tmp.Write(data)
	if err == nil {
		err = tmp.Chmod(mode)
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), file)
	}
	if err != nil {
		return fmt.Errorf("writing %s: %w", file, err)
	}
	return nil
}
