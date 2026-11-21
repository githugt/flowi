// SPDX-License-Identifier: Apache-2.0

// Package yamldoc decodes YAML documents (OpenAPI, Arazzo) into plain
// map[string]any, []any and scalars, for transforms that walk them by key.
package yamldoc

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/internal/textenc"
)

// maxNodes is how many values a document may expand to once its aliases are followed. A real OpenAPI or Arazzo file is
// far below it; a "billion laughs" file is not.
const maxNodes = 2_000_000

// ErrTooLarge means the document expands, through its aliases, to more than maxNodes values.
var ErrTooLarge = errors.New("the document expands to too many values (a YAML alias is used to repeat a large structure)")

// Decode parses data and normalises it; the root must be a mapping. Before: aliases were expanded without a limit,
// so a 307-byte document with nested aliases took 370 ms at 7 levels and each level multiplied the cost by nine.
func Decode(data []byte) (map[string]any, error) {
	data = textenc.ToUTF8(data)
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	n := &normalizer{budget: maxNodes}
	norm := n.normalize(raw)
	if n.budget < 0 {
		return nil, ErrTooLarge
	}
	root, ok := norm.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the document is not a mapping")
	}
	return root, nil
}

// Normalize turns decoded YAML into map[string]any, []any and scalars.
func Normalize(v any) any {
	return (&normalizer{budget: 1 << 62}).normalize(v)
}

// normalizer copies a decoded document and counts the values it copies; it stops copying when the budget is used up
// so a document that expands without end costs no more than the budget.
type normalizer struct{ budget int }

func (n *normalizer) normalize(v any) any {
	if n.budget < 0 {
		return nil
	}
	n.budget--
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = n.normalize(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[fmt.Sprint(k)] = n.normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = n.normalize(item)
		}
		return out
	}
	return v
}
