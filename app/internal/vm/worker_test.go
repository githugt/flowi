// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestWorker_should_run_off_the_posting_goroutine_and_post_the_result(t *testing.T) {
	var posted atomic.Int32
	w := vm.NewWorker(func(f func()) { posted.Add(1); f() })
	results := make(chan int, 1)

	vm.Run(w, func() int { return 42 }, func(n int) { results <- n })
	w.Wait()

	if got := <-results; got != 42 || posted.Load() != 1 {
		t.Fatalf("got %d, posted %d times", got, posted.Load())
	}
}
