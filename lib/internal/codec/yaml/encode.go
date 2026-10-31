// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"strings"
	"time"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/model"
)

// omap builds an ordered YAML mapping, skipping empty values so canonical
// files contain only what was set.
type omap struct{ items yaml.MapSlice }

func (m *omap) put(key string, value any) {
	if isEmpty(value) {
		return
	}
	m.items = append(m.items, yaml.MapItem{Key: key, Value: value})
}

// putAlways writes the value even when it is empty or zero.
func (m *omap) putAlways(key string, value any) {
	m.items = append(m.items, yaml.MapItem{Key: key, Value: value})
}

func isEmpty(v any) bool {
	switch x := v.(type) {
	case nil:
		return true
	case string:
		return x == ""
	case bool:
		return !x
	case int64:
		return x == 0
	case int:
		return x == 0
	case []any:
		return len(x) == 0
	case []string:
		return len(x) == 0
	case yaml.MapSlice:
		return len(x) == 0
	}
	return false
}

func keyValues(kvs []model.KeyValue) yaml.MapSlice {
	var out yaml.MapSlice
	for _, kv := range kvs {
		out = append(out, yaml.MapItem{Key: kv.Key, Value: kv.Value})
	}
	return out
}

func fieldsToYAML(fields []model.Field) yaml.MapSlice {
	var out yaml.MapSlice
	for _, f := range fields {
		out = append(out, yaml.MapItem{Key: f.Key, Value: valueToYAML(f.Value)})
	}
	return out
}

// multipartToYAML writes the parts as a mapping, or, when a name repeats, as a
// list of one-entry mappings: a mapping cannot hold a key twice.
func multipartToYAML(parts []model.Field) any {
	seen := map[string]bool{}
	repeated := false
	for _, p := range parts {
		repeated = repeated || seen[p.Key]
		seen[p.Key] = true
	}
	if !repeated {
		return fieldsToYAML(parts)
	}
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		out = append(out, fieldsToYAML([]model.Field{p}))
	}
	return out
}

func valueToYAML(v any) any {
	switch x := v.(type) {
	case []model.Field:
		return fieldsToYAML(x)
	case []any:
		out := make([]any, 0, len(x))
		for _, item := range x {
			out = append(out, valueToYAML(item))
		}
		return out
	}
	return v
}

func stringsToYAML(ss []string) []any {
	out := make([]any, 0, len(ss))
	for _, s := range ss {
		out = append(out, s)
	}
	return out
}

// formatDuration prints durations the way people write them: "10m", not
// Go's "10m0s".
func formatDuration(d time.Duration) string {
	s := d.String()
	if strings.HasSuffix(s, "m0s") {
		s = strings.TrimSuffix(s, "0s")
	}
	if strings.HasSuffix(s, "h0m") {
		s = strings.TrimSuffix(s, "0m")
	}
	return s
}
