// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// flowiSource is the name of the collection's generated description in a
// document's sourceDescriptions.
const flowiSource = "flowi"

// EncodeFlow writes f as an Arazzo document with one workflow. The standard
// parts (steps, success criteria, outputs, inputs) are derived for other
// Arazzo tools; a node Arazzo cannot express becomes a custom step. The flow's
// own data is kept whole in the workflow's x-flowi-flow, which is what
// Flowi reads back, so the round trip loses nothing.
func EncodeFlow(f model.Flow, o FlowOptions) ([]byte, diag.List, error) {
	body, err := o.Body.EncodeFlow(f)
	if err != nil {
		return nil, nil, fmt.Errorf("encoding %s: %w", f.Path, err)
	}
	var bodyMap yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(body, &bodyMap, yaml.UseOrderedMap()); err != nil {
		return nil, nil, fmt.Errorf("reading back the encoded flow %s: %w", f.Path, err)
	}
	ex := &exporter{o: ExportOptions{Resolve: o.Resolve}, source: flowiSource, workflowIDs: workflowIDs([]model.Flow{f}), custom: true}
	workflow := ex.workflow(f)
	workflow = append(workflow, yaml.MapItem{Key: "x-flowi-digest", Value: stepsDigest(valueOf(workflow, "steps"))})
	workflow = append(workflow, yaml.MapItem{Key: "x-flowi-flow", Value: bodyMap})
	doc := yaml.MapSlice{
		{Key: "arazzo", Value: FlowVersion},
		{Key: "info", Value: yaml.MapSlice{{Key: "title", Value: cmp.Or(f.Name, f.Path, "flow")}, {Key: "version", Value: "1.0.0"}}},
		{Key: "sourceDescriptions", Value: sourceList(o.Sources)},
		{Key: "workflows", Value: []any{workflow}},
		{Key: "x-flowi-version", Value: 1},
	}
	out, err := yaml.MarshalWithOptions(doc, yaml.IndentSequence(true))
	if err != nil {
		return nil, nil, fmt.Errorf("writing %s: %w", f.Path, err)
	}
	return out, ex.diags, nil
}

func sourceList(sources []SourceDescription) []any {
	out := make([]any, 0, len(sources))
	for _, s := range sources {
		out = append(out, yaml.MapSlice{{Key: "name", Value: s.Name}, {Key: "url", Value: s.URL}, {Key: "type", Value: cmp.Or(s.Type, "openapi")}})
	}
	return out
}

// DecodeFlow reads a flow from an Arazzo document. A document Flowi wrote
// carries x-flowi-flow, which is trusted; if the standard part of a step no
// longer matches what that data derives, the step was edited elsewhere:
// FLW-ARAZZO-001, and Flowi's data is used. A foreign document (no
// extension) is read through the importer: its first workflow becomes the
// flow, and what has no Flowi equivalent is reported by Import.
func DecodeFlow(file, path string, data []byte, o FlowOptions) (model.Flow, diag.List) {
	var root map[string]any
	if err := yaml.Unmarshal(data, &root); err != nil {
		return model.Flow{Path: path, File: file}, invalid(err.Error())
	}
	workflows := mapsOf(root["workflows"])
	if len(workflows) == 0 {
		return model.Flow{Path: path, File: file}, invalid("the document has no workflows")
	}
	ownBytes, ok, err := ownFlowBytes(data)
	if err != nil {
		return model.Flow{Path: path, File: file}, invalid(err.Error())
	}
	if !ok {
		return decodeForeign(file, path, data, o)
	}
	f, diags := o.Body.DecodeFlow(file, path, ownBytes)
	for i := range diags {
		diags[i].Pos = diag.Position{} // the embedded bytes have their own lines; the store positions it by path
	}
	return f, append(diags, driftDiags(f, workflows[0], file)...)
}

func decodeForeign(file, path string, data []byte, o FlowOptions) (model.Flow, diag.List) {
	res, diags := Import(data, Options{NewID: o.NewID, Resolve: o.Resolve})
	if len(res.Flows) == 0 {
		return model.Flow{Path: path, File: file}, diags
	}
	f := res.Flows[0]
	f.Path, f.File = path, file
	return f, diags
}

// driftDiags reports a workflow whose standard steps no longer match the
// digest Flowi wrote beside them: someone edited them in another tool. A
// file without a digest (written by hand) is not checked.
func driftDiags(f model.Flow, fileWorkflow map[string]any, file string) diag.List {
	want, _ := fileWorkflow["x-flowi-digest"].(string)
	if want == "" || want == stepsDigest(fileWorkflow["steps"]) {
		return nil
	}
	return diag.List{diag.New(diag.CodeArazzoDrift, diag.Location{File: file, Path: "$.workflows[0].steps"}, map[string]string{"flow": f.Path})}
}

// stepsDigest is a hash of the standard steps in canonical form (parsed and
// written again with sorted keys), so formatting and key order do not matter.
func stepsDigest(steps any) string {
	data, err := yaml.Marshal(steps)
	if err != nil {
		return ""
	}
	var canonical any
	if yaml.Unmarshal(data, &canonical) != nil {
		return ""
	}
	data, err = yaml.Marshal(canonical)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// CustomStep is a node of a flow that Arazzo cannot express: its step calls
// the built-in operation and only Flowi can run it.
type CustomStep struct {
	Node  string // the node's id
	Type  string // the node's type
	Index int    // its place among the flow's top-level nodes ($.nodes[Index])
}

// CustomSteps lists the top-level nodes of f that become custom steps in its
// Arazzo form. Nodes folded into another step (an assert after a request)
// are standard, and so are nodes inside containers: the container is the
// custom step.
func CustomSteps(f model.Flow, r Resolver) []CustomStep {
	ex := &exporter{o: ExportOptions{Resolve: r}, source: flowiSource, workflowIDs: workflowIDs([]model.Flow{f}), custom: true}
	ex.workflow(f)
	custom := map[string]bool{}
	for _, n := range ex.customNodes {
		custom[n] = true
	}
	var out []CustomStep
	for i, n := range f.Nodes {
		if custom[n.ID] {
			out = append(out, CustomStep{Node: n.ID, Type: string(n.Type), Index: i})
		}
	}
	return out
}

// ownFlowBytes is the x-flowi-flow of the first workflow, as YAML in the order
// it was written (a node's params are ordered). ok is false when there is none.
func ownFlowBytes(data []byte) ([]byte, bool, error) {
	var doc yaml.MapSlice
	if err := yaml.UnmarshalWithOptions(data, &doc, yaml.UseOrderedMap()); err != nil {
		return nil, false, err
	}
	workflows, _ := valueOf(doc, "workflows").([]any)
	if len(workflows) == 0 {
		return nil, false, nil
	}
	first, _ := workflows[0].(yaml.MapSlice)
	own, ok := valueOf(first, "x-flowi-flow").(yaml.MapSlice)
	if !ok {
		return nil, false, nil
	}
	out, err := yaml.Marshal(own)
	return out, err == nil, err
}

func valueOf(m yaml.MapSlice, key string) any {
	for _, item := range m {
		if item.Key == key {
			return item.Value
		}
	}
	return nil
}

// Limit is something an otherwise standard Arazzo step cannot say: What is
// "reply" (the request also waits for a reply) or "match" (it filters the
// messages it counts). Flowi keeps both; other Arazzo tools do not see them.
type Limit struct {
	Node string // the node's id
	What string
}

// Limits lists what the standard steps of f lose in Arazzo.
func Limits(f model.Flow, r Resolver) []Limit {
	ex := &exporter{o: ExportOptions{Resolve: r}, source: flowiSource, workflowIDs: workflowIDs([]model.Flow{f}), custom: true}
	ex.workflow(f)
	return ex.limits
}
