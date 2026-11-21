// SPDX-License-Identifier: Apache-2.0

package trigger

import (
	"fmt"
	"testing"
	"time"
)

// Before: only entries whose window had ended were dropped, so a flood of
// distinct addresses inside one minute grew the table without limit.
func TestFailureLimiter_Given_a_flood_of_addresses_When_all_are_live_Then_the_table_stays_bounded(t *testing.T) {
	now := time.Unix(1000, 0)
	l := newFailureLimiter(func() time.Time { return now })

	for i := range 3 * maxTracked {
		now = now.Add(time.Millisecond)
		l.recordFailure(fmt.Sprintf("10.0.%d.%d", i/256, i%256))
	}

	if len(l.seen) > maxTracked {
		t.Fatalf("%d addresses tracked, want at most %d", len(l.seen), maxTracked)
	}
	if _, ok := l.seen[fmt.Sprintf("10.0.%d.%d", (3*maxTracked-1)/256, (3*maxTracked-1)%256)]; !ok {
		t.Fatal("the newest address was forgotten")
	}
}
