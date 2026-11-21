// SPDX-License-Identifier: Apache-2.0

// Package snippet turns a previewed request (engine.PreviewRequest) into a
// command or code that sends it: cURL, wget, PowerShell Invoke-RestMethod,
// xh/HTTPie, a .http file, JavaScript fetch or Python requests. The functions
// are pure; the caller decides which values the preview holds (secrets masked or not). Unresolved {{names}} are text and
// pass through untouched.
package snippet
