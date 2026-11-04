// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"time"
)

func jwtExpiring(at time.Time) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc([]byte(fmt.Sprintf(`{"sub":"u1","exp":%d}`, at.Unix()))) + "." + enc([]byte("sig"))
}

func TestStaleReason_should_flag_what_a_replay_would_use_after_it_expired(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	past, future := now.Add(-3*time.Hour), now.Add(time.Hour)
	cases := []struct {
		name  string
		field string
		value any
		want  string // a word the reason holds; "" means not stale
	}{
		{"a token that ran out", "token", jwtExpiring(past), "expired 3h"},
		{"a token still good", "token", jwtExpiring(future), ""},
		{"an expiry time in the past", "expires_at", past.Format(time.RFC3339), "3h"},
		{"an expiry time ahead", "expires_at", future.Format(time.RFC3339), ""},
		{"an expiry in unix seconds in the past", "token_expiry", float64(past.Unix()), "3h"},
		{"a time that is not named an expiry", "created_at", past.Format(time.RFC3339), ""},
		{"text that is not a token", "name", "a.b.c", ""},
		{"a number", "count", float64(3), ""},
		{"nothing", "x", nil, ""},
	}

	for _, c := range cases {
		got := StaleReason(c.field, c.value, now)

		if (c.want == "") != (got == "") || !strings.Contains(got, c.want) {
			t.Errorf("%s: got %q, want it to hold %q", c.name, got, c.want)
		}
	}
}
