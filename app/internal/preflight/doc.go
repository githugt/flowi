// SPDX-License-Identifier: AGPL-3.0-or-later

// Package preflight checks, before the window opens, that a Linux system has what WebKitGTK needs to start its web
// content: the helper programs it spawns, and the sandbox tools bubblewrap and xdg-dbus-proxy. Without them the window
// dies with a signal instead of an error. The check says what is missing and the command that installs it on this
// distribution.
//
// A missing shared library cannot be caught here: the dynamic loader stops the program before main runs. The
// distribution packages depend on those libraries, so the package manager installs them.
package preflight
