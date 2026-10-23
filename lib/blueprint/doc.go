// SPDX-License-Identifier: Apache-2.0

// Package blueprint turns blueprints into flows and flows into blueprints.
//
// A blueprint (design §4.8) is a flow body with declared parameters. Request
// and subflow references in its nodes may be {param: name}; the other
// parameters (string, int, bool) become inputs of the flow made from it.
// Using a blueprint copies it into an ordinary flow; nothing links the flow
// back to the blueprint.
//
// Everything here is pure: references are looked up through a Resolver the
// caller provides and ids come from a function the caller provides.
package blueprint
