// SPDX-License-Identifier: Apache-2.0

// Package yamldoc decodes YAML documents (OpenAPI, Arazzo) into plain
// map[string]any, []any and scalars, for transforms that walk them by key.
package yamldoc

import (
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/internal/textenc"
)

// Decode parses data and normalises it; the root must be a mapping.
func Decode(data []byte) (map[string]any, error) {
	data = textenc.ToUTF8(data)
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	root, ok := Normalize(raw).(map[string]any)
	if !ok {
		return nil, fmt.Errorf("the document is not a mapping")
	}
	return root, nil
}

// Normalize turns decoded YAML into map[string]any, []any and scalars.
func Normalize(v any) any {
	switch x := v.(type) {
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[k] = Normalize(item)
		}
		return out
	case map[any]any:
		out := make(map[string]any, len(x))
		for k, item := range x {
			out[fmt.Sprint(k)] = Normalize(item)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i, item := range x {
			out[i] = Normalize(item)
		}
		return out
	}
	return v
}
