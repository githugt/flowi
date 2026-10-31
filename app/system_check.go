// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build !server

package main

import (
	"os"

	"github.com/githugt/flowi/internal/preflight"
)

// systemReady checks, before the window is made, that the system has what the web view needs; the message says what to
// install. The server build (for trying the pages in a browser) has no web view, so it checks nothing.
func systemReady() bool { return preflight.Run(os.Stderr) }
