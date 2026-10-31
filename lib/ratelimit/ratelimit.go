// SPDX-License-Identifier: Apache-2.0

// Package ratelimit reads the quota a server reports in its response headers: the RateLimit and RateLimit-Policy
// fields of the IETF draft (draft-ietf-httpapi-ratelimit-headers, still a draft: its field names may change), and the
// RateLimit-Limit/-Remaining/-Reset and X-RateLimit-* headers servers sent before it. A client can show what is left
// and wait for the quota to come back instead of failing.
package ratelimit

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Info is the quota a response reported. A figure the server did not give is -1 (Limit, Remaining) or zero.
type Info struct {
	Policy    string        // the draft's policy name, "" for the older headers
	Limit     int           // requests allowed in the window; -1 when unknown
	Remaining int           // requests left; -1 when unknown
	Reset     time.Duration // until the quota is back
	Window    time.Duration // the policy's window, when the server gave it
}

// Exhausted says nothing is left and the server said when it comes back, so waiting makes sense.
func (i Info) Exhausted() bool { return i.Remaining == 0 && i.Reset > 0 }

// Values is the quota as a flow reads it (nodes.<step>.ratelimit and response.ratelimit): policy, limit, remaining, reset
// (a duration), and wait, which is reset when nothing is left and zero otherwise, so a delay can take it as it is.
func (i Info) Values() map[string]any {
	wait := time.Duration(0)
	if i.Exhausted() {
		wait = i.Reset
	}
	return map[string]any{"policy": i.Policy, "limit": i.Limit, "remaining": i.Remaining, "reset": i.Reset, "wait": wait}
}

// Read reads the quota from the headers of a response received at now; found is false when it reports none. With several
// policies the one with the least left is returned.
func Read(h http.Header, now time.Time) (Info, bool) {
	if info, ok := readDraft(h); ok {
		return info, true
	}
	return readLegacy(h, now)
}

func readDraft(h http.Header) (Info, bool) {
	policies := map[string]Info{}
	for _, item := range structuredItems(h.Values("RateLimit-Policy")) {
		policies[item.name] = Info{Policy: item.name, Limit: item.number("q", -1), Remaining: -1, Window: seconds(item.number("w", 0))}
	}
	var best Info
	found := false
	for _, item := range structuredItems(h.Values("RateLimit")) {
		info, known := policies[item.name]
		if !known {
			info = Info{Policy: item.name, Limit: -1}
		}
		info.Remaining, info.Reset = item.number("r", -1), seconds(item.number("t", 0))
		if info.Remaining < 0 {
			continue
		}
		if !found || info.Remaining < best.Remaining {
			best, found = info, true
		}
	}
	return best, found
}

func readLegacy(h http.Header, now time.Time) (Info, bool) {
	get := func(names ...string) (int, bool) {
		for _, name := range names {
			if n, err := strconv.Atoi(strings.TrimSpace(h.Get(name))); err == nil {
				return n, true
			}
		}
		return 0, false
	}
	remaining, ok := get("RateLimit-Remaining", "X-RateLimit-Remaining", "X-Rate-Limit-Remaining")
	if !ok {
		return Info{}, false
	}
	limit, hasLimit := get("RateLimit-Limit", "X-RateLimit-Limit", "X-Rate-Limit-Limit")
	if !hasLimit {
		limit = -1
	}
	info := Info{Limit: limit, Remaining: remaining}
	if reset, ok := get("RateLimit-Reset", "X-RateLimit-Reset", "X-Rate-Limit-Reset"); ok {
		info.Reset = seconds(resetSeconds(reset, now))
	}
	return info, true
}

// resetSeconds is the seconds until the reset: some servers send that, others the epoch second it happens at.
func resetSeconds(reset int, now time.Time) int {
	const epochFloor = 1_000_000_000 // after 2001: a count of seconds this large is a moment, not a wait
	if reset >= epochFloor {
		return max(0, reset-int(now.Unix()))
	}
	return reset
}

func seconds(n int) time.Duration { return time.Duration(n) * time.Second }

// item is one member of a structured field list: "name";key=value;key=value.
type item struct {
	name   string
	params map[string]int
}

func (i item) number(key string, fallback int) int {
	if n, ok := i.params[key]; ok {
		return n
	}
	return fallback
}

// structuredItems reads the members of RateLimit and RateLimit-Policy values: a comma-separated list whose members are
// a quoted name or token followed by integer parameters. A member it cannot read is skipped.
func structuredItems(values []string) []item {
	var out []item
	for _, value := range values {
		for _, member := range strings.Split(value, ",") {
			parts := strings.Split(strings.TrimSpace(member), ";")
			name := strings.Trim(strings.TrimSpace(parts[0]), `"`)
			if name == "" {
				continue
			}
			it := item{name: name, params: map[string]int{}}
			for _, param := range parts[1:] {
				if k, v, ok := strings.Cut(strings.TrimSpace(param), "="); ok {
					if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
						it.params[k] = n
					}
				}
			}
			out = append(out, it)
		}
	}
	return out
}
