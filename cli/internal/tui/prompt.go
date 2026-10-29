// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import "github.com/githugt/flowi-lib/engine"

// PromptText says what a request waits for the person to do, in one line a
// terminal can show while it waits.
func PromptText(e engine.AuthPrompted) string {
	if e.Kind == "device" {
		return "waiting for you: open " + e.URL + " and enter the code " + e.Code
	}
	return "waiting for you: open " + e.URL + " to sign in"
}
