// SPDX-License-Identifier: Apache-2.0

// Package schemainfer infers JSON Schema (2020-12) from sample bodies, widens
// a schema with later samples while reporting what changed, and validates
// samples against a schema. It works on decoded JSON values (maps, slices,
// strings, numbers, bools, nil), so it does not care where a body came from.
package schemainfer

import (
	"encoding/json"
	"math"
	"net/mail"
	"net/url"
	"slices"
	"strings"
	"time"
)

// Schema is a JSON Schema object as decoded JSON.
type Schema = map[string]any

// Bounds on what Infer reads, so a huge or deeply nested body costs a bounded
// amount of work. A body past them gives a schema marked x-flowi-truncated.
const (
	MaxDepth = 20
	MaxNodes = 10000
)

// Infer describes one sample: types, nested properties (every present key is
// required), array items (the union of the elements) and string formats
// (date-time, uuid, email, uri). Numbers decoded as json.Number are integers
// when written without a fraction or exponent; a float64 counts as an
// integer when it is whole, because decoding cannot tell 132 from 132.0.
func Infer(sample any) Schema {
	b := &inferrer{nodes: MaxNodes}
	s := b.infer(sample, 0)
	if b.truncated {
		s["x-flowi-truncated"] = true
	}
	return s
}

type inferrer struct {
	nodes     int
	truncated bool
}

func (b *inferrer) infer(v any, depth int) Schema {
	b.nodes--
	if depth > MaxDepth || b.nodes < 0 {
		b.truncated = true
		return Schema{}
	}
	switch x := v.(type) {
	case nil:
		return Schema{"type": "null"}
	case bool:
		return Schema{"type": "boolean"}
	case string:
		return stringSchema(x)
	case json.Number:
		return Schema{"type": numberType(x)}
	case float64:
		return Schema{"type": floatType(x)}
	case int, int64:
		return Schema{"type": "integer"}
	case []any:
		return b.array(x, depth)
	case map[string]any:
		return b.object(x, depth)
	}
	return Schema{}
}

func numberType(n json.Number) string {
	if strings.ContainsAny(string(n), ".eE") {
		return "number"
	}
	return "integer"
}

func floatType(f float64) string {
	if f == math.Trunc(f) && !math.IsInf(f, 0) {
		return "integer"
	}
	return "number"
}

func (b *inferrer) array(items []any, depth int) Schema {
	s := Schema{"type": "array"}
	var union Schema
	for _, it := range items {
		inferred := b.infer(it, depth+1)
		if union == nil {
			union = inferred
			continue
		}
		union = unite(union, inferred, "$[]", nil)
	}
	if union != nil {
		s["items"] = union
	}
	return s
}

func (b *inferrer) object(m map[string]any, depth int) Schema {
	props := Schema{}
	keys := make([]string, 0, len(m))
	for k, v := range m {
		props[k] = b.infer(v, depth+1)
		keys = append(keys, k)
	}
	slices.Sort(keys)
	s := Schema{"type": "object", "properties": props}
	if len(keys) > 0 {
		s["required"] = toAny(keys) // decoded-JSON shapes only: the validator rejects []string
	}
	return s
}

// stringSchema is a string schema with the format the value looks like.
func stringSchema(v string) Schema {
	s := Schema{"type": "string"}
	if f := formatOf(v); f != "" {
		s["format"] = f
	}
	return s
}

func formatOf(v string) string {
	switch {
	case isUUID(v):
		return "uuid"
	case isDateTime(v):
		return "date-time"
	case isEmail(v):
		return "email"
	case isURI(v):
		return "uri"
	}
	return ""
}

func isDateTime(v string) bool {
	_, err := time.Parse(time.RFC3339, v)
	return err == nil
}

// isUUID is the 8-4-4-4-12 hexadecimal form.
func isUUID(v string) bool {
	if len(v) != 36 {
		return false
	}
	for i, r := range v {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !strings.ContainsRune("0123456789abcdefABCDEF", r) {
				return false
			}
		}
	}
	return true
}

// isEmail wants a plain address with a dotted domain; a display name or a
// bare local@host is not one.
func isEmail(v string) bool {
	a, err := mail.ParseAddress(v)
	if err != nil || a.Address != v {
		return false
	}
	_, domain, _ := strings.Cut(v, "@")
	return strings.Contains(domain, ".")
}

func isURI(v string) bool {
	u, err := url.Parse(v)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}
