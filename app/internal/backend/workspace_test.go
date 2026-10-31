// SPDX-License-Identifier: AGPL-3.0-or-later

package backend_test

import (
	"testing"

	"github.com/githugt/flowi/app/internal/backend"
	"github.com/githugt/flowi/app/internal/uithread"
)

func TestWorkspace_Given_call_on_ui_goroutine_in_debug_Then_panics_before_the_library(t *testing.T) {
	guard := uithread.NewGuard(true)
	guard.Mark()
	b := backend.New(nil, guard) // a nil workspace would panic differently if reached

	defer func() {
		p := recover()
		msg, _ := p.(string)
		if msg == "" || msg[:9] != "uithread:" {
			t.Fatalf("recovered %v, want the uithread guard", p)
		}
	}()
	b.Validate()
}
