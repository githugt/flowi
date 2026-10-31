// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// CookieInfo is one cookie in a collection's saved jar, the one standalone
// sends share.
type CookieInfo struct {
	Key      string // identifies the cookie (name, domain, path) for DeleteCookie
	Name     string
	Value    string
	Domain   string // where it applies; the host that set it when the cookie names none
	Path     string
	Expires  time.Time // zero for a session cookie
	Secure   bool
	HTTPOnly bool
}

// Cookies lists the cookies in the saved jar of a collection (name or id),
// without those that have expired.
func (w *Workspace) Cookies(collection string) ([]CookieInfo, error) {
	file, err := w.jarFile(collection)
	if err != nil {
		return nil, err
	}
	stored, err := httpx.ReadStored(file, time.Now())
	if err != nil {
		return nil, err
	}
	out := make([]CookieInfo, 0, len(stored))
	for _, s := range stored {
		out = append(out, cookieInfo(s))
	}
	return out, nil
}

// DeleteCookie removes the cookie with key (CookieInfo.Key) from the saved
// jar of a collection. A send running at that moment may write the cookie
// back when it finishes.
func (w *Workspace) DeleteCookie(collection, key string) error {
	file, err := w.jarFile(collection)
	if err != nil {
		return err
	}
	stored, err := httpx.ReadStored(file, time.Now())
	if err != nil {
		return err
	}
	kept := slices.DeleteFunc(slices.Clone(stored), func(s httpx.StoredCookie) bool { return httpx.StoredKey(s) == key })
	if len(kept) == len(stored) {
		return nil // already gone
	}
	return httpx.WriteStored(file, kept)
}

func (w *Workspace) jarFile(collection string) (string, error) {
	w.mu.RLock()
	c, err := w.oneCollection(collection)
	w.mu.RUnlock()
	if err != nil {
		return "", fmt.Errorf("finding the cookie jar: %w", err)
	}
	return filepath.Join(w.DataDir(c.Manifest.ID), "cookies", "jar.json"), nil
}

func cookieInfo(s httpx.StoredCookie) CookieInfo {
	c := s.Cookie
	domain := strings.TrimPrefix(c.Domain, ".")
	if domain == "" {
		if u, err := url.Parse(s.URL); err == nil {
			domain = u.Hostname()
		}
	}
	return CookieInfo{Key: httpx.StoredKey(s), Name: c.Name, Value: c.Value, Domain: domain, Path: c.Path,
		Expires: c.Expires, Secure: c.Secure, HTTPOnly: c.HttpOnly}
}
