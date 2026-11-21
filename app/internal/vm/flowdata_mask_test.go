// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func TestMaskedHeaderRows_should_hide_any_header_named_like_a_credential(t *testing.T) {
	// Before: only six names were masked, so X-Access-Token and X-Session-Id showed in full.
	rows := maskedHeaderRows([]engine.Header{
		{Name: "Authorization", Value: "Bearer abc.def"},
		{Name: "X-Access-Token", Value: "t0ken"},
		{Name: "X-Session-Id", Value: "s3ssion"},
		{Name: "Accept", Value: "application/json"},
		{Name: "X-Request-Id", Value: "r-1"},
	})

	want := map[string]string{
		"Authorization":  "Bearer ••••••",
		"X-Access-Token": "••••••",
		"X-Session-Id":   "••••••",
		"Accept":         "application/json",
		"X-Request-Id":   "r-1",
	}
	for _, r := range rows {
		if r.Value != want[r.Key] {
			t.Errorf("%s: got %q, want %q", r.Key, r.Value, want[r.Key])
		}
	}
}
