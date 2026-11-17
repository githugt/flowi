//go:build linux && cgo && !gtk3 && !android && !server && !purego

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

/*
#cgo pkg-config: gtk4
#cgo CFLAGS: -Wno-deprecated-declarations
#include <gtk/gtk.h>

// drawMenuIcon draws the menu icon: two long lines and a short one, left aligned, in the button's text colour.
static void drawMenuIcon(GtkDrawingArea *area, cairo_t *cr, int width, int height, gpointer data) {
    GdkRGBA colour;
    gtk_widget_get_color(GTK_WIDGET(area), &colour);
    gdk_cairo_set_source_rgba(cr, &colour);
    cairo_set_line_width(cr, 1.6);
    cairo_set_line_cap(cr, CAIRO_LINE_CAP_ROUND);
    double left = (width - 12) / 2.0, top = (height - 9) / 2.0;
    cairo_move_to(cr, left, top);            cairo_line_to(cr, left + 12, top);
    cairo_move_to(cr, left, top + 4.5);      cairo_line_to(cr, left + 12, top + 4.5);
    cairo_move_to(cr, left, top + 9);        cairo_line_to(cr, left + 6, top + 9);
    cairo_stroke(cr);
}

static void redrawOnState(GtkWidget *widget, GtkStateFlags flags, gpointer data) {
    gtk_widget_queue_draw(GTK_WIDGET(data));
}

// useMenuIcon shows the drawn icon in place of the theme's three-line one.
static void useMenuIcon(GtkWidget *button) {
    GtkWidget *area = gtk_drawing_area_new();
    gtk_drawing_area_set_content_width(GTK_DRAWING_AREA(area), 16);
    gtk_drawing_area_set_content_height(GTK_DRAWING_AREA(area), 16);
    gtk_drawing_area_set_draw_func(GTK_DRAWING_AREA(area), drawMenuIcon, NULL, NULL);
    g_signal_connect(button, "state-flags-changed", G_CALLBACK(redrawOnState), area);
    gtk_menu_button_set_child(GTK_MENU_BUTTON(button), area);
}

// moveMenuButtonToStart moves the main-menu button (and gives it a drawn icon) of the window's header bar to the start (left) of the bar.
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
            useMenuIcon(child);
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

// menuButtonToStart puts the main-menu button of the Linux header bar, with its own icon, on the left, where the window's own menu
// usually is, instead of beside the window buttons on the right. It waits for the window to exist.
func menuButtonToStart(window *application.WebviewWindow) {
	window.RegisterHook(events.Common.WindowRuntimeReady, func(*application.WindowEvent) {
		application.InvokeAsync(func() { C.moveMenuButtonToStart(window.NativeWindow()) })
	})
}
