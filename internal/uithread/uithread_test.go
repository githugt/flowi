// SPDX-License-Identifier: AGPL-3.0-or-later

package uithread_test

import (
	"testing"

	"github.com/githugt/flowi/internal/uithread"
)

func TestCheck_Given_debug_and_ui_goroutine_Then_panics(t *testing.T) {
	g := uithread.NewGuard(true)
	g.Mark()

	defer func() {
		if recover() == nil {
			t.Fatal("no panic for a library call on the UI goroutine")
		}
	}()
	g.Check("Validate")
}

func TestCheck_Given_other_goroutine_Then_passes(t *testing.T) {
	g := uithread.NewGuard(true)
	g.Mark()
	done := make(chan any)

	go func() {
		defer func() { done <- recover() }()
		g.Check("Validate")
	}()

	if p := <-done; p != nil {
		t.Fatalf("panicked off the UI goroutine: %v", p)
	}
}

func TestCheck_Given_not_debug_Then_never_panics(_ *testing.T) { // passing means no panic
	g := uithread.NewGuard(false)
	g.Mark()

	g.Check("Validate")
}
