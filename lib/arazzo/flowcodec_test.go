// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi-lib/diag"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

func flowCodecOptions() FlowOptions {
	return FlowOptions{
		Body:    yamlcodec.New(settings.Schema()),
		Resolve: fakeResolver{},
		Sources: []SourceDescription{{Name: "flowi", URL: "../descriptions/requests.openapi.yaml", Type: "openapi"}},
		NewID:   ids(),
	}
}

// fixtureFlows decodes every flow the repo ships as a fixture, so the codec is
// checked on all the node types they use.
func fixtureFlows(t *testing.T) []model.Flow {
	t.Helper()
	c := yamlcodec.New(settings.Schema())
	var out []model.Flow
	for _, glob := range []string{"../internal/codec/yaml/testdata/valid/*.flow.yaml", "../examples/user-creation/flows/*.flow.yaml"} {
		files, _ := filepath.Glob(glob)
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			path := strings.TrimSuffix(filepath.Base(file), ".flow.yaml")
			f, diags := c.DecodeFlow(file, path, data)
			if diags.HasErrors() {
				t.Fatalf("%s: %v", file, diags)
			}
			// Start from the canonical form (the codec sorts a node's params), the form every saved flow has.
			canonical, err := c.EncodeFlow(f)
			if err != nil {
				t.Fatal(err)
			}
			f, _ = c.DecodeFlow(file, path, canonical)
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		t.Fatal("no flow fixtures found")
	}
	return out
}

func TestFlowCodec_should_round_trip_every_fixture_flow_byte_stably(t *testing.T) {
	o := flowCodecOptions()
	for _, f := range fixtureFlows(t) {
		t.Run(f.Path, func(t *testing.T) {
			first, _, err := EncodeFlow(f, o)
			if err != nil {
				t.Fatal(err)
			}

			back, diags := DecodeFlow(f.File, f.Path, first, o)
			second, _, err2 := EncodeFlow(back, o)

			if diags.HasErrors() || err2 != nil {
				t.Fatalf("decode %v, encode %v", diags, err2)
			}
			if !reflect.DeepEqual(back, f) {
				t.Fatalf("flow changed by the round trip: %s", firstDifference(t, back, f))
			}
			if string(first) != string(second) {
				t.Fatalf("bytes changed on the second encode:\n%s\n---\n%s", first, second)
			}
		})
	}
}

func sampleFlow() model.Flow {
	return model.Flow{
		Meta: model.Meta{ID: "01JA0000000000000000000050", Name: "Login flow"}, Path: "login-flow", File: "login-flow.arazzo.yaml",
		Nodes: []model.Node{
			{ID: "login", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: "01JA0000000000000000000040", Path: "pets/getPet"}}},
			{ID: "ok", Type: "assert", Params: []model.Field{{Key: "check", Value: "nodes.login.status == 200"}}},
			{ID: "remember", Type: "set", Params: []model.Field{{Key: "vars", Value: "x"}}},
		},
	}
}

func TestEncodeFlow_should_write_standard_Arazzo_for_a_request_with_its_assert(t *testing.T) {
	got, _, err := EncodeFlow(sampleFlow(), flowCodecOptions())

	text := string(got)
	for _, want := range []string{"arazzo: 1.1.0", "workflowId: login-flow", "operationId: $sourceDescriptions.flowi.getPet", "successCriteria", "x-flowi-flow:"} {
		if !strings.Contains(text, want) {
			t.Errorf("document lacks %q:\n%s", want, text)
		}
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestEncodeFlow_should_make_a_node_Arazzo_cannot_express_a_custom_step(t *testing.T) {
	got, _, _ := EncodeFlow(sampleFlow(), flowCodecOptions())

	text := string(got)
	if !strings.Contains(text, "operationId: $sourceDescriptions.flowi."+BuiltinOperation) || !strings.Contains(text, "x-flowi-node: remember") {
		t.Fatalf("no custom step for the set node:\n%s", text)
	}
}

func TestDecodeFlow_Given_a_foreign_document_Then_steps_become_nodes(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "official-1.1.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	f, diags := DecodeFlow("official.arazzo.yaml", "arazzo/official", data, flowCodecOptions())

	if diags.HasErrors() || len(f.Nodes) == 0 || f.Path != "arazzo/official" || f.File != "official.arazzo.yaml" || f.ID == "" {
		t.Fatalf("flow %+v diags %v", f, diags)
	}
}

// A person edited the standard part of a step in another tool, and left Flowi's own data alone.
func TestDecodeFlow_Given_a_step_edited_outside_Flow_I_Then_FLW_ARAZZO_001_and_Flow_I_s_data_wins(t *testing.T) {
	o := flowCodecOptions()
	f := sampleFlow()
	data, _, _ := EncodeFlow(f, o)
	edited := strings.Replace(string(data), "$statusCode == 200", "$statusCode == 201", 1)
	if edited == string(data) {
		t.Fatal("the fixture has no criterion to edit")
	}

	back, diags := DecodeFlow(f.File, f.Path, []byte(edited), o)

	var drift bool
	for _, d := range diags {
		drift = drift || d.Code == diag.CodeArazzoDrift
	}
	if !drift || !reflect.DeepEqual(back, f) {
		t.Fatalf("drift %v diags %v flow %+v", drift, diags, back)
	}
}

// firstDifference names where two flows first differ, as JSON, with a little context.
func firstDifference(t *testing.T, got, want model.Flow) string {
	t.Helper()
	a, _ := json.Marshal(got)
	b, _ := json.Marshal(want)
	for i := range min(len(a), len(b)) {
		if a[i] != b[i] {
			from := max(0, i-60)
			return fmt.Sprintf("at byte %d:\n got  …%s\n want …%s", i, a[from:min(len(a), i+80)], b[from:min(len(b), i+80)])
		}
	}
	return "one is a prefix of the other"
}

// Other Arazzo tools must be able to read what Flowi writes.
func TestEncodeFlow_should_produce_a_document_a_standard_parser_accepts(t *testing.T) {
	for _, f := range append(fixtureFlows(t), sampleFlow()) {
		data, _, err := EncodeFlow(f, flowCodecOptions())
		if err != nil {
			t.Fatal(err)
		}

		if _, err := libopenapi.NewArazzoDocument(data); err != nil {
			t.Errorf("%s: %v", f.Path, err)
		}
	}
}
