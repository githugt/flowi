// SPDX-License-Identifier: Apache-2.0

// Package schema_test keeps the published JSON Schemas in step with what
// Flow-I actually accepts.
package schema_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/settings"
)

const base = "https://github.com/githugt/flowi-lib/schema/"

func compile(t *testing.T, name string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	files, _ := filepath.Glob("*.schema.json")
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := c.AddResource(base+f, doc); err != nil {
			t.Fatal(err)
		}
	}
	sch, err := c.Compile(base + name + ".schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return sch
}

func yamlInstance(t *testing.T, file string) any {
	t.Helper()
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	js, err := yaml.YAMLToJSON(data)
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(js))
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// schemaFor maps a file name to the schema that describes it.
func schemaFor(file string) string {
	base := filepath.Base(file)
	switch {
	case base == "flowi.yaml":
		return "manifest"
	case base == "_folder.yaml":
		return "folder"
	case base == "workspace.yaml" || strings.HasPrefix(base, "workspace-"):
		return "workspace"
	case base == "settings.yaml":
		return "settings"
	case strings.HasSuffix(base, ".req.yaml"):
		return "request"
	case strings.HasSuffix(base, ".flow.yaml"):
		return "flow"
	case strings.HasSuffix(base, ".blueprint.yaml"):
		return "blueprint"
	case strings.Contains(filepath.ToSlash(file), "/environments/") || base == "staging.yaml":
		return "environment"
	}
	return ""
}

func validFiles(t *testing.T) []string {
	t.Helper()
	var files []string
	for _, pattern := range []string{
		"../examples/*/flowi.yaml", "../examples/*/environments/*.yaml", "../examples/*/requests/_folder.yaml",
		"../examples/*/requests/*/*.req.yaml", "../examples/*/flows/*.flow.yaml", "../examples/*/blueprints/*.blueprint.yaml",
		"../internal/codec/yaml/testdata/valid/*.yaml",
	} {
		matches, _ := filepath.Glob(pattern)
		files = append(files, matches...)
	}
	if len(files) < 15 {
		t.Fatalf("found only %d files to check", len(files))
	}
	return files
}

func TestSchemas_should_accept_every_example_and_valid_fixture(t *testing.T) {
	for _, file := range validFiles(t) {
		t.Run(file, func(t *testing.T) {
			name := schemaFor(file)
			if name == "" {
				t.Fatalf("no schema for %s", file)
			}

			err := compile(t, name).Validate(yamlInstance(t, file))

			if err != nil {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestSchemas_should_reject_invalid_fixtures(t *testing.T) {
	for _, file := range []string{"unknown-key.yaml", "bad-type.yaml", "missing-id.yaml", "bad-id.yaml", "empty-id.yaml"} {
		t.Run(file, func(t *testing.T) {
			err := compile(t, "manifest").Validate(yamlInstance(t, "../internal/codec/yaml/testdata/invalid/"+file))

			if err == nil {
				t.Fatal("schema accepted an invalid manifest")
			}
		})
	}
}

// TestSettingsSchema_should_match_settings_Schema keeps settings.schema.json
// identical in keys, enums and defaults to the Go settings table.
func TestSettingsSchema_should_match_settings_Schema(t *testing.T) {
	data, err := os.ReadFile("settings.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	inSchema := map[string]map[string]any{}
	flattenProperties(doc, "", inSchema)

	for _, k := range settings.Schema() {
		prop, ok := inSchema[k.Name]
		if !ok {
			t.Errorf("%s missing from settings.schema.json", k.Name)
			continue
		}
		delete(inSchema, k.Name)
		if !reflect.DeepEqual(normalise(prop["default"]), normalise(k.Default)) {
			t.Errorf("%s default: schema %v, Go %v", k.Name, prop["default"], k.Default)
		}
		if k.Kind == settings.KindEnum && !reflect.DeepEqual(normalise(prop["enum"]), normalise(k.Enum)) {
			t.Errorf("%s enum: schema %v, Go %v", k.Name, prop["enum"], k.Enum)
		}
	}
	for extra := range inSchema {
		t.Errorf("settings.schema.json has %s, which Go does not define", extra)
	}
}

func flattenProperties(node map[string]any, prefix string, out map[string]map[string]any) {
	props, _ := node["properties"].(map[string]any)
	for name, raw := range props {
		child, _ := raw.(map[string]any)
		if child["type"] == "object" {
			flattenProperties(child, prefix+name+".", out)
			continue
		}
		out[prefix+name] = child
	}
}

// normalise makes JSON-decoded and Go values comparable.
func normalise(v any) any {
	data, _ := json.Marshal(v)
	var out any
	_ = json.Unmarshal(data, &out) // both sides came from json.Marshal-able values
	if s, ok := out.(string); ok {
		if d, err := time.ParseDuration(s); err == nil {
			return d.String()
		}
	}
	return out
}

func TestSchemas_should_reject_inline_nodes_missing_required_params(t *testing.T) {
	for _, file := range []string{"set-no-target.yaml", "assert-no-check.yaml", "log-no-value.yaml", "extract-no-name.yaml"} {
		t.Run(file, func(t *testing.T) {
			err := compile(t, "manifest").Validate(yamlInstance(t, "testdata/invalid-inline/"+file))

			if err == nil {
				t.Fatal("schema accepted an inline node without its required parameters")
			}
		})
	}
}

// TestSchema_should_require_what_node_specs_require keeps the hand-written
// schema in step with the node registry: required parameters and enums.
func TestSchema_should_require_what_node_specs_require(t *testing.T) {
	data, err := os.ReadFile("common.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Defs struct {
			Node struct {
				AllOf []struct {
					If struct {
						Properties struct {
							Type struct {
								Const string `json:"const"`
							} `json:"type"`
						} `json:"properties"`
					} `json:"if"`
					Then struct {
						Required   []string                           `json:"required"`
						Properties map[string]struct{ Enum []string } `json:"properties"`
					} `json:"then"`
				} `json:"allOf"`
			} `json:"node"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	required := map[string][]string{}
	enums := map[string]map[string][]string{}
	for _, r := range doc.Defs.Node.AllOf {
		typ := r.If.Properties.Type.Const
		required[typ] = append(required[typ], r.Then.Required...)
		for name, p := range r.Then.Properties {
			if enums[typ] == nil {
				enums[typ] = map[string][]string{}
			}
			enums[typ][name] = p.Enum
		}
	}

	for _, s := range nodes.Specs() {
		for _, p := range s.Params {
			if p.Required && !slices.Contains(required[string(s.Type)], p.Name) {
				t.Errorf("schema does not require %s.%s", s.Type, p.Name)
			}
			if p.Kind == nodes.ParamEnum && !slices.Equal(enums[string(s.Type)][p.Name], p.Enum) {
				t.Errorf("schema enum for %s.%s is %v, spec says %v", s.Type, p.Name, enums[string(s.Type)][p.Name], p.Enum)
			}
		}
	}
}

func TestBlueprintSchema_should_reject_what_the_decoder_rejects(t *testing.T) {
	good := "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  api:\n    type: request-ref\nnodes:\n  - id: a\n    type: request\n    request:\n      param: api\n"
	tests := map[string]string{
		"param without type": "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  api:\n    description: x\n",
		"unknown type":       "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  api:\n    type: money\n",
		"bad param name":     "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  not valid:\n    type: string\n",
		"unknown param key":  "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nparams:\n  api:\n    type: string\n    colour: red\n",
		"unknown top key":    good + "colour: red\n",
		"node without type":  "id: 01J9ZB3M4N5P6Q7R8S9T0VWXYZ\nnodes:\n  - id: a\n",
	}
	sch := compile(t, "blueprint")
	dir := t.TempDir()
	write := func(name, text string) any {
		file := filepath.Join(dir, name+".yaml")
		if err := os.WriteFile(file, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		return yamlInstance(t, file)
	}

	if err := sch.Validate(write("good", good)); err != nil {
		t.Fatalf("a good blueprint was rejected: %v", err)
	}
	for name, text := range tests {
		if err := sch.Validate(write(strings.ReplaceAll(name, " ", "-"), text)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}
