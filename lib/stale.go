// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// StaleReason says why a value a recording froze would be wrong to use again now, or "" when nothing says so. A fork
// starts from recorded values, and the one thing a recording cannot know is that time has passed since: a token
// whose exp is behind us, or a value named for an expiry (expires_at, token_expiry) that is already in the past.
// Anything else may be stale too, but nothing in the value says so.
func StaleReason(name string, value any, now time.Time) string {
	if text, ok := value.(string); ok {
		if exp, ok := tokenExpiry(text); ok {
			return expiredText("the token", exp, now)
		}
	}
	if !strings.Contains(strings.ToLower(name), "expir") {
		return ""
	}
	if exp, ok := instantOf(value); ok {
		return expiredText("this", exp, now)
	}
	return ""
}

func expiredText(what string, at, now time.Time) string {
	if !at.Before(now) {
		return ""
	}
	return fmt.Sprintf("%s expired %s ago", what, now.Sub(at).Round(time.Minute))
}

// tokenExpiry reads the exp claim of a JWT-shaped string. The signature is not checked: only the time is wanted.
func tokenExpiry(s string) (time.Time, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp *float64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == nil {
		return time.Time{}, false
	}
	return time.Unix(int64(*claims.Exp), 0), true
}

// instantOf reads a time from an RFC 3339 string or a number of unix seconds.
func instantOf(v any) (time.Time, bool) {
	switch x := v.(type) {
	case string:
		t, err := time.Parse(time.RFC3339, x)
		return t, err == nil
	case float64:
		return time.Unix(int64(x), 0), x > 0
	}
	return time.Time{}, false
}
