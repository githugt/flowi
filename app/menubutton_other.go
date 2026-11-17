//go:build !(linux && cgo && !gtk3 && !android && !server && !purego)

// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import "github.com/wailsapp/wails/v3/pkg/application"

// menuButtonToStart is for the Linux header bar only; nothing to move elsewhere.
func menuButtonToStart(*application.WebviewWindow) {}
