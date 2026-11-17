// SPDX-License-Identifier: Apache-2.0

package dataset

import "testing"

func TestWholeNumber_Given_a_float_beyond_int64_When_read_Then_it_is_refused(t *testing.T) {
	// Before: the float was converted to int64 unchecked, so 1e30 wrapped to an arbitrary number and was written back.
	for _, v := range []any{1e30, -1e30, 9.3e18} {
		if n, ok := wholeNumber(v); ok {
			t.Errorf("wholeNumber(%v) = %d, true; want it refused", v, n)
		}
	}
	if n, ok := wholeNumber(float64(42)); !ok || n != 42 {
		t.Errorf("wholeNumber(42) = %d, %v; want 42, true", n, ok)
	}
}
