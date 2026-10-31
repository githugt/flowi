// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// DriftKind says how a response differs from the schema learned from earlier ones.
type DriftKind string

// Kinds of drift.
const (
	DriftNew     DriftKind = "new"     // a field the schema does not know
	DriftMissing DriftKind = "missing" // a required field that is gone
	DriftChanged DriftKind = "changed" // a field of another type
)

// Difference is one place a response differs from its schema.
type Difference struct {
	Path   string // $.owner.email; list items are [*]
	Kind   DriftKind
	Detail string
}

// Drift compares a response with the schema stored for it and lists what changed: fields the schema does not
// know (new), required fields that are gone (missing), and fields of another type (changed). It is the other
// side of Validate: it does not judge the response, it says what moved, and it never widens the schema. A field
// the schema allows to be null or absent is not drift when it is. A whole number where the schema says number
// is not drift either. A place that appears many times (every item of a list) is listed once.
func Drift(schema Schema, sample any) []Difference {
	var out []Difference
	seen := map[string]bool{}
	add := func(d Difference) {
		if key := string(d.Kind) + " " + d.Path; !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	walkDrift(schema, sample, "$", add)
	slices.SortFunc(out, func(a, b Difference) int {
		if c := strings.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
	return out
}

func walkDrift(schema Schema, sample any, path string, add func(Difference)) {
	want := typesOf(schema)
	if len(want) == 0 {
		return // the schema says nothing about this place: there is nothing to compare
	}
	got := sampleType(sample)
	if !typeFits(got, want) {
		add(Difference{Path: path, Kind: DriftChanged, Detail: fmt.Sprintf("was %s, now %s", strings.Join(want, " or "), got)})
		return
	}
	switch x := sample.(type) {
	case map[string]any:
		driftObject(schema, x, path, add)
	case []any:
		items := asSchema(schema["items"])
		for _, el := range x {
			if items != nil {
				walkDrift(items, el, path+"[*]", add)
			}
		}
	}
}

func driftObject(schema Schema, sample map[string]any, path string, add func(Difference)) {
	props := propsOf(schema)
	for _, key := range slices.Sorted(maps.Keys(sample)) {
		sub, known := props[key]
		if !known {
			add(Difference{Path: path + "." + key, Kind: DriftNew})
			continue
		}
		if s := asSchema(sub); s != nil {
			walkDrift(s, sample[key], path+"."+key, add)
		}
	}
	for _, key := range requiredOf(schema) {
		if _, present := sample[key]; !present {
			add(Difference{Path: path + "." + key, Kind: DriftMissing})
		}
	}
}

// sampleType names the JSON type of a decoded value. A whole number is an integer.
func sampleType(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case string:
		return "string"
	case json.Number:
		return numberType(x)
	case float64:
		return floatType(x)
	case int, int64:
		return "integer"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	}
	return "unknown"
}

// typeFits says whether a sample's type is one the schema allows. An integer is a number.
func typeFits(got string, want []string) bool {
	return slices.Contains(want, got) || got == "integer" && slices.Contains(want, "number")
}
