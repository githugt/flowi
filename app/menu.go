// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"runtime"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// menuAction is one custom item of the application menu: its label, the
// shortcut ("" for none) and the action the window runs (the "menu" event).
type menuAction struct{ label, shortcut, action string }

// buildMenu is the application menu. The standard roles (app, edit, window)
// give the system's own items and shortcuts; the rest asks the window to open
// a screen or close a tab, which only it knows about.
func buildMenu(wails *application.App, emit func(name string, data any)) *application.Menu {
	menu := wails.Menu.New()
	if runtime.GOOS == "darwin" {
		menu.AddRole(application.AppMenu)
	}
	add := func(m *application.Menu, items ...menuAction) {
		for _, it := range items {
			if it.label == "" {
				m.AddSeparator()
				continue
			}
			item := m.Add(it.label)
			if it.shortcut != "" {
				item.SetAccelerator(it.shortcut)
			}
			item.OnClick(func(*application.Context) { emit("menu", it.action) })
		}
	}
	add(menu.AddSubmenu("File"),
		menuAction{"New request", "CmdOrCtrl+N", "new-request"},
		menuAction{"New flow", "CmdOrCtrl+Shift+N", "new-flow"},
		menuAction{"", "", ""},
		menuAction{"Close tab", "CmdOrCtrl+W", "close-tab"},
		menuAction{"Next tab", "CmdOrCtrl+Shift+]", "next-tab"},
		menuAction{"Previous tab", "CmdOrCtrl+Shift+[", "previous-tab"},
	)
	menu.AddRole(application.EditMenu)
	add(menu.AddSubmenu("Workspace"),
		menuAction{"Collections…", "", "collections"},
		menuAction{"Import…", "CmdOrCtrl+I", "import"},
		menuAction{"", "", ""},
		menuAction{"Secrets…", "", "secrets"},
		menuAction{"Schedules…", "", "schedules"},
		menuAction{"Replayer…", "", "replayer"},
		menuAction{"Checks…", "", "checks"},
		menuAction{"Blueprints…", "", "blueprints"},
		menuAction{"", "", ""},
		menuAction{"Settings…", "CmdOrCtrl+,", "settings"},
	)
	menu.AddRole(application.WindowMenu)
	add(menu.AddSubmenu("Help"),
		menuAction{"Welcome", "", "welcome"},
		menuAction{"Telemetry…", "", "telemetry"},
	)
	return menu
}
