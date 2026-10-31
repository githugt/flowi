// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"cmp"
	"fmt"
	"maps"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/pb33f/libopenapi"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamldoc"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/openapi"
)

// FlowPrefix is where imported workflows go, under flows/.
const FlowPrefix = "arazzo/"

// Import turns every workflow of an Arazzo document into a flow.
func Import(data []byte, o Options) (Result, diag.List) {
	if _, err := libopenapi.NewArazzoDocument(data); err != nil {
		return Result{}, invalid(err.Error())
	}
	root, err := yamldoc.Decode(data)
	if err != nil {
		return Result{}, invalid(err.Error())
	}
	workflows := mapsOf(root["workflows"])
	if len(workflows) == 0 {
		return Result{}, invalid("the document has no workflows")
	}
	im := &importer{resolve: o.Resolve, sources: sourceTypes(root), flowIDs: map[string]model.ID{}}
	for _, w := range workflows {
		im.flowIDs[str(w, "workflowId")] = o.NewID()
	}
	var out Result
	for _, w := range workflows {
		out.Flows = append(out.Flows, im.workflow(w))
	}
	return out, im.diags
}

func invalid(detail string) diag.List {
	return diag.List{diag.New(diag.CodeImportInvalid, diag.Location{}, map[string]string{"detail": detail})}
}

// importer carries what every step needs, and collects the diagnostics.
type importer struct {
	resolve Resolver
	sources map[string]string // source description name → type
	flowIDs map[string]model.ID
	// written maps the variables the current workflow's imported step
	// outputs write so far to the output that writes each ("step\x00name").
	// A $steps read of anything else would render a variable nothing (or
	// another output) sets, so it is reported instead.
	written map[string]string
	diags   diag.List
}

func (im *importer) lossy(what, how, detail string) {
	im.diags = append(im.diags, diag.New(diag.CodeImportUnmapped, diag.Location{},
		map[string]string{"what": what, "how": how, "detail": detail}))
}

func (im *importer) workflow(w map[string]any) model.Flow {
	id := str(w, "workflowId")
	im.written = map[string]string{}
	f := model.Flow{
		Meta: model.Meta{ID: im.flowIDs[id], Name: cmp.Or(str(w, "summary"), id), Description: str(w, "description")},
		Path: FlowPrefix + id,
	}
	f.Inputs = im.inputs(id, w["inputs"])
	for _, s := range mapsOf(w["steps"]) {
		f.Nodes = append(f.Nodes, im.step(s)...)
	}
	for _, kv := range im.keyValues(fmt.Sprintf("output %%s of workflow %s", id), w["outputs"], "") {
		f.Outputs = append(f.Outputs, model.Output{Name: kv.Key, Value: kv.Value})
	}
	for _, key := range []string{"successActions", "failureActions", "parameters", "dependsOn"} {
		if w[key] != nil {
			im.lossy("workflow "+id+" "+key, "left out", "Flowi flows have no equivalent")
		}
	}
	return f
}

// inputs maps an inputs schema's properties to flow inputs.
func (im *importer) inputs(workflow string, v any) []model.Input {
	if v == nil {
		return nil
	}
	schema, _ := v.(map[string]any)
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		im.lossy("inputs of workflow "+workflow, "left out", "only an inline object schema with properties maps to flow inputs")
		return nil
	}
	var out []model.Input
	for _, name := range slices.Sorted(maps.Keys(props)) {
		in := model.Input{Name: name}
		if p, _ := props[name].(map[string]any); p["default"] != nil {
			in.Default = fmt.Sprint(p["default"])
		}
		out = append(out, in)
	}
	return out
}

// step maps one step to its node and, when it has them, the assert nodes
// of its success criteria and the set node of its outputs.
func (im *importer) step(s map[string]any) []model.Node {
	id := str(s, "stepId")
	n, ok := im.callNode(id, s)
	if !ok {
		return []model.Node{n}
	}
	n.Needs = append(stringsOf(s["dependsOn"]), stringsOf(s["x-flowi-depends-on"])...)
	n.AfterFailureOf = stringsOf(s["x-flowi-after-failure-of"])
	n.AfterOutcome = stringsOf(s["x-flowi-after-outcome"])
	n.Unlinked, _ = s["x-flowi-unlinked"].(bool)
	n.Timeout = timeoutOf(s)
	im.stepLosses(id, s)
	nodes := []model.Node{n}
	nodes = append(nodes, im.criteria(id, s["successCriteria"])...)
	if out := im.outputs(id, s["outputs"]); out != nil {
		nodes = append(nodes, *out)
	}
	return nodes
}

// callNode builds the request or subflow node, or a note when the step
// cannot be called from Flowi.
func (im *importer) callNode(id string, s map[string]any) (model.Node, bool) {
	switch {
	case s["channelPath"] != nil || (s["operationId"] == nil && s["action"] != nil):
		return im.note(id, "AsyncAPI steps that name a channel instead of an operation are not supported"), false
	case s["operationPath"] != nil:
		return im.note(id, "operationPath steps are not supported; use operationId"), false
	case s["workflowId"] != nil:
		return im.subflow(id, s)
	case s["operationId"] != nil:
		return im.request(id, s)
	}
	return im.note(id, "the step calls nothing"), false
}

func (im *importer) note(step, detail string) model.Node {
	im.lossy("step "+step, "imported as a note", detail)
	return model.Node{ID: step, Type: "note", Params: []model.Field{{Key: "text", Value: "Arazzo step " + step + " was not imported: " + detail}}}
}

func (im *importer) request(id string, s map[string]any) (model.Node, bool) {
	op := operationName(str(s, "operationId"))
	ref, version, ok := im.resolve.RequestFor(op)
	if !ok {
		return im.note(id, fmt.Sprintf("operation %q is not a request in the collection", op)), false
	}
	with := append(im.parameters(id, s["parameters"]), im.payload(id, s["requestBody"])...)
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: ref, Version: version, With: with}}, true
}

func (im *importer) subflow(id string, s map[string]any) (model.Node, bool) {
	target := str(s, "workflowId")
	flowID, ok := im.flowIDs[target]
	if !ok {
		return im.note(id, fmt.Sprintf("workflow %q is not in this document", target)), false
	}
	with := im.parameters(id, s["parameters"])
	return model.Node{ID: id, Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: flowID, Path: FlowPrefix + target}, With: with}}, true
}

// parameters maps step parameters to with entries. Parameter values are
// rendered before the call, so $response and $statusCode cannot appear.
func (im *importer) parameters(step string, v any) []model.KeyValue {
	var out []model.KeyValue
	for _, p := range mapsOf(v) {
		name := str(p, "name")
		if p["reference"] != nil {
			im.lossy("parameter reference in step "+step, "left out", "reusable components are not supported")
			continue
		}
		if value, ok := im.value("parameter "+name+" of step "+step, p["value"], ""); ok {
			out = append(out, model.KeyValue{Key: openapi.ParamKey(name), Value: value})
		}
	}
	return out
}

// payload maps a request body's top-level fields to with entries, which
// fill the request's {{field}} placeholders.
func (im *importer) payload(step string, v any) []model.KeyValue {
	body, _ := v.(map[string]any)
	if body == nil {
		return nil
	}
	if body["replacements"] != nil {
		im.lossy("request body replacements of step "+step, "left out", "Flowi fills the request's own body template instead")
	}
	if body["payload"] == nil {
		return nil
	}
	fields, ok := body["payload"].(map[string]any)
	if !ok {
		im.lossy("request body of step "+step, "left out", "only an object payload maps to request fields")
		return nil
	}
	return im.keyValues("body field %s of step "+step, fields, "")
}

// keyValues maps a name → expression map in name order.
func (im *importer) keyValues(whatFormat string, v any, step string) []model.KeyValue {
	m, _ := v.(map[string]any)
	var out []model.KeyValue
	for _, k := range slices.Sorted(maps.Keys(m)) {
		if value, ok := im.value(fmt.Sprintf(whatFormat, k), m[k], step); ok {
			out = append(out, model.KeyValue{Key: k, Value: value})
		}
	}
	return out
}

// value maps one scalar value; objects, lists and unmappable expressions
// are reported and left out.
func (im *importer) value(what string, v any, step string) (string, bool) {
	switch v.(type) {
	case map[string]any, []any:
		im.lossy(what, "left out", "structured values are not supported")
		return "", false
	}
	text := fmt.Sprint(v)
	if v == nil {
		text = ""
	}
	out, ok := mapExpression(text, step)
	if ok && !im.outputsWritten(text) {
		im.lossy(what, "left out", fmt.Sprintf("expression %q reads a step output that was not imported or comes later", text))
		return "", false
	}
	if !ok {
		im.lossy(what, "left out", fmt.Sprintf("expression %q has no Flowi equivalent", text))
	}
	return out, ok
}

var stepOutput = regexp.MustCompile(`\$steps\.([^.\s{}]+)\.outputs\.([^.\s{}]+)`)

// outputsWritten reports whether every $steps read in text names an
// output already imported in this workflow.
func (im *importer) outputsWritten(text string) bool {
	for _, m := range stepOutput.FindAllStringSubmatch(text, -1) {
		if im.written[varName(m[1], m[2])] != m[1]+"\x00"+m[2] {
			return false
		}
	}
	return true
}

// criteria maps each simple success criterion to an assert node.
func (im *importer) criteria(step string, v any) []model.Node {
	var out []model.Node
	for i, c := range mapsOf(v) {
		cond := str(c, "condition")
		what := fmt.Sprintf("success criterion %d of step %s", i+1, step)
		if c["type"] != nil && c["type"] != "simple" || c["context"] != nil {
			im.lossy(what, "left out", fmt.Sprintf("only simple conditions are supported: %q", cond))
			continue
		}
		check, ok := mapCondition(cond, step)
		if !ok || !im.outputsWritten(cond) {
			im.lossy(what, "left out", fmt.Sprintf("condition %q has no Flowi equivalent", cond))
			continue
		}
		id := step + "-check"
		if len(out) > 0 {
			id = fmt.Sprintf("%s-%d", id, len(out)+1)
		}
		out = append(out, model.Node{ID: id, Type: "assert", Params: []model.Field{{Key: "check", Value: check}}})
	}
	return out
}

// outputs maps step outputs to a set node writing vars.<step>_<name>.
func (im *importer) outputs(step string, v any) *model.Node {
	values := im.keyValues("output %s of step "+step, v, step)
	if len(values) == 0 {
		return nil
	}
	n := model.Node{ID: step + "-outputs", Type: "set"}
	for _, kv := range values {
		name, owner := varName(step, kv.Key), step+"\x00"+kv.Key
		if other, taken := im.written[name]; taken && other != owner {
			im.lossy("output "+kv.Key+" of step "+step, "left out",
				fmt.Sprintf("its variable vars.%s is already written by output %s", name, strings.Replace(other, "\x00", ".", 1)))
			continue
		}
		n.Values = append(n.Values, model.SetValue{Name: name, Value: kv.Value})
		im.written[name] = owner
	}
	if len(n.Values) == 0 {
		return nil
	}
	return &n
}

// stepLosses reports step fields Flowi does not carry over.
func (im *importer) stepLosses(step string, s map[string]any) {
	for _, key := range []string{"onSuccess", "onFailure"} {
		if s[key] != nil {
			im.lossy("step "+step+" "+key, "left out", "Flowi has no step actions; use on_error and condition nodes")
		}
	}
}

// maxTimeoutMS keeps a timeout within time.Duration; larger values are
// ignored like any other unreadable timeout.
const maxTimeoutMS = uint64(math.MaxInt64 / int64(time.Millisecond))

// timeoutOf reads the 1.1 step timeout, or the extension Flowi exports
// to stay within 1.0.1; both are milliseconds.
func timeoutOf(s map[string]any) time.Duration {
	for _, key := range []string{"timeout", "x-flowi-timeout"} {
		if ms, ok := s[key].(uint64); ok && ms <= maxTimeoutMS {
			return time.Duration(ms) * time.Millisecond
		}
		if ms, ok := s[key].(int64); ok && ms > 0 {
			return time.Duration(ms) * time.Millisecond
		}
	}
	return 0
}

// operationName strips a $sourceDescriptions.<name>. prefix.
func operationName(op string) string {
	if rest, ok := strings.CutPrefix(op, "$sourceDescriptions."); ok {
		if _, name, found := strings.Cut(rest, "."); found {
			return name
		}
	}
	return op
}

func sourceTypes(root map[string]any) map[string]string {
	out := map[string]string{}
	for _, s := range mapsOf(root["sourceDescriptions"]) {
		out[str(s, "name")] = cmp.Or(str(s, "type"), "openapi")
	}
	return out
}

func mapsOf(v any) []map[string]any {
	list, _ := v.([]any)
	var out []map[string]any
	for _, item := range list {
		if m, ok := item.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}

func str(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

func stringsOf(v any) []string {
	list, _ := v.([]any)
	var out []string
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
