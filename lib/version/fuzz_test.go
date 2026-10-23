// SPDX-License-Identifier: Apache-2.0

package version

import "testing"

func FuzzParse(f *testing.F) {
	for _, seed := range []string{"1", "1.10", "2.0.1", "01", "", "1..2"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		l, err := Parse(s)
		if err != nil {
			return
		}

		again, err := Parse(l.String())

		if err != nil {
			t.Fatalf("re-parse of %q failed: %v", l.String(), err)
		}
		if again.Compare(l) != 0 {
			t.Fatalf("re-parse of %q compares unequal", s)
		}
	})
}
