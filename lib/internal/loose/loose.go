// SPDX-License-Identifier: Apache-2.0

// Package loose reads decoded JSON or YAML as plain maps and lists, so a
// field another tool adds in a later version does not stop an import.
package loose

import (
	"fmt"
	"slices"
	"strconv"
)

// Map is v as an object, nil when it is not one.
func Map(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// List is the objects of v; anything else in the list is dropped.
func List(v any) []map[string]any {
	items, _ := v.([]any)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

// Str is a scalar as text ("" for nothing); numbers and booleans as written.
func Str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int, int64, uint64:
		return fmt.Sprint(t)
	}
	return ""
}

// Flag is v as a boolean, false when it is not one.
func Flag(v any) bool {
	b, _ := v.(bool)
	return b
}

// SortedKeys are the keys of m in order, so output does not depend on map order.
func SortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
