//go:build linux && cgo && !gtk3 && !android && !server && !purego

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo pkg-config: gtk4
#include <gtk/gtk.h>

// moveMenuButtonToStart moves the main-menu button of the window's header bar to the start (left) of the bar.
// Wails packs it at the end, beside the minimize, maximize and close buttons. It does nothing when the window
// has no header bar or no such button.
static void moveMenuButtonToStart(void *window) {
    GtkWidget *bar = gtk_window_get_titlebar(GTK_WINDOW(window));
    if (bar == NULL || !GTK_IS_HEADER_BAR(bar)) return;
    for (GtkWidget *child = gtk_widget_get_first_child(bar); child != NULL; child = gtk_widget_get_next_sibling(child)) {
        if (GTK_IS_MENU_BUTTON(child)) {
            g_object_ref(child);
            gtk_header_bar_remove(GTK_HEADER_BAR(bar), child);
            gtk_header_bar_pack_start(GTK_HEADER_BAR(bar), child);
            g_object_unref(child);
            return;
        }
    }
}
*/
import "C"

import (
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// menuButtonToStart puts the main-menu button of the Linux header bar on the left, where the window's own menu
// usually is, instead of beside the window buttons on the right. It waits for the window to exist.
func menuButtonToStart(window *application.WebviewWindow) {
	window.RegisterHook(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		application.InvokeAsync(func() { C.moveMenuButtonToStart(window.NativeWindow()) })
	})
}
