// SPDX-License-Identifier: AGPL-3.0-or-later

// Package vm holds the GUI's view-models: plain Go, no UI toolkit. Each one
// keeps the state of a screen, turns user intents into Backend calls run
// by a Worker off the UI goroutine, and reports changes through callbacks
// that the Worker's poster delivers on the UI goroutine. Screens in
// package ui only bind these to widgets, so the behaviour is tested here
// headlessly.
package vm
