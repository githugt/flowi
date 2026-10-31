// SPDX-License-Identifier: Apache-2.0

// Package snippet turns a previewed request (engine.PreviewRequest) into a
// command that sends it: cURL, wget, PowerShell Invoke-RestMethod or
// xh/HTTPie. The functions are pure; the caller decides which values the
// preview holds (secrets masked or not). Unresolved {{names}} are text and
// pass through untouched.
package snippet
