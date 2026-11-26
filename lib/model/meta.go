// SPDX-License-Identifier: Apache-2.0

// Package model defines Flowi's domain types as plain data. It performs no
// I/O and holds no behaviour beyond small structural helpers; storage,
// validation and execution live in other packages.
package model

import "fmt"

// ID is a ULID in canonical upper-case form.
type ID string

// Meta is the identity shared by every entity.
type Meta struct {
	ID          ID
	Name        string
	Description string
}

// KeyValue is an ordered string pair (headers, query parameters, inputs).
type KeyValue struct {
	Key   string
	Value string
	// Description says what the entry is for; only requests keep it (query,
	// headers, variables).
	Description string
	// Disabled keeps the row in the file but out of what is sent: a query parameter or header switched off to try
	// a request without it. Only query parameters and headers of a request carry it.
	Disabled bool
}

// Field is an ordered key with an arbitrary YAML value: string, bool,
// int64, float64, []any, []Field or nil.
type Field struct {
	Key   string
	Value any
}

// Settings maps dotted setting keys ("http.max_redirects") to values that
// the settings package has already validated.
type Settings map[string]any

// Ref points at a request or flow. Collection is empty for the referencing
// collection itself, otherwise the name or id of a declared dependency.
// ID is authoritative; Path is a readable hint that tools repair.
type Ref struct {
	Collection string
	ID         ID
	Path       string
	// Param, set only inside a blueprint, names the blueprint parameter
	// that fills this reference when the blueprint is used; ID and Path are
	// then empty.
	Param string
}

// Label says which request or flow r points at for a person reading a message: the path the file remembers, which is
// what the sidebar shows, then the id ("auth/login" (id 01J…)); just the id when the file has no path hint.
func (r Ref) Label() string {
	if r.Path == "" {
		return "id " + string(r.ID)
	}
	return fmt.Sprintf("%q (id %s)", r.Path, r.ID)
}

// HookStep is one pre or post hook: a reusable flow or inline nodes.
// Exactly one of Flow and Inline is set.
type HookStep struct {
	Flow   *Ref
	Inline []Node
}

// Hooks are the steps attached to an API at some scope: Pre runs before the
// request is sent and Post after its response. Message runs for each message
// a message request keeps, as it arrives, in the order Pre does; requests that
// receive no messages never run it.
type Hooks struct {
	Pre     []HookStep
	Post    []HookStep
	Message []HookStep
}
