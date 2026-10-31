// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// toggleMenuBarEvent is what the page emits when Alt is tapped on Windows.
const toggleMenuBarEvent = "toggle-menu-bar"

// hideMenuBarUntilAlt keeps the window's menu bar out of sight, as the page already has its own toolbar,
// and shows it when the page reports a tap of Alt (and hides it again on the next one).
// The bar can only be hidden once the window exists, so that waits for the page to be ready.
func hideMenuBarUntilAlt(wails *application.App, window *application.WebviewWindow) {
	window.RegisterHook(events.Common.WindowRuntimeReady, func(*application.WindowEvent) { window.HideMenuBar() })
	wails.Event.On(toggleMenuBarEvent, func(*application.CustomEvent) { window.ToggleMenuBar() })
}
