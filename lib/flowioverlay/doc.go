// SPDX-License-Identifier: Apache-2.0

// Package flowioverlay applies flowi overlays to request versions, works out
// the overlay that turns one version into an edited copy, and reads and
// writes overlay files.
//
// A flowi overlay is not an OpenAPI Overlay (see package specoverlay): that
// patches an API description once, at import; this patches a request at send
// time and can set any field a request has.
package flowioverlay
