// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

// flatSettings is a settings block flattened to dotted keys, with the key
// node of each for positioning diagnostics.
type flatSettings struct {
	values map[string]any
	nodes  map[string]ast.Node
}

func (d *decoder) flattenSettings(m *mapping) flatSettings {
	fs := flatSettings{values: map[string]any{}, nodes: map[string]ast.Node{}}
	d.flattenInto(fs, m, "")
	return fs
}

func (d *decoder) flattenInto(fs flatSettings, m *mapping, prefix string) {
	for _, e := range m.entries {
		name := prefix + e.key
		if inner, ok := e.value.(*ast.MappingNode); ok {
			child, _ := d.mapping(inner, e.path, e.key)
			d.flattenInto(fs, child, name+".")
			continue
		}
		fs.values[name] = d.raw(e.value, e.path, e.key)
		fs.nodes[name] = e.keyNode
	}
}

// settingsBlock validates a block against the schema at a layer and places
// the resulting diagnostics on the offending keys.
func (c *Codec) settingsBlock(d *decoder, m *mapping, layer settings.Layer) model.Settings {
	fs := d.flattenSettings(m)
	lv, diags := settings.FromBlock(c.schema, layer, d.file, fs.values)
	for _, dg := range diags {
		if n, ok := fs.nodes[dg.Args["key"]]; ok {
			dg.Pos = positionOf(n)
		}
		d.diags = append(d.diags, dg)
	}
	if len(lv.Values) == 0 {
		return nil
	}
	return lv.Values
}

func (c *Codec) settingsChild(d *decoder, parent *mapping, layer settings.Layer) model.Settings {
	m, ok := parent.child("settings")
	if !ok {
		return nil
	}
	return c.settingsBlock(d, m, layer)
}

// encodeSettings nests dotted keys back into mappings, in schema order.
func (c *Codec) encodeSettings(s model.Settings) yaml.MapSlice {
	var root yaml.MapSlice
	for _, k := range c.schema {
		v, ok := s[k.Name]
		if !ok {
			continue
		}
		root = insertNested(root, strings.Split(k.Name, "."), settingToYAML(v))
	}
	return root
}

func insertNested(m yaml.MapSlice, path []string, value any) yaml.MapSlice {
	if len(path) == 1 {
		return append(m, yaml.MapItem{Key: path[0], Value: value})
	}
	for i, item := range m {
		if item.Key == path[0] {
			if inner, ok := item.Value.(yaml.MapSlice); ok {
				m[i].Value = insertNested(inner, path[1:], value)
				return m
			}
		}
	}
	return append(m, yaml.MapItem{Key: path[0], Value: insertNested(nil, path[1:], value)})
}

func settingToYAML(v any) any {
	switch x := v.(type) {
	case time.Duration:
		return formatDuration(x)
	case []string:
		return stringsToYAML(x)
	}
	return v
}

// DecodeSettingsFile reads the user settings file into flattened, not yet
// validated values.
func (c *Codec) DecodeSettingsFile(file string, data []byte) (map[string]any, diag.List) {
	d, root, ok := c.parse(file, data)
	if !ok {
		return nil, d.diags
	}
	m, ok := d.mapping(root, "$", "settings")
	if !ok {
		return nil, d.diags
	}
	return d.flattenSettings(m).values, d.diags
}

// EncodeSettingsFile writes validated user settings in schema order.
func (c *Codec) EncodeSettingsFile(s model.Settings) ([]byte, error) {
	return marshal(c.encodeSettings(s))
}
