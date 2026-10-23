// SPDX-License-Identifier: Apache-2.0

package model

// Point is a canvas position in canvas units.
type Point struct{ X, Y int64 }

// Layout is a flow's *.layout.yaml: canvas positions only, kept apart from
// the flow so moving nodes never changes the flow file (design §4.7).
type Layout struct {
	Flow      ID
	Nodes     map[string]Point // manual positions by node id
	Collapsed []string         // collapsed action strips, by first node id
	File      string
}
