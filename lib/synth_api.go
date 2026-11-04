// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/synth"
)

// FlowSuggestion is a flow Flowi proposes from the requests of a collection and what their responses return. It is
// a draft: nothing is written until AcceptSuggestion.
type FlowSuggestion struct {
	// Name says what the flow exercises (the resource, e.g. pets).
	Name string `json:"name"`
	// Path is where the flow would be saved, under flows/, e.g. suggested/pets.
	Path  string          `json:"path"`
	Steps []SuggestedStep `json:"steps"`
	// Flow is the draft, ready to save.
	Flow model.Flow `json:"-"`
}

// SuggestedStep is one step of a suggestion and why it takes what it takes.
type SuggestedStep struct {
	ID      string `json:"id"` // the step's id in the draft flow
	Request string `json:"request"`
	Method  string `json:"method"`
	// Takes say which earlier response each path parameter comes from, one per parameter.
	Takes []SuggestedValue `json:"takes"`
}

// SuggestedValue is a path parameter filled from an earlier step's response.
type SuggestedValue struct {
	Param      string  `json:"param"`
	From       string  `json:"from"`  // the step id
	Field      string  `json:"field"` // the response field
	Confidence float64 `json:"confidence"`
}

// SuggestFlows proposes flows for a collection: for each request that creates something the requests of that
// resource take (an id in their path), a flow that creates, reads, updates and deletes it. It reads the default
// version of each HTTP request and what its successful response is known to hold (response schemas from the
// import, pinned examples or the last saved response). A collection with nothing known to feed anything gets none.
func (w *Workspace) SuggestFlows(collection model.ID) ([]FlowSuggestion, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	ops, versions := w.opsOf(c)
	proposals := synth.Propose(ops, synth.Graph(ops))
	out := make([]FlowSuggestion, 0, len(proposals))
	for _, p := range proposals {
		out = append(out, suggestionOf(p, versions))
	}
	return out, nil
}

// requestRef is how a draft step refers to a request: by id, as every saved flow must, at the version read.
type requestRef struct {
	ID      model.ID
	Version string
}

// opsOf describes the HTTP requests of c for synthesis, with the version of each that was read.
func (w *Workspace) opsOf(c model.Collection) ([]synth.Op, map[string]requestRef) {
	var ops []synth.Op
	versions := map[string]requestRef{}
	contracts := w.contracts()
	for _, r := range c.Requests {
		v, ok := defaultVersion(r)
		if !ok || v.Async != nil || v.GRPC != nil || v.Dataset != nil {
			continue
		}
		path, params := splitURL(v.URL)
		contract, _ := contracts.Contract(c, r, v.Label)
		ops = append(ops, synth.Op{Request: r.Path, Method: strings.ToUpper(v.Method), Path: path, Params: params, Fields: successFields(contract)})
		versions[r.Path] = requestRef{ID: r.ID, Version: v.Label}
	}
	return ops, versions
}

// defaultVersion is the version a run of r uses when none is pinned.
func defaultVersion(r model.Request) (model.RequestVersion, bool) {
	if i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == r.Default }); i >= 0 {
		return r.Versions[i], true
	}
	if len(r.Versions) == 0 {
		return model.RequestVersion{}, false
	}
	return r.Versions[0], true
}

var (
	baseURL    = regexp.MustCompile(`^\{\{[^}]*\}\}`)
	pathHolder = regexp.MustCompile(`\{\{([A-Za-z_][A-Za-z0-9_]*)\}\}`)
)

// splitURL gives the path of a request URL (no server, no query) written with {param} placeholders, and the names
// of the placeholders. Only a bare {{name}} is a path parameter: {{env.base_url}} and the like are not.
func splitURL(url string) (path string, params []string) {
	path, _, _ = strings.Cut(baseURL.ReplaceAllString(url, ""), "?")
	path = pathHolder.ReplaceAllStringFunc(path, func(m string) string {
		name := pathHolder.FindStringSubmatch(m)[1]
		params = append(params, name)
		return "{" + name + "}"
	})
	return path, params
}

// successFields lists the top-level fields a successful response is known to hold.
func successFields(c analysis.Contract) []string {
	var fields []string
	for _, s := range c.Samples {
		if s.Status < 200 || s.Status > 299 {
			continue
		}
		for _, name := range fieldNames(s) {
			if !slices.Contains(fields, name) {
				fields = append(fields, name)
			}
		}
	}
	return fields
}

func fieldNames(s analysis.Sample) []string {
	names := schemaFieldNames(s.Schema)
	if body, ok := s.Body.(map[string]any); ok {
		names = append(names, slices.Sorted(mapKeysOf(body))...)
	}
	return names
}

// schemaFieldNames lists the top-level properties of a JSON Schema, including those of allOf members (the
// importer keeps a $ref'd schema that extends another as an allOf).
func schemaFieldNames(schema any) []string {
	obj, ok := schema.(map[string]any)
	if !ok {
		return nil
	}
	var names []string
	if props, ok := obj["properties"].(map[string]any); ok {
		names = slices.Sorted(mapKeysOf(props))
	}
	if parts, ok := obj["allOf"].([]any); ok {
		for _, part := range parts {
			names = append(names, schemaFieldNames(part)...)
		}
	}
	return names
}

// suggestionOf turns a proposal into a draft flow: one request step per proposed step, each taking its path
// parameters from the body of the step that makes them.
func suggestionOf(p synth.Proposal, versions map[string]requestRef) FlowSuggestion {
	s := FlowSuggestion{Name: p.Name, Path: "suggested/" + p.Name}
	ids := map[string]string{}
	taken := map[string]bool{}
	for i, step := range p.Steps {
		id := stepID(step, i == 0, taken)
		ids[step.Request] = id
		node := model.Node{ID: id, Type: model.NodeType("request"), Request: &model.RequestCall{Request: model.Ref{ID: versions[step.Request].ID, Path: step.Request}, Version: versions[step.Request].Version}}
		out := SuggestedStep{ID: id, Request: step.Request, Method: step.Method}
		for _, e := range step.Takes {
			node.Request.With = append(node.Request.With, model.KeyValue{Key: e.Param, Value: fmt.Sprintf("{{nodes.%s.body.%s}}", ids[e.From], e.Field)})
			out.Takes = append(out.Takes, SuggestedValue{Param: e.Param, From: ids[e.From], Field: e.Field, Confidence: e.Confidence})
		}
		s.Flow.Nodes = append(s.Flow.Nodes, node)
		s.Steps = append(s.Steps, out)
	}
	s.Flow.Name = p.Name + " lifecycle"
	return s
}

// verbOf names a step by what its method does to the resource; "" for a method with no such name.
func verbOf(method string) string {
	switch method {
	case "POST":
		return "create"
	case "GET":
		return "get"
	case "HEAD":
		return "check"
	case "PUT":
		return "update"
	case "PATCH":
		return "patch"
	case "DELETE":
		return "delete"
	}
	return ""
}

// stepID is a step id that reads as what the step does, made unique within the flow.
func stepID(step synth.Step, first bool, taken map[string]bool) string {
	id := verbOf(step.Method)
	if id == "" || (first && step.Method != "POST") {
		id = strings.ToLower(step.Method)
	}
	for n := 2; taken[id]; n++ {
		id = fmt.Sprintf("%s-%d", verbOf(step.Method), n)
	}
	taken[id] = true
	return id
}

// AcceptSuggestion saves a suggestion as a flow of the collection, under its path; if that is taken the next free
// name (suggested/pets-2, …) is used. Nothing already saved is changed.
func (w *Workspace) AcceptSuggestion(collection model.ID, s FlowSuggestion) (FlowTarget, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return FlowTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	target, err := w.NewFlow(collection, freeFlowPath(c, s.Path))
	if err != nil {
		return FlowTarget{}, err
	}
	_, f, err := w.flowFor(target)
	if err != nil {
		return FlowTarget{}, err
	}
	f.Name, f.Nodes = s.Flow.Name, s.Flow.Nodes
	if err := w.SaveFlow(target, f, SaveOptions{}); err != nil {
		return FlowTarget{}, fmt.Errorf("saving suggested flow %s: %w", s.Path, err)
	}
	return target, nil
}

// freeFlowPath is path, or path-2, path-3, … the first no flow of c has.
func freeFlowPath(c model.Collection, path string) string {
	taken := func(p string) bool {
		return slices.ContainsFunc(c.Flows, func(f model.Flow) bool { return f.Path == p })
	}
	free := path
	for n := 2; taken(free); n++ {
		free = fmt.Sprintf("%s-%d", path, n)
	}
	return free
}
