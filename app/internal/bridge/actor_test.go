// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"context"
	"slices"
	"testing"

	"github.com/githugt/flowi/internal/bridge"
)

func TestActor_should_run_posted_functions_in_order_on_one_goroutine(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := bridge.NewActor(ctx)
	var seen []int

	for i := range 50 {
		a.Post(func() { seen = append(seen, i) }) // no lock: only the actor touches seen
	}
	a.Do(func() {}) // runs after everything posted before it

	want := make([]int, 50)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(seen, want) {
		t.Fatalf("order %v", seen)
	}
}

func TestActor_Do_should_return_when_the_context_ends_before_it_ran(_ *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := bridge.NewActor(ctx)
	cancel()

	a.Do(func() {}) // must not block forever
}

func TestInlineActor_should_run_at_once_on_the_caller(t *testing.T) {
	a := bridge.NewInlineActor()
	ran := false

	a.Post(func() { ran = true })

	if !ran {
		t.Fatal("inline actor did not run the function")
	}
}
