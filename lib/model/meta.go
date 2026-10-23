// SPDX-License-Identifier: Apache-2.0

// Package model defines Flow-I's domain types as plain data. It performs no
// I/O and holds no behaviour beyond small structural helpers; storage,
// validation and execution live in other packages.
package model

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

// HookStep is one pre or post hook: a reusable flow or inline nodes.
// Exactly one of Flow and Inline is set.
type HookStep struct {
	Flow   *Ref
	Inline []Node
}

// Hooks are the pre and post steps attached to an API at some scope.
type Hooks struct {
	Pre  []HookStep
	Post []HookStep
}
