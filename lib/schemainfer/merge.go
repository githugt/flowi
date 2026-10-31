// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"slices"
)

// ChangeKind says how a later sample changed a schema.
type ChangeKind string

// The ways a schema changes when a sample does not fit it.
const (
	OptionalField ChangeKind = "optional-field" // a required field was missing from the sample
	NewField      ChangeKind = "new-field"      // the sample has a field the schema did not know
	TypeWidened   ChangeKind = "type-widened"   // integer became number
	NowNullable   ChangeKind = "now-nullable"   // null joined the types
)

// Change is one difference between a schema and what a sample showed.
type Change struct {
	Path   string // JSONPath-like place: $.user.email, $.items[]
	Kind   ChangeKind
	Detail string
}

// unite widens a with b: types are united (integer and number give number),
// objects keep every property and only those required by both, arrays unite
// their items. onChange, if not nil, is told each difference b made; path
// names the place for those reports.
func unite(a, b Schema, path string, onChange func(Change)) Schema {
	out := Schema{}
	types := uniteTypes(typesOf(a), typesOf(b), path, onChange)
	if len(types) == 1 {
		out["type"] = types[0]
	} else if len(types) > 1 {
		out["type"] = toAny(types)
	}
	if f, ok := a["format"]; ok && f == b["format"] {
		out["format"] = f
	}
	if props := uniteProperties(a, b, path, onChange); props != nil {
		out["properties"] = props
		if req := uniteRequired(a, b, path, onChange); len(req) > 0 {
			out["required"] = toAny(req)
		}
	}
	if items, ok := uniteItems(a, b, path, onChange); ok {
		out["items"] = items
	}
	for _, k := range []string{"x-flowi-truncated", "x-flowi-source"} {
		if v, ok := a[k]; ok {
			out[k] = v
		} else if v, ok := b[k]; ok {
			out[k] = v
		}
	}
	return out
}

func typesOf(s Schema) []string {
	switch t := s["type"].(type) {
	case string:
		return []string{t}
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func uniteTypes(a, b []string, path string, onChange func(Change)) []string {
	out := slices.Clone(a)
	for _, t := range b {
		if slices.Contains(out, t) {
			continue
		}
		switch {
		case t == "null":
			report(onChange, Change{Path: path, Kind: NowNullable, Detail: "null seen"})
		case t == "number" && slices.Contains(out, "integer"):
			out[slices.Index(out, "integer")] = "number"
			report(onChange, Change{Path: path, Kind: TypeWidened, Detail: "integer widened to number"})
			continue
		case t == "integer" && slices.Contains(out, "number"):
			continue // a whole number fits number
		case slices.Contains(a, "null") || len(out) > 0:
			report(onChange, Change{Path: path, Kind: TypeWidened, Detail: t + " added to the types"})
		}
		out = append(out, t)
	}
	if len(a) == 0 {
		return b
	}
	return dedupe(out)
}

func dedupe(in []string) []string {
	var out []string
	for _, t := range in {
		if !slices.Contains(out, t) {
			out = append(out, t)
		}
	}
	return out
}

func report(onChange func(Change), c Change) {
	if onChange != nil {
		onChange(c)
	}
}

func toAny(in []string) []any {
	out := make([]any, len(in))
	for i, s := range in {
		out[i] = s
	}
	return out
}

func propsOf(s Schema) Schema {
	p, _ := s["properties"].(Schema)
	if p == nil {
		p, _ = s["properties"].(map[string]any)
	}
	return p
}

func requiredOf(s Schema) []string {
	switch r := s["required"].(type) {
	case []string:
		return r
	case []any:
		out := make([]string, 0, len(r))
		for _, x := range r {
			if str, ok := x.(string); ok {
				out = append(out, str)
			}
		}
		return out
	}
	return nil
}

func uniteProperties(a, b Schema, path string, onChange func(Change)) Schema {
	pa, pb := propsOf(a), propsOf(b)
	if pa == nil && pb == nil {
		return nil
	}
	out := Schema{}
	for k, v := range pa {
		out[k] = v
	}
	for k, vb := range pb {
		child := path + "." + k
		if va, ok := pa[k]; ok {
			out[k] = unite(asSchema(va), asSchema(vb), child, onChange)
			continue
		}
		out[k] = vb
		if pa != nil {
			report(onChange, Change{Path: child, Kind: NewField, Detail: "field not seen before"})
		}
	}
	return out
}

func asSchema(v any) Schema {
	s, _ := v.(map[string]any)
	return s
}

// uniteRequired keeps a field required only when both schemas require it,
// and reports a field that stopped being required.
func uniteRequired(a, b Schema, path string, onChange func(Change)) []string {
	ra, rb := requiredOf(a), requiredOf(b)
	var out []string
	for _, k := range ra {
		if slices.Contains(rb, k) || propsOf(b) == nil {
			out = append(out, k)
		} else {
			report(onChange, Change{Path: path + "." + k, Kind: OptionalField, Detail: "missing from a later sample"})
		}
	}
	for _, k := range rb {
		if !slices.Contains(ra, k) && propsOf(a) != nil && propsOf(a)[k] != nil {
			report(onChange, Change{Path: path + "." + k, Kind: OptionalField, Detail: "missing from an earlier sample"})
		}
	}
	slices.Sort(out)
	return out
}

func uniteItems(a, b Schema, path string, onChange func(Change)) (Schema, bool) {
	ia, ib := asSchema(a["items"]), asSchema(b["items"])
	switch {
	case ia != nil && ib != nil:
		return unite(ia, ib, path+"[]", onChange), true
	case ia != nil:
		return ia, true
	case ib != nil:
		return ib, true
	}
	return nil, false
}

// Merge widens existing with what sample shows and reports each difference.
// A schema marked x-flowi-source openapi (imported from a spec) or user
// (edited by hand) is returned unchanged: inference must not rewrite what a
// person or a spec said.
func Merge(existing Schema, sample any) (Schema, []Change) {
	if src, _ := existing["x-flowi-source"].(string); src == "openapi" || src == "user" {
		return existing, nil
	}
	var changes []Change
	out := unite(existing, Infer(sample), "$", func(c Change) { changes = append(changes, c) })
	return out, changes
}
