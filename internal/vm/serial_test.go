// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"testing"
)

func TestOpQueue_should_start_a_job_only_after_the_one_before_finished(t *testing.T) {
	var q opQueue
	var order []string
	var finishFirst func()

	q.add(func(finish func()) { order = append(order, "a start"); finishFirst = finish })
	q.add(func(finish func()) { order = append(order, "b start"); finish() })
	before := slices.Clone(order)
	finishFirst()

	if !slices.Equal(before, []string{"a start"}) || !slices.Equal(order, []string{"a start", "b start"}) || q.busy {
		t.Fatalf("before %v after %v busy %v", before, order, q.busy)
	}
}

func TestListeners_should_call_in_order_and_forget_a_removed_one(t *testing.T) {
	var l listeners
	var got []string
	l.add(func() { got = append(got, "a") })
	remove := l.add(func() { got = append(got, "b") })
	l.add(func() { got = append(got, "c") })

	l.fire()
	remove()
	l.fire()

	if !slices.Equal(got, []string{"a", "b", "c", "a", "c"}) {
		t.Fatalf("calls %v", got)
	}
}
