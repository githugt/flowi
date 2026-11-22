// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Before: the Edit menu came from the EditMenu role, whose Delete item has Backspace as its shortcut: the menu
// took the key, and Backspace in a text field deleted nothing unless something was selected.
func TestEditRoles_Given_the_edit_menu_off_the_Mac_When_listed_Then_it_has_no_Delete_item(t *testing.T) {
	roles := editRoles()

	if slices.Contains(roles, application.Delete) {
		t.Fatalf("the Edit menu has Delete, which takes Backspace: %v", roles)
	}
	for _, want := range []application.Role{application.Undo, application.Redo, application.Cut, application.Copy, application.Paste, application.SelectAll} {
		if !slices.Contains(roles, want) {
			t.Fatalf("the Edit menu lost %v: %v", want, roles)
		}
	}
}

// Before: the environment screens were reachable from the tab strip's menu only, so a person using the window's menu
// bar found no Manage environments. Each screen the menu opens has its action here as well as in the front end.
func TestBuildMenu_Given_the_environment_screens_Then_the_window_menu_names_their_actions(t *testing.T) {
	src, err := os.ReadFile("menu.go")
	if err != nil {
		t.Fatal(err)
	}

	for _, action := range []string{"envmanage", "envcompare"} {
		if !strings.Contains(string(src), `"`+action+`"`) {
			t.Errorf("menu.go has no %q action", action)
		}
	}
}
