// SPDX-License-Identifier: Apache-2.0

package model

// Operation verbs of a flowi overlay.
const (
	OverlaySet    = "set"    // give a field or entry a value
	OverlayRemove = "remove" // take an entry away
	OverlayAdd    = "add"    // append a line to a list (assert)
	OverlayMerge  = "merge"  // merge-patch a JSON body
)

// OverlayOp is one change a flowi overlay makes to a request version. Path
// says where (url, headers.X-Tier, body.json, assert, ...); Was is what the
// base held when the overlay was saved, used to report drift later.
type OverlayOp struct {
	Verb  string
	Path  string
	Value any
	Was   any
}

// RequestOverlay is the part of an overlay that only one request needs.
type RequestOverlay struct {
	Request ID
	// Path is a readable hint at the request; Request is authoritative.
	Path string
	Ops  []OverlayOp
}

// Overlay is a flowi overlay: a named set of changes laid over a request
// version at send time (the same request as a VIP customer, a delayed
// request, one with a bad token). Shared applies to every request that uses
// the overlay; each RequestOverlay applies to its request after that.
type Overlay struct {
	Name        string
	Description string
	Shared      []OverlayOp
	Requests    []RequestOverlay
	File        string
}
