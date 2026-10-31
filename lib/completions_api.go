// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// Completion is something a template at a node can read, for editor
// autocomplete: Text is the reference (nodes.login.token), Detail says
// where it comes from.
type Completion struct {
	Text, Detail string
}

// contractDepth is how deep body fields from contract samples are offered.
const contractDepth = 2

// Completions lists what templates of the node at nodePath (its YAML path,
// e.g. "$.nodes[3]") can read: flow inputs, environment keys and secret
// names, variables written by nodes that run before it, and outputs of
// those nodes, including body fields from request contracts (pinned
// examples or the last saved response). Nodes are assumed to run in list
// order (a concurrent node sees the same nodes as a serial one).
func (w *Workspace) Completions(t FlowTarget, nodePath string) ([]Completion, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	return w.CompletionsIn(t.Collection, f, nodePath)
}

// CompletionsIn is Completions for f, a flow of collection as an editor
// holds it (possibly unsaved), so offers follow the edited order.
func (w *Workspace) CompletionsIn(collection model.ID, f model.Flow, nodePath string) ([]Completion, error) {
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	chain, ok := chainTo(f.Nodes, yamlpath.Key("$", "nodes"), nodePath, nil)
	if !ok {
		return nil, fmt.Errorf("flow %s has no node at %s", f.Path, nodePath)
	}
	w.mu.RLock()
	cs := slices.Clone(w.collections)
	w.mu.RUnlock()
	b := completionSet{}
	b.addFlow(c, f)
	readable := readableBefore(chain)
	for name := range sites.WrittenVars(readable) {
		b.add("vars."+name, "variable")
	}
	for _, n := range readable {
		b.addNode(w, cs, c, f, n)
	}
	if inBatch(chain) {
		b.add("item", "batch item")
		b.add("index", "batch index")
	}
	return b.sorted(), nil
}

// frame is one level of the path to a node: its list and position.
type frame struct {
	list  []model.Node
	index int
}

// chainTo finds the node at target and returns the frames from the top
// level down to it.
func chainTo(list []model.Node, listPath, target string, chain []frame) ([]frame, bool) {
	for i, n := range list {
		here := append(slices.Clone(chain), frame{list: list, index: i})
		p := yamlpath.Index(listPath, i)
		if p == target {
			return here, true
		}
		for _, child := range sites.ChildLists(n, p) {
			if found, ok := chainTo(child.Value, child.Path, target, here); ok {
				return found, true
			}
		}
	}
	return nil, false
}

// readableBefore is every node that has finished before the last frame's
// node starts: earlier siblings at each level and what they make visible.
func readableBefore(chain []frame) []model.Node {
	var out []model.Node
	for _, fr := range chain {
		for _, n := range fr.list[:fr.index] {
			out = append(out, n)
			out = append(out, visibleNodes(n)...)
		}
	}
	return out
}

// visibleNodes are the nodes inside n readable after it (not those of
// repeated bodies; see sites.VisibleDescendants).
func visibleNodes(n model.Node) []model.Node {
	ids := sites.VisibleDescendants(n)
	var out []model.Node
	var walk func(list []model.Node)
	walk = func(list []model.Node) {
		for _, c := range list {
			if ids[c.ID] {
				out = append(out, c)
			}
			for _, child := range c.Children() {
				walk(child)
			}
		}
	}
	for _, child := range n.Children() {
		walk(child)
	}
	return out
}

func inBatch(chain []frame) bool {
	for _, fr := range chain[:len(chain)-1] {
		if fr.list[fr.index].Type == "batch" {
			return true
		}
	}
	return false
}

// completionSet collects completions without duplicates.
type completionSet map[string]string

func (s completionSet) add(text, detail string) {
	if _, seen := s[text]; !seen {
		s[text] = detail
	}
}

func (s completionSet) sorted() []Completion {
	out := make([]Completion, 0, len(s))
	for _, text := range slices.Sorted(maps.Keys(s)) {
		out = append(out, Completion{Text: text, Detail: s[text]})
	}
	return out
}

func (s completionSet) addFlow(c model.Collection, f model.Flow) {
	for _, in := range f.Inputs {
		s.add("inputs."+in.Name, "flow input")
	}
	for _, kv := range c.Manifest.Variables {
		s.add("collection_var."+kv.Key, "collection variable")
	}
	for _, e := range c.Environments {
		for _, v := range e.Vars {
			if v.Secret {
				s.add("secret."+v.Name, "secret ("+e.Name+")")
			} else {
				s.add("env."+v.Name, "environment ("+e.Name+")")
			}
		}
	}
}

// nodeOutputs are outputs every node of a type has, besides nodes.<id>.
func nodeOutputs() map[model.NodeType][]string {
	return map[model.NodeType][]string{
		"request":  {"status", "headers", "body", "time", "vars"},
		"fetch":    {"status", "headers", "body", "time"},
		"batch":    {"results", "passed", "failed"},
		"run_till": {"met", "iterations"},
	}
}

func (s completionSet) addNode(w *Workspace, cs []model.Collection, c model.Collection, f model.Flow, n model.Node) {
	base := "nodes." + n.ID
	s.add(base, string(n.Type)+" node")
	for _, out := range nodeOutputs()[n.Type] {
		s.add(base+"."+out, string(n.Type)+" output")
	}
	if n.Type != "request" || n.Request == nil {
		return
	}
	rc, r, ok := findRequest(cs, c, n.Request.Request)
	if !ok {
		return
	}
	v, ok := sites.ResolvedVersion(r, n.Request.Version, f.PreferVersion)
	if !ok {
		return
	}
	for _, e := range v.Extract {
		s.add(base+"."+e.Name, "extract "+e.Path+e.Header+e.Cookie)
	}
	contract, _ := analysis.FileContracts{DataDir: w.DataDir}.Contract(rc, r, v.Label)
	for _, sample := range contract.Samples {
		s.addFields(base+".body", sample.Body, contractDepth, "from "+contract.Source)
	}
}

// addFields offers the object keys of a sample body, depth levels deep.
func (s completionSet) addFields(prefix string, v any, depth int, detail string) {
	m, ok := v.(map[string]any)
	if !ok || depth == 0 {
		return
	}
	for k, child := range m {
		if strings.ContainsAny(k, " .-[]") {
			continue // not reachable with dotted steps; written with ["…"] instead
		}
		s.add(prefix+"."+k, detail)
		s.addFields(prefix+"."+k, child, depth-1, detail)
	}
}

// findRequest resolves a request reference as seen from collection from:
// an empty collection is from itself, otherwise a depends_on entry.
func findRequest(cs []model.Collection, from model.Collection, ref model.Ref) (model.Collection, model.Request, bool) {
	target := from.Manifest.ID
	if ref.Collection != "" {
		i := slices.IndexFunc(from.Manifest.DependsOn, func(d model.Dependency) bool {
			return d.Name == ref.Collection || string(d.ID) == ref.Collection
		})
		if i < 0 {
			return model.Collection{}, model.Request{}, false
		}
		target = from.Manifest.DependsOn[i].ID
	}
	for _, c := range cs {
		if c.Manifest.ID != target {
			continue
		}
		for _, r := range c.Requests {
			if r.ID == ref.ID {
				return c, r, true
			}
		}
	}
	return model.Collection{}, model.Request{}, false
}

// responseFieldDepth is how deep ResponseFields follows a body.
const responseFieldDepth = 4

// ResponseFields lists the body fields version of a request is known to return, written as an
// expression reads them: body, body.user.id, body.items[0].id. They come from its schema, its
// pinned examples or its last saved response, in that order of preference (an empty version
// is the one a run resolves to); a request with none
// of them has no fields to offer. A list is named by its first item.
func (w *Workspace) ResponseFields(t RequestTarget, version string) ([]Completion, error) {
	w.mu.RLock()
	c, ok := w.collection(t.Collection)
	w.mu.RUnlock()
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(t.Collection)})
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == t.Request })
	if i < 0 {
		return nil, fmt.Errorf("collection %s has no request %s", t.Collection, t.Request)
	}
	r := c.Requests[i]
	if v, ok := sites.ResolvedVersion(r, version, ""); ok {
		version = v.Label // an empty version is the one a run resolves to
	}
	contract, _ := analysis.FileContracts{DataDir: w.DataDir}.Contract(c, r, version)
	seen := map[string]bool{}
	var out []Completion
	add := func(text string) {
		if !seen[text] {
			seen[text] = true
			out = append(out, Completion{Text: text, Detail: "from " + contract.Source})
		}
	}
	for _, sample := range contract.Samples {
		if sample.Body == nil {
			continue
		}
		add("body")
		for _, text := range bodyFieldTexts("body", sample.Body, responseFieldDepth) {
			add(text)
		}
	}
	return out, nil
}

// bodyFieldTexts lists the paths under prefix in a parsed JSON body, depth levels deep, object
// keys in order. Keys that are not plain words are left out: they are written with ["…"].
func bodyFieldTexts(prefix string, v any, depth int) []string {
	if depth == 0 {
		return nil
	}
	var out []string
	switch x := v.(type) {
	case map[string]any:
		for _, k := range slices.Sorted(maps.Keys(x)) {
			if strings.ContainsAny(k, " .-[]") {
				continue
			}
			out = append(out, prefix+"."+k)
			out = append(out, bodyFieldTexts(prefix+"."+k, x[k], depth-1)...)
		}
	case []any:
		if len(x) > 0 {
			out = append(out, prefix+"[0]")
			out = append(out, bodyFieldTexts(prefix+"[0]", x[0], depth-1)...)
		}
	}
	return out
}
