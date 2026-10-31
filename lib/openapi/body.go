// SPDX-License-Identifier: Apache-2.0

package openapi

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/internal/jsonschema"
)

// maxBodyDepth bounds generated bodies (deeply nested schemas).
const maxBodyDepth = 4

// skeleton is a JSON value for a (resolved) schema: its example or
// default if it has one, else a template per field ("{{name}}" for
// strings, 0, false), arrays with one item and objects with required
// properties first.
func skeleton(schema any, name string, depth int) jsonValue {
	s, _ := schema.(map[string]any)
	if ex, ok := s["example"]; ok {
		return jsonValue{raw: ex}
	}
	if def, ok := s["default"]; ok {
		return jsonValue{raw: def}
	}
	s = jsonschema.MergeAllOf(s)
	switch jsonschema.Type(s) {
	case "object":
		if depth >= maxBodyDepth {
			return jsonValue{raw: map[string]any{}}
		}
		return objectSkeleton(s, depth)
	case "array":
		return jsonValue{list: []jsonValue{skeleton(s["items"], name, depth+1)}, isList: true}
	case "integer", "number":
		return jsonValue{raw: 0}
	case "boolean":
		return jsonValue{raw: false}
	}
	return jsonValue{raw: "{{" + ParamKey(name) + "}}"}
}

func objectSkeleton(s map[string]any, depth int) jsonValue {
	props, _ := s["properties"].(map[string]any)
	required := stringList(s["required"])
	var keys []string
	for _, k := range required {
		if _, ok := props[k]; ok {
			keys = append(keys, k)
		}
	}
	var rest []string
	for k := range props {
		if !slices.Contains(keys, k) {
			rest = append(rest, k)
		}
	}
	slices.Sort(rest)
	out := jsonValue{isObject: true}
	for _, k := range append(keys, rest...) {
		out.keys = append(out.keys, k)
		out.fields = append(out.fields, skeleton(props[k], k, depth+1))
	}
	return out
}

func stringList(v any) []string {
	l, _ := v.([]any)
	var out []string
	for _, x := range l {
		if s, ok := x.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// jsonValue is JSON with object keys in a chosen order.
type jsonValue struct {
	raw      any
	isObject bool
	keys     []string
	fields   []jsonValue
	isList   bool
	list     []jsonValue
}

// text renders v as JSON indented by two spaces.
func (v jsonValue) text() string {
	var b strings.Builder
	v.write(&b, "")
	return b.String()
}

func (v jsonValue) write(b *strings.Builder, indent string) {
	inner := indent + "  "
	switch {
	case v.isObject && len(v.keys) == 0:
		b.WriteString("{}")
	case v.isObject:
		b.WriteString("{\n")
		for i, k := range v.keys {
			key, _ := json.Marshal(k)
			b.WriteString(inner + string(key) + ": ")
			v.fields[i].write(b, inner)
			if i < len(v.keys)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + "}")
	case v.isList:
		b.WriteString("[\n")
		for i, item := range v.list {
			b.WriteString(inner)
			item.write(b, inner)
			if i < len(v.list)-1 {
				b.WriteString(",")
			}
			b.WriteString("\n")
		}
		b.WriteString(indent + "]")
	default:
		data, err := json.MarshalIndent(v.raw, indent, "  ")
		if err != nil {
			data = []byte("null")
		}
		b.Write(data)
	}
}
