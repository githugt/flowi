// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "testing"

func TestRelativeInside(t *testing.T) {
	for _, c := range []struct{ root, path, want string }{
		{"/col", "/col/data/sales.csv", "data/sales.csv"},
		{"/col", "/col", "."},
		{"/col", "/elsewhere/a.csv", "/elsewhere/a.csv"},
		{"/col", "/col2/a.csv", "/col2/a.csv"},
		{"", "/a.csv", "/a.csv"},
	} {
		if got := relativeInside(c.root, c.path); got != c.want {
			t.Errorf("%q in %q: %q, want %q", c.path, c.root, got, c.want)
		}
	}
}
