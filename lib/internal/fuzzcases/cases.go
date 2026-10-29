// SPDX-License-Identifier: Apache-2.0

// Package fuzzcases makes bad inputs from a good one: the same body with one thing wrong at a time, in the way
// property-based API testers do. The order is fixed, so a case that broke a server can be found again.
package fuzzcases

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Case is a body with one thing wrong, and a name that says what.
type Case struct {
	Name string
	Body any
}

// Cases returns at most limit cases made from base (a decoded JSON body, numbers as json.Number). schema is the
// body's stored JSON Schema, or nil: it adds a case just past each bound it states. The cases are spread over
// the fields first, so a small limit still touches every field once before it touches any twice.
func Cases(base any, schema map[string]any, limit int) []Case {
	byPath := map[string][]Case{}
	var order []string
	add := func(path []any, what string, mutate func(any) (any, bool)) {
		body, ok := editAt(base, path, mutate)
		if !ok {
			return
		}
		key := pathText(path)
		if _, seen := byPath[key]; !seen {
			order = append(order, key)
		}
		byPath[key] = append(byPath[key], Case{Name: key + ": " + what, Body: body})
	}
	walk(base, schema, nil, add)
	if _, isObject := base.(map[string]any); isObject {
		key := "(body)"
		order = append(order, key)
		extra := deepCopy(base).(map[string]any)
		extra["__flowi_unknown__"] = json.Number("1")
		byPath[key] = []Case{{Name: key + ": an unknown field", Body: extra}}
	}
	return interleave(order, byPath, limit)
}

// interleave takes one case from each path in turn until limit cases are taken or none are left.
func interleave(order []string, byPath map[string][]Case, limit int) []Case {
	var out []Case
	for round := 0; len(out) < limit; round++ {
		took := false
		for _, key := range order {
			if cs := byPath[key]; round < len(cs) && len(out) < limit {
				out = append(out, cs[round])
				took = true
			}
		}
		if !took {
			break
		}
	}
	return out
}

// walk proposes mutations for every value under v. add is called with the value's path, a description and the
// edit, and records the case when the edit applies.
func walk(v any, schema map[string]any, path []any, add func(path []any, what string, mutate func(any) (any, bool))) {
	set := func(what string, value any) { add(path, what, func(any) (any, bool) { return value, true }) }
	switch x := v.(type) {
	case map[string]any:
		if len(path) > 0 {
			set("null", nil)
			set("a list instead of an object", []any{})
			set("empty object", map[string]any{})
		}
		props, _ := schema["properties"].(map[string]any)
		for _, key := range slices.Sorted(maps.Keys(x)) {
			child, _ := props[key].(map[string]any)
			walk(x[key], child, append(slices.Clone(path), key), add)
			childPath := append(slices.Clone(path), key)
			add(childPath, "removed", func(any) (any, bool) { return removeMarker{}, true })
		}
	case []any:
		set("null", nil)
		set("an object instead of a list", map[string]any{})
		set("empty list", []any{})
		if len(x) > 0 {
			items, _ := schema["items"].(map[string]any)
			walk(x[0], items, append(slices.Clone(path), 0), add)
		}
	case string:
		stringCases(set, schema)
	case json.Number, float64, int, int64:
		numberCases(set, schema)
	case bool:
		set("null", nil)
		set("text instead of true or false", "true")
	case nil:
		set("a value where null was", "x")
	}
}

func stringCases(set func(string, any), schema map[string]any) {
	set("null", nil)
	set("a number instead of text", json.Number("12345"))
	set("empty text", "")
	set("very long text", strings.Repeat("a", 4096))
	set("SQL quote", "x'; DROP TABLE users;--")
	set("markup", "<script>alert(1)</script>")
	set("path traversal", "../../../../etc/passwd")
	set("header injection", "a\r\nX-Injected: 1")
	set("odd unicode", "\u202e\U0001F4A9\x00 a b")
	if limit, ok := number(schema["maxLength"]); ok {
		set("longer than the maximum", strings.Repeat("a", int(limit)+1))
	}
	if enum, ok := schema["enum"].([]any); ok && len(enum) > 0 {
		set("not one of the allowed values", "__not_an_allowed_value__")
	}
}

func numberCases(set func(string, any), schema map[string]any) {
	set("null", nil)
	set("text instead of a number", "abc")
	set("zero", json.Number("0"))
	set("negative", json.Number("-1"))
	set("past 32 bits", json.Number("4294967296"))
	if lowest, ok := number(schema["minimum"]); ok {
		set("below the minimum", json.Number(fmt.Sprint(int64(lowest)-1)))
	}
	if highest, ok := number(schema["maximum"]); ok {
		set("above the maximum", json.Number(fmt.Sprint(int64(highest)+1)))
	}
}

func number(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	case int:
		return float64(x), true
	}
	return 0, false
}

// removeMarker asks editAt to delete the value instead of replacing it.
type removeMarker struct{}

// editAt returns a copy of base with the value at path replaced by what mutate returns. ok is false when mutate
// is nil or the path is not there.
func editAt(base any, path []any, mutate func(any) (any, bool)) (any, bool) {
	if mutate == nil {
		return nil, false
	}
	if len(path) == 0 {
		v, ok := mutate(base)
		if _, remove := v.(removeMarker); remove {
			return nil, false
		}
		return v, ok
	}
	out := deepCopy(base)
	parent := out
	for _, step := range path[:len(path)-1] {
		parent = child(parent, step)
		if parent == nil {
			return nil, false
		}
	}
	last := path[len(path)-1]
	v, ok := mutate(child(parent, last))
	if !ok {
		return nil, false
	}
	switch p := parent.(type) {
	case map[string]any:
		key := last.(string)
		if _, remove := v.(removeMarker); remove {
			delete(p, key)
		} else {
			p[key] = v
		}
	case []any:
		p[last.(int)] = v
	}
	return out, true
}

func child(parent any, step any) any {
	switch p := parent.(type) {
	case map[string]any:
		if key, ok := step.(string); ok {
			return p[key]
		}
	case []any:
		if i, ok := step.(int); ok && i < len(p) {
			return p[i]
		}
	}
	return nil
}

func deepCopy(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = deepCopy(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, val := range x {
			out[i] = deepCopy(val)
		}
		return out
	}
	return v
}

// pathText writes a path the way a person reads it: owner.email, tags[0]. The whole body is "body".
func pathText(path []any) string {
	if len(path) == 0 {
		return "body"
	}
	var sb strings.Builder
	for i, step := range path {
		switch s := step.(type) {
		case string:
			if i > 0 {
				sb.WriteString(".")
			}
			sb.WriteString(s)
		case int:
			fmt.Fprintf(&sb, "[%d]", s)
		}
	}
	return sb.String()
}
