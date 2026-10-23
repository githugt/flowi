// SPDX-License-Identifier: Apache-2.0

// Package jsonschema reads the JSON Schemas Flow-I keeps from OpenAPI
// documents (already decoded, references inlined): types, allOf merging,
// and whether a path of steps exists in a schema.
package jsonschema

import (
	"strconv"
)

// Result is what a schema says about a path.
type Result int

// Results.
const (
	Present Result = iota // the schema has it
	Absent                // the schema rules it out
	Unknown               // the schema allows anything there
)

// Type is a schema's type; OpenAPI 3.1 type lists give their first
// non-null entry, and a schema with properties is an object.
func Type(s map[string]any) string {
	switch t := s["type"].(type) {
	case string:
		return t
	case []any:
		for _, x := range t {
			if str, ok := x.(string); ok && str != "null" {
				return str
			}
		}
	}
	if _, ok := s["properties"]; ok {
		return "object"
	}
	if _, ok := s["items"]; ok {
		return "array"
	}
	return ""
}

// MergeAllOf folds allOf branches into one schema (properties and
// required combined); other keys of the outer schema win.
func MergeAllOf(s map[string]any) map[string]any {
	parts, ok := s["allOf"].([]any)
	if !ok {
		return s
	}
	outer := map[string]any{} // the schema's own keys, without allOf (or merging it would recurse forever)
	for k, v := range s {
		if k != "allOf" {
			outer[k] = v
		}
	}
	out := map[string]any{}
	props := map[string]any{}
	var required []any
	for _, p := range append(parts, any(outer)) {
		m, _ := p.(map[string]any)
		for k, v := range MergeAllOf(m) {
			switch k {
			case "properties":
				pm, _ := v.(map[string]any)
				for pk, pv := range pm {
					props[pk] = pv
				}
			case "required":
				rl, _ := v.([]any)
				required = append(required, rl...)
			case "allOf":
			default:
				out[k] = v
			}
		}
	}
	if len(props) > 0 {
		out["properties"] = props
		if _, typed := out["type"]; !typed {
			out["type"] = "object"
		}
	}
	if len(required) > 0 {
		out["required"] = required
	}
	return out
}

// Lookup says whether the path of steps (object keys, array indexes)
// exists in schema. A property not listed is Absent even when
// additionalProperties is not false: the contract is what the schema
// documents. An untyped {} schema, an object listing no properties, and
// additionalProperties as a schema or true are Unknown. anyOf/oneOf
// branches are judged together with the properties beside them.
func Lookup(schema any, steps []string) Result {
	s, _ := schema.(map[string]any)
	s = MergeAllOf(s)
	if len(steps) == 0 {
		return Present
	}
	if branches, ok := alternatives(s); ok {
		return best(withBase(s, branches), steps)
	}
	switch Type(s) {
	case "object":
		return lookupProperty(s, steps)
	case "array":
		if _, err := strconv.Atoi(steps[0]); err != nil {
			return Absent
		}
		return Lookup(s["items"], steps[1:])
	case "":
		return Unknown
	}
	return Absent // a scalar with steps left
}

func lookupProperty(s map[string]any, steps []string) Result {
	props, _ := s["properties"].(map[string]any)
	if child, ok := props[steps[0]]; ok {
		return Lookup(child, steps[1:])
	}
	switch extra := s["additionalProperties"].(type) {
	case bool:
		if extra {
			return Unknown
		}
		return Absent
	case map[string]any:
		if len(extra) > 0 {
			return Unknown
		}
	}
	if len(props) == 0 {
		return Unknown // a free-form object documents no fields to rely on or miss
	}
	return Absent
}

// withBase merges the schema around anyOf/oneOf into each branch.
func withBase(s map[string]any, branches []any) []any {
	base := map[string]any{}
	for k, v := range s {
		if k != "anyOf" && k != "oneOf" {
			base[k] = v
		}
	}
	out := make([]any, len(branches))
	for i, b := range branches {
		out[i] = MergeAllOf(map[string]any{"allOf": []any{base, b}})
	}
	return out
}

func alternatives(s map[string]any) ([]any, bool) {
	for _, k := range []string{"anyOf", "oneOf"} {
		if l, ok := s[k].([]any); ok && len(l) > 0 {
			return l, true
		}
	}
	return nil, false
}

// best is the most favourable result over alternative schemas.
func best(branches []any, steps []string) Result {
	out := Absent
	for _, b := range branches {
		switch Lookup(b, steps) {
		case Present:
			return Present
		case Unknown:
			out = Unknown
		}
	}
	return out
}
