// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// LayerValues are the validated values one layer sets. Source names the
// file or variable they came from.
type LayerValues struct {
	Layer  Layer
	Source string
	Values model.Settings
}

// FromBlock validates a flattened settings block read from a file at the
// given layer. Unknown keys warn and are dropped; invalid values and keys
// not allowed at the layer are errors. Diagnostic paths point into the
// file: "$.<key>" for the user settings file, "$.settings.<key>" elsewhere.
func FromBlock(schema []Key, layer Layer, source string, raw map[string]any) (LayerValues, diag.List) {
	lv := LayerValues{Layer: layer, Source: source, Values: model.Settings{}}
	var diags diag.List
	for _, name := range sortedKeys(raw) {
		loc := diag.Location{File: source, Path: blockPath(layer, name)}
		k, ok := Lookup(schema, name)
		if !ok {
			diags = append(diags, diag.New(diag.CodeSettingUnknown, loc, map[string]string{"key": name}))
			continue
		}
		if !slices.Contains(k.FileLayers, layer) {
			diags = append(diags, diag.New(diag.CodeSettingWrongLayer, loc, map[string]string{"key": name, "layer": layer.String()}))
			continue
		}
		v, err := ParseValue(k, raw[name])
		if err != nil {
			diags = append(diags, diag.New(diag.CodeSettingInvalid, loc, map[string]string{"key": name, "detail": err.Error()}))
			continue
		}
		lv.Values[name] = v
	}
	return lv, diags
}

func blockPath(layer Layer, name string) string {
	if layer == LayerUser {
		return "$." + name
	}
	return "$.settings." + name
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// Value is a resolved setting and where it came from.
type Value struct {
	Value  any
	Layer  Layer
	Source string
}

// Resolved holds every layer for lookup and explanation.
type Resolved struct {
	schema []Key
	layers []LayerValues // highest precedence first
}

// Resolve combines layers; their order in the call does not matter. Built-in
// defaults are always the lowest layer.
func Resolve(schema []Key, layers ...LayerValues) Resolved {
	sorted := slices.Clone(layers)
	slices.SortStableFunc(sorted, func(a, b LayerValues) int { return int(b.Layer) - int(a.Layer) })
	return Resolved{schema: schema, layers: sorted}
}

// Get returns the winning value of a setting.
func (r Resolved) Get(name string) (Value, bool) {
	all := r.Explain(name)
	if len(all) == 0 {
		return Value{}, false
	}
	return all[0], true
}

// Explain returns every layer that sets the key, highest first, ending with
// the built-in default.
func (r Resolved) Explain(name string) []Value {
	k, ok := Lookup(r.schema, name)
	if !ok {
		return nil
	}
	var out []Value
	for _, lv := range r.layers {
		if v, set := lv.Values[name]; set {
			out = append(out, Value{Value: v, Layer: lv.Layer, Source: lv.Source})
		}
	}
	def, err := ParseValue(k, k.Default)
	if err == nil { // schema defaults are covered by tests; a bad one is simply not listed
		out = append(out, Value{Value: def, Layer: LayerDefault, Source: "built-in"})
	}
	return out
}
