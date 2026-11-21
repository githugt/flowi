// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"

	"github.com/githugt/flowi/lib/settings"
)

func TestDefaultText_should_write_a_built_in_value_the_way_the_page_shows_it(t *testing.T) {
	// Before: the page did not send the default, so a changed value never said what it had been.
	cases := map[string]struct {
		key  settings.Key
		want string
	}{
		"duration": {settings.Key{Default: "30s"}, "30s"},
		"bool":     {settings.Key{Default: true}, "true"},
		"number":   {settings.Key{Default: 5}, "5"},
		"list":     {settings.Key{Default: []any{"Accept: */*", "Cache-Control: no-cache"}}, "Accept: */*, Cache-Control: no-cache"},
		"none":     {settings.Key{}, ""},
	}
	for name, c := range cases {
		if got := defaultText(c.key); got != c.want {
			t.Errorf("%s: got %q, want %q", name, got, c.want)
		}
	}
}
