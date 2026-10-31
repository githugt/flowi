// SPDX-License-Identifier: Apache-2.0

package announce

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"time"

	"github.com/githugt/flowi/lib/settings"
)

// ConfigFrom reads the announcements settings. on is false when the toggle
// is off or no feed URL is set, which is the default: nothing is fetched.
// A URL without a usable key is an error, not a silent skip, since showing
// unsigned text is exactly what the keys exist to prevent.
func ConfigFrom(r settings.Resolved, cacheFile string) (cfg Config, on bool, err error) {
	if enabled, _ := value(r, "announcements").(bool); !enabled {
		return Config{}, false, nil
	}
	url, _ := value(r, "announcements_url").(string)
	if url == "" {
		return Config{}, false, nil
	}
	raw, _ := value(r, "announcements_keys").([]string)
	keys := make([]ed25519.PublicKey, 0, len(raw))
	for n, s := range raw {
		k, err := ParseKey(s)
		if err != nil {
			return Config{}, false, fmt.Errorf("announcements_keys[%d]: %w", n, err)
		}
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return Config{}, false, fmt.Errorf("announcements_url is set but announcements_keys is empty: %w", ErrNoKeys)
	}
	return Config{URL: url, Keys: keys, CacheFile: cacheFile}, true, nil
}

func value(r settings.Resolved, name string) any {
	v, _ := r.Get(name)
	return v.Value
}

// refreshTTL makes any cached feed count as too old, forcing a fetch.
const refreshTTL = time.Nanosecond

// Load fetches the announcements the settings ask for: ErrTurnedOff when the
// announcements toggle is off, ErrNotConfigured when no feed URL is set,
// otherwise as Feed.Fetch. refresh ignores a recent cache.
func Load(ctx context.Context, r settings.Resolved, cacheFile string, refresh bool) (Result, error) {
	if enabled, _ := value(r, "announcements").(bool); !enabled {
		return Result{}, ErrTurnedOff
	}
	cfg, on, err := ConfigFrom(r, cacheFile)
	if err != nil {
		return Result{}, err
	}
	if !on {
		return Result{}, ErrNotConfigured
	}
	if refresh {
		cfg.TTL = refreshTTL
	}
	feed, err := New(cfg)
	if err != nil {
		return Result{}, err
	}
	return feed.Fetch(ctx)
}
