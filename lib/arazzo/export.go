// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"cmp"
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Version is the Arazzo version Export writes. Step fields 1.0.1 lacks
// (dependsOn, timeout) are written as x-flowi-* extensions, which Import
// reads back.
const Version = "1.0.1"

var unsafeWorkflowID = regexp.MustCompile(`[^A-Za-z0-9_\-]`)

// Export writes flows as one Arazzo document (plan Decision 5). Request
// nodes become steps; asserts and map-form extracts directly after a
// request become its successCriteria and outputs; subflows of exported
// flows become workflowId steps. Other nodes are listed in the workflow's
// x-flowi-skipped and reported.
func Export(flows []model.Flow, o ExportOptions) ([]byte, diag.List) {
	ex := &exporter{o: o, source: cmp.Or(o.Source, "api"), workflowIDs: workflowIDs(flows)}
	var workflows []any
	for _, f := range flows {
		workflows = append(workflows, ex.workflow(f))
	}
	title := o.Title
	if title == "" && len(flows) > 0 {
		title = flows[0].Name
	}
	sources := []any{yaml.MapSlice{
		{Key: "name", Value: ex.source},
		{Key: "url", Value: cmp.Or(o.SourceURL, "./"+ex.source+".openapi.yaml")},
		{Key: "type", Value: "openapi"},
	}}
	version := Version
	if ex.usedAsync {
		// action and AsyncAPI sources are Arazzo 1.1.
		version = FlowVersion
		sources = append(sources, yaml.MapSlice{
			{Key: "name", Value: AsyncSource}, {Key: "url", Value: cmp.Or(o.AsyncSourceURL, "./requests.asyncapi.yaml")}, {Key: "type", Value: "asyncapi"}})
	}
	doc := yaml.MapSlice{
		{Key: "arazzo", Value: version},
		{Key: "info", Value: yaml.MapSlice{{Key: "title", Value: cmp.Or(title, "Flowi export")}, {Key: "version", Value: "1.0.0"}}},
		{Key: "sourceDescriptions", Value: sources},
		{Key: "workflows", Value: workflows},
	}
	out, err := yaml.MarshalWithOptions(doc, yaml.IndentSequence(true))
	if err != nil {
		// Only maps, lists and strings are marshalled; failure is a bug.
		panic(fmt.Sprintf("arazzo: marshal export: %v", err))
	}
	return out, ex.diags
}

// workflowIDs names each flow's workflow: its path without the import
// prefix, made safe, and unique.
func workflowIDs(flows []model.Flow) map[model.ID]string {
	out := map[model.ID]string{}
	taken := map[string]bool{}
	for _, f := range flows {
		base := unsafeWorkflowID.ReplaceAllString(strings.TrimPrefix(f.Path, FlowPrefix), "-")
		id := cmp.Or(base, "workflow")
		for i := 2; taken[id]; i++ {
			id = fmt.Sprintf("%s-%d", base, i)
		}
		taken[id] = true
		out[f.ID] = id
	}
	return out
}

type exporter struct {
	// custom makes a node Arazzo cannot express a step that calls the
	// built-in operation, instead of leaving it out (flows are stored whole).
	custom      bool
	customNodes []string // ids of the nodes that became custom steps
	limits      []Limit  // what standard async steps could not say
	usedAsync   bool     // some step calls the AsyncAPI description
	o           ExportOptions
	source      string
	workflowIDs map[model.ID]string
	diags       diag.List
}

// step is one Arazzo step being built.
type step struct {
	id, operationID, workflowID string
	params                      yaml.MapSlice // name → in ("" for workflow parameters)
	values                      map[string]string
	payload                     yaml.MapSlice
	criteria                    []string
	outputs                     yaml.MapSlice
	needs                       []string
	afterFailureOf              []string
	afterOutcome                []string
	unlinked                    bool
	timeoutMS                   int64
	customNode                  string // the node a custom step stands for
	action                      string // send or receive (AsyncAPI steps)
	receiveMS                   int64  // Arazzo's own timeout: how long a receive waits
}

// workflowState is what a workflow's nodes share while being exported:
// the steps so far, whether the last one may still absorb asserts and
// extracts, and the variables earlier outputs wrote.
type workflowState struct {
	steps   []*step
	open    *step
	vars    map[string]string // var name → $steps.<s>.outputs.<o>
	skipped []string
}

func (ex *exporter) workflow(f model.Flow) yaml.MapSlice {
	st := &workflowState{vars: map[string]string{}}
	for _, n := range f.Nodes {
		ex.node(n, st)
	}
	id := ex.workflowIDs[f.ID]
	w := yaml.MapSlice{{Key: "workflowId", Value: id}}
	if f.Name != "" && f.Name != id {
		w = append(w, yaml.MapItem{Key: "summary", Value: f.Name})
	}
	if f.Description != "" {
		w = append(w, yaml.MapItem{Key: "description", Value: f.Description})
	}
	if len(f.Inputs) > 0 {
		w = append(w, yaml.MapItem{Key: "inputs", Value: inputsSchema(f.Inputs)})
	}
	var steps []any
	for _, s := range st.steps {
		steps = append(steps, s.yaml())
	}
	w = append(w, yaml.MapItem{Key: "steps", Value: steps})
	if outputs := ex.flowOutputs(f, st.vars); len(outputs) > 0 {
		w = append(w, yaml.MapItem{Key: "outputs", Value: outputs})
	}
	if len(st.skipped) > 0 {
		w = append(w, yaml.MapItem{Key: "x-flowi-skipped", Value: st.skipped})
	}
	return w
}

// stepID is Arazzo's stepId syntax.
var stepID = regexp.MustCompile(`^[A-Za-z0-9_\-]+$`)

// carriable reports whether a node's own settings survive as a step:
// Arazzo steps have no condition, error policy or concurrency, and ids
// follow stepId syntax.
func carriable(n model.Node) bool {
	return n.When == "" && n.OnError == "" && !n.Concurrent && stepID.MatchString(n.ID)
}

// node exports one node into the workflow state.
func (ex *exporter) node(n model.Node, st *workflowState) {
	ok := false
	switch {
	case !carriable(n):
	case n.Type == "request" && n.Request != nil:
		ok = ex.request(n, st)
	case n.Type == "subflow" && n.Subflow != nil:
		ok = ex.subflow(n, st)
	case n.Type == "assert" && st.open != nil:
		ok = ex.criterion(n, st.open, st.vars)
	case n.Type == "set" && st.open != nil && exprValuesOnly(n):
		ok = ex.outputs(n, st.open, st.vars)
	}
	if !ok {
		ex.skip(n, st)
	}
}

func (ex *exporter) skip(n model.Node, st *workflowState) {
	st.open = nil
	if ex.custom {
		s := newStep(n)
		s.operationID = "$sourceDescriptions." + ex.source + "." + BuiltinOperation
		s.customNode = n.ID
		ex.customNodes = append(ex.customNodes, n.ID)
		st.steps = append(st.steps, s)
		return
	}
	st.skipped = append(st.skipped, n.ID)
	ex.diags = append(ex.diags, diag.New(diag.CodeExportLossy, diag.Location{}, map[string]string{"node": n.ID, "type": string(n.Type)}))
}

func (ex *exporter) lossyValue(what, node, value string) {
	ex.diags = append(ex.diags, diag.New(diag.CodeExportValue, diag.Location{},
		map[string]string{"what": what, "node": node, "value": value}))
}

func (ex *exporter) request(n model.Node, st *workflowState) bool {
	op, ok := ex.o.Resolve.OperationOf(*n.Request)
	if ok && op.Custom != "" {
		ex.limits = append(ex.limits, Limit{Node: n.ID, What: op.Custom})
		return false // a custom step: Flowi runs the call from x-flowi-flow
	}
	if !ok || op.ID == "" {
		return false
	}
	s := newStep(n)
	s.operationID = "$sourceDescriptions." + cmp.Or(op.Source, ex.source) + "." + op.ID
	s.action, s.receiveMS = op.Action, op.Timeout.Milliseconds()
	ex.usedAsync = ex.usedAsync || op.Source == AsyncSource
	ex.noteLimits(n, op)
	if n.Request.Overlay != "" {
		// Arazzo has no such idea: the step is the base request, said so.
		ex.lossyValue("flowi overlay", n.ID, n.Request.Overlay)
	}
	for _, use := range []struct{ what, name string }{{"open connection", n.Request.Open}, {"over connection", n.Request.Over}} {
		if use.name != "" {
			// Arazzo has no connection kept between steps: each step stands alone.
			ex.lossyValue(use.what, n.ID, use.name)
		}
	}
	for _, kv := range n.Request.With {
		value, ok := unmapTemplate(kv.Value, "", st.vars)
		if !ok {
			ex.lossyValue("with "+kv.Key, n.ID, kv.Value)
			continue
		}
		if p, ok := op.Params[kv.Key]; ok && p.In != "" {
			s.params = append(s.params, yaml.MapItem{Key: p.Name, Value: p.In})
			s.values[p.Name] = value
		} else {
			s.payload = append(s.payload, yaml.MapItem{Key: kv.Key, Value: value})
		}
	}
	st.steps = append(st.steps, s)
	st.open = s
	return true
}

// noteLimits records what an async step cannot say in Arazzo.
func (ex *exporter) noteLimits(n model.Node, op Operation) {
	if op.Reply {
		ex.limits = append(ex.limits, Limit{Node: n.ID, What: "reply"})
	}
	if op.Match {
		ex.limits = append(ex.limits, Limit{Node: n.ID, What: "match"})
	}
}

func (ex *exporter) subflow(n model.Node, st *workflowState) bool {
	target, ok := ex.workflowIDs[n.Subflow.Flow.ID]
	if !ok || n.Subflow.Flow.Collection != "" {
		return false
	}
	s := newStep(n)
	s.workflowID = target
	for _, kv := range n.Subflow.With {
		value, ok := unmapTemplate(kv.Value, "", st.vars)
		if !ok {
			ex.lossyValue("with "+kv.Key, n.ID, kv.Value)
			continue
		}
		s.params = append(s.params, yaml.MapItem{Key: kv.Key, Value: ""})
		s.values[kv.Key] = value
	}
	st.steps = append(st.steps, s)
	st.open = nil // a workflow step has no response to check
	return true
}

// criterion turns an assert after a request into a success criterion. A
// warn-only assert or one with a message is not a criterion: Arazzo
// criteria always fail the step and carry no message.
func (ex *exporter) criterion(n model.Node, s *step, vars map[string]string) bool {
	if len(n.Params) != 1 || n.Params[0].Key != "check" {
		return false
	}
	check, _ := n.Params[0].Value.(string)
	cond, ok := unmapCondition(check, s.id, vars)
	if !ok {
		return false
	}
	s.criteria = append(s.criteria, cond)
	return true
}

// exprValuesOnly reports whether a set node is a list of plain expressions,
// the only kind an Arazzo step output can say.
func exprValuesOnly(n model.Node) bool {
	for _, v := range n.Values {
		if _, isText := v.Value.(string); !isText || (v.From != "" && v.From != "expr") {
			return false
		}
	}
	return len(n.Values) > 0
}

// outputs turns a set node of expressions after a request into its outputs.
// Keys written by Import (<step>_<name>) get their name back.
func (ex *exporter) outputs(n model.Node, s *step, vars map[string]string) bool {
	prefix := varName(s.id, "")
	for _, v := range n.Values {
		text, _ := v.Value.(string)
		value, ok := unmapTemplate(text, s.id, vars)
		if !ok {
			ex.lossyValue("value "+v.Name, n.ID, fmt.Sprint(v.Value))
			continue
		}
		name := cmp.Or(strings.TrimPrefix(v.Name, prefix), v.Name)
		s.outputs = append(s.outputs, yaml.MapItem{Key: name, Value: value})
		vars[v.Name] = "$steps." + s.id + ".outputs." + name
	}
	return true
}

func (ex *exporter) flowOutputs(f model.Flow, vars map[string]string) yaml.MapSlice {
	var out yaml.MapSlice
	for _, o := range f.Outputs {
		value, ok := unmapTemplate(o.Value, "", vars)
		if !ok {
			ex.lossyValue("output "+o.Name, f.Path, o.Value)
			continue
		}
		out = append(out, yaml.MapItem{Key: o.Name, Value: value})
	}
	return out
}

func newStep(n model.Node) *step {
	return &step{id: n.ID, values: map[string]string{}, needs: n.Needs, afterFailureOf: n.AfterFailureOf, afterOutcome: n.AfterOutcome, unlinked: n.Unlinked, timeoutMS: n.Timeout.Milliseconds()}
}

func (s *step) yaml() yaml.MapSlice {
	out := yaml.MapSlice{{Key: "stepId", Value: s.id}}
	if s.operationID != "" {
		out = append(out, yaml.MapItem{Key: "operationId", Value: s.operationID})
	} else {
		out = append(out, yaml.MapItem{Key: "workflowId", Value: s.workflowID})
	}
	if s.action != "" {
		out = append(out, yaml.MapItem{Key: "action", Value: s.action})
	}
	if len(s.params) > 0 {
		out = append(out, yaml.MapItem{Key: "parameters", Value: s.parameters()})
	}
	if len(s.payload) > 0 {
		out = append(out, yaml.MapItem{Key: "requestBody", Value: yaml.MapSlice{
			{Key: "contentType", Value: "application/json"}, {Key: "payload", Value: s.payload},
		}})
	}
	if len(s.criteria) > 0 {
		var criteria []any
		for _, c := range s.criteria {
			criteria = append(criteria, yaml.MapSlice{{Key: "condition", Value: c}})
		}
		out = append(out, yaml.MapItem{Key: "successCriteria", Value: criteria})
	}
	if len(s.outputs) > 0 {
		out = append(out, yaml.MapItem{Key: "outputs", Value: s.outputs})
	}
	if s.receiveMS > 0 {
		out = append(out, yaml.MapItem{Key: "timeout", Value: s.receiveMS})
	}
	if s.customNode != "" {
		out = append(out, yaml.MapItem{Key: "x-flowi-node", Value: s.customNode})
	}
	if len(s.needs) > 0 {
		out = append(out, yaml.MapItem{Key: "x-flowi-depends-on", Value: s.needs})
	}
	if len(s.afterFailureOf) > 0 {
		out = append(out, yaml.MapItem{Key: "x-flowi-after-failure-of", Value: s.afterFailureOf})
	}
	if s.unlinked {
		out = append(out, yaml.MapItem{Key: "x-flowi-unlinked", Value: true})
	}
	if len(s.afterOutcome) > 0 {
		out = append(out, yaml.MapItem{Key: "x-flowi-after-outcome", Value: s.afterOutcome})
	}
	if s.timeoutMS > 0 {
		out = append(out, yaml.MapItem{Key: "x-flowi-timeout", Value: s.timeoutMS})
	}
	return out
}

func (s *step) parameters() []any {
	var out []any
	for _, p := range s.params {
		name := p.Key.(string)
		item := yaml.MapSlice{{Key: "name", Value: name}}
		if in := p.Value.(string); in != "" {
			item = append(item, yaml.MapItem{Key: "in", Value: in})
		}
		out = append(out, append(item, yaml.MapItem{Key: "value", Value: s.values[name]}))
	}
	return out
}

func inputsSchema(inputs []model.Input) yaml.MapSlice {
	var props yaml.MapSlice
	for _, in := range inputs {
		p := yaml.MapSlice{{Key: "type", Value: "string"}}
		if in.Default != "" {
			p = append(p, yaml.MapItem{Key: "default", Value: in.Default})
		}
		props = append(props, yaml.MapItem{Key: in.Name, Value: p})
	}
	return yaml.MapSlice{{Key: "type", Value: "object"}, {Key: "properties", Value: props}}
}
