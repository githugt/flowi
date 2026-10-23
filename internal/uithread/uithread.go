// SPDX-License-Identifier: AGPL-3.0-or-later

// Package uithread guards the UI goroutine: library calls (disk, network,
// vault) must never run on it, or the window freezes. In debug mode a call
// made there panics, so the mistake shows up in development and tests.
package uithread

import (
	"bytes"
	"fmt"
	"runtime"
	"strconv"
	"sync/atomic"
)

// Guard knows which goroutine is the UI goroutine.
type Guard struct {
	debug bool
	ui    atomic.Uint64 // goroutine id recorded by Mark; 0 before
}

// NewGuard returns a guard; debug decides whether Check panics.
func NewGuard(debug bool) *Guard { return &Guard{debug: debug} }

// Mark records the calling goroutine as the UI goroutine. Call it from a
// function the app's toolkit runs on its UI goroutine (a lifecycle hook).
func (g *Guard) Mark() { g.ui.Store(goroutineID()) }

// Check panics in debug mode when called on the UI goroutine. what names
// the library call for the message.
func (g *Guard) Check(what string) {
	if !g.debug {
		return
	}
	if ui := g.ui.Load(); ui != 0 && ui == goroutineID() {
		panic(fmt.Sprintf("uithread: library call %s on the UI goroutine; run it through vm.Worker", what))
	}
}

// goroutineID parses the current goroutine's id from its stack header
// ("goroutine 18 [running]:"). It is slow, so it is used only in debug
// mode and for Mark.
func goroutineID() uint64 {
	buf := make([]byte, 64)
	buf = buf[:runtime.Stack(buf, false)]
	buf = bytes.TrimPrefix(buf, []byte("goroutine "))
	id, _, _ := bytes.Cut(buf, []byte(" "))
	n, err := strconv.ParseUint(string(id), 10, 64)
	if err != nil {
		return 0 // unknown format: the guard then never matches
	}
	return n
}
