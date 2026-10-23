// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build server

package main

// systemReady is always true in the server build, which has no web view.
func systemReady() bool { return true }
