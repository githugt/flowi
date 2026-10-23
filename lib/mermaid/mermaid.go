// SPDX-License-Identifier: Apache-2.0

// Package mermaid reads Mermaid flowcharts as flow sketches and writes
// flows as Mermaid. Node labels follow conventions ("req: auth/login@2",
// "delay: 2s", "extract: token=$.data.token") that resolve against the
// collection; what does not resolve becomes a placeholder note, so a
// sketch always imports and shows what is left to do.
package mermaid

import (
	"cmp"
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/importers"
	"github.com/githugt/flowi-lib/model"
)

// Resolver finds requests and flows by path.
type Resolver interface {
	Request(path string) (model.Ref, bool)
	Flow(path string) (model.Ref, bool)
}

// Import reads a flowchart as one flow, mermaid/<name stem>. A syntax
// error (FLW-IMPORT-004) or a cycle (FLW-MERMAID-002) imports nothing.
func Import(src []byte, name string, r Resolver) (importers.Result, diag.List) {
	g, err := parse(src)
	if err != nil {
		se := err.(*syntaxError) //nolint:errorlint // parse returns only *syntaxError
		d := diag.New(diag.CodeImportSyntax, diag.Location{File: name, Pos: se.pos}, map[string]string{"format": "Mermaid", "detail": se.detail})
		d.Excerpt = diag.ExcerptAt(src, se.pos)
		return importers.Result{}, diag.List{d}
	}
	order, cycle := topological(g)
	if len(cycle) > 0 {
		first := g.byID[cycle[0]]
		pos := diag.Position{Line: first.line, Column: first.col}
		d := diag.New(diag.CodeMermaidCycle, diag.Location{File: name, Pos: pos}, map[string]string{"nodes": strings.Join(cycle, ", ")})
		d.Excerpt = diag.ExcerptAt(src, pos)
		return importers.Result{}, diag.List{d}
	}
	it := importers.Item{Name: "diagram " + name, Source: diag.Location{File: name, Pos: diag.Position{Line: 1, Column: 1}}}
	reportUnused(&it, g)
	m := &mapper{src: src, file: name, r: r, it: &it, last: map[string]string{}}
	var nodes []model.Node
	for i, n := range order {
		var prev *gnode
		if i > 0 {
			prev = order[i-1]
		}
		nodes = append(nodes, m.place(n, prev, g)...)
	}
	stem := strings.TrimSuffix(path.Base(name), path.Ext(name))
	flowName := cmp.Or(importers.FileName(stem), "sketch")
	f := model.Flow{Meta: model.Meta{Name: flowName}, Path: "mermaid/" + flowName, Nodes: nodes}
	return importers.Result{Flows: []model.Flow{f}, Items: []importers.Item{it}}, nil
}

// topological orders nodes so every edge points forward. Among nodes
// ready to run, response readers (extract, assert) go first: they read
// the latest response, so nothing may run between a request and them.
// Other ties are broken by first appearance. Nodes left over are on a
// cycle.
func topological(g *graph) ([]*gnode, []string) {
	indegree := map[string]int{}
	out := map[string][]string{}
	for _, e := range g.edges {
		indegree[e.to]++
		out[e.from] = append(out[e.from], e.to)
	}
	var ready, order []*gnode
	for _, n := range g.nodes {
		if indegree[n.id] == 0 {
			ready = append(ready, n)
		}
	}
	for len(ready) > 0 {
		slices.SortFunc(ready, func(a, b *gnode) int {
			if ra, rb := readsResponse(a), readsResponse(b); ra != rb {
				if ra {
					return -1
				}
				return 1
			}
			return a.order - b.order
		})
		n := ready[0]
		ready = ready[1:]
		order = append(order, n)
		for _, to := range out[n.id] {
			if indegree[to]--; indegree[to] == 0 {
				ready = append(ready, g.byID[to])
			}
		}
	}
	var cycle []string
	for _, n := range g.nodes {
		if indegree[n.id] > 0 {
			cycle = append(cycle, n.id)
		}
	}
	return order, cycle
}

// readsResponse reports labels that become nodes reading the latest
// response.
func readsResponse(n *gnode) bool {
	prefix, _, ok := strings.Cut(n.label, ":")
	p := strings.ToLower(strings.TrimSpace(prefix))
	return ok && (p == "extract" || p == "assert")
}

func reportUnused(it *importers.Item, g *graph) {
	for _, s := range g.subgraphs {
		it.Unmapped("subgraph "+s, "left out", "its nodes are imported; the grouping is not")
	}
	if labels := slices.Compact(slices.Sorted(slices.Values(g.edgeLabels))); len(labels) > 0 {
		it.Unmapped("edge labels ("+strings.Join(labels, ", ")+")", "left out", "Flow-I edges have no labels; use a condition node for branches")
	}
}

// mapper turns diagram nodes into flow nodes.
type mapper struct {
	src  []byte
	file string
	r    Resolver
	it   *importers.Item
	ids  importers.Names
	last map[string]string // diagram id → its last flow node id
}

var unsafeID = regexp.MustCompile(`[^A-Za-z0-9_-]`)

// place maps n and sets needs where the drawing differs from sequence:
// a node whose only predecessor is the node before it needs nothing.
func (m *mapper) place(n, prev *gnode, g *graph) []model.Node {
	nodes := m.nodes(n)
	base := cmp.Or(unsafeID.ReplaceAllString(n.id, "-"), "node")
	for i := range nodes {
		nodes[i].ID = m.ids.Unique(base)
	}
	var preds []string
	for _, e := range g.edges {
		if e.to == n.id && !slices.Contains(preds, e.from) {
			preds = append(preds, e.from)
		}
	}
	slices.SortFunc(preds, func(a, b string) int { return g.byID[a].order - g.byID[b].order })
	onlyPrevious := len(preds) == 1 && prev != nil && preds[0] == prev.id
	if len(preds) > 0 && !onlyPrevious {
		for _, p := range preds {
			nodes[0].Needs = append(nodes[0].Needs, m.last[p])
		}
	}
	m.last[n.id] = nodes[len(nodes)-1].ID
	return nodes
}

// nodes maps one diagram node by its label's convention.
func (m *mapper) nodes(n *gnode) []model.Node {
	prefix, rest, has := strings.Cut(n.label, ":")
	rest = strings.TrimSpace(rest)
	switch strings.ToLower(strings.TrimSpace(prefix)) {
	case "req", "request":
		if has {
			return m.request(n, rest)
		}
	case "subflow", "flow":
		if has {
			ref, ok := m.r.Flow(rest)
			if !ok {
				return m.placeholder(n, "no flow "+rest+" in this collection")
			}
			return []model.Node{{Type: "subflow", Subflow: &model.SubflowCall{Flow: ref}}}
		}
	case "delay":
		if has {
			return m.delay(n, rest)
		}
	case "extract":
		if has {
			return m.extract(n, rest)
		}
	case "set":
		if name, value, ok := strings.Cut(rest, "="); has && ok && identifier(strings.TrimSpace(name)) {
			return []model.Node{{Type: "set", Params: []model.Field{{Key: "target", Value: "vars." + strings.TrimSpace(name)}, {Key: "value", Value: strings.TrimSpace(value)}}}}
		}
	case "assert":
		if has && rest != "" {
			return []model.Node{{Type: "assert", Params: []model.Field{{Key: "check", Value: rest}}}}
		}
	case "log":
		if has && rest != "" {
			return []model.Node{{Type: "log", Params: []model.Field{{Key: "value", Value: rest}}}}
		}
	case "note":
		if has {
			return []model.Node{{Type: "note", Params: []model.Field{{Key: "text", Value: rest}}}}
		}
	}
	if ref, ok := m.r.Request(strings.TrimSpace(n.label)); ok {
		return []model.Node{{Type: "request", Request: &model.RequestCall{Request: ref}}}
	}
	return m.placeholder(n, "no label convention matches; write req: <path>, delay: 2s, extract: name=$.path, set: name=value, assert: …, log: …, note: …")
}

func (m *mapper) request(n *gnode, rest string) []model.Node {
	reqPath, version, _ := strings.Cut(rest, "@")
	ref, ok := m.r.Request(strings.TrimSpace(reqPath))
	if !ok {
		return m.placeholder(n, "no request "+strings.TrimSpace(reqPath)+" in this collection")
	}
	return []model.Node{{Type: "request", Request: &model.RequestCall{Request: ref, Version: strings.TrimSpace(version)}}}
}

func (m *mapper) delay(n *gnode, rest string) []model.Node {
	low, high, isRange := strings.Cut(rest, "..")
	_, errLow := time.ParseDuration(low)
	_, errHigh := time.ParseDuration(high)
	if errLow != nil || isRange && errHigh != nil {
		return m.placeholder(n, rest+" is not a duration such as 2s or 1s..3s")
	}
	return []model.Node{{Type: "delay", Params: []model.Field{{Key: "duration", Value: rest}}}}
}

// extract maps "name=$.path, …" to one single-form extract per pair.
func (m *mapper) extract(n *gnode, rest string) []model.Node {
	var out []model.Node
	for _, pair := range strings.Split(rest, ",") {
		name, jsonPath, ok := strings.Cut(pair, "=")
		name, jsonPath = strings.TrimSpace(name), strings.TrimSpace(jsonPath)
		if !ok || !identifier(name) || !strings.HasPrefix(jsonPath, "$") {
			return m.placeholder(n, "write extract: name=$.json.path, separated by commas")
		}
		out = append(out, model.Node{Type: "extract", Params: []model.Field{{Key: "name", Value: name}, {Key: "from", Value: "body"}, {Key: "path", Value: jsonPath}}})
	}
	return out
}

func (m *mapper) placeholder(n *gnode, detail string) []model.Node {
	pos := diag.Position{Line: n.line, Column: n.col}
	d := diag.New(diag.CodeMermaidPlaceholder, diag.Location{File: m.file, Pos: pos}, map[string]string{"label": n.label, "detail": detail})
	d.Excerpt = diag.ExcerptAt(m.src, pos)
	m.it.Report(d)
	return []model.Node{{Type: "note", Params: []model.Field{{Key: "text", Value: "placeholder: " + n.label}}}}
}

var identifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func identifier(s string) bool { return identifierRE.MatchString(s) }

// Export writes f as a flowchart. Supported nodes get convention labels
// and import back as themselves; containers become subgraphs (export
// only); other nodes are labelled "<type> <id>" and import as
// placeholders.
func Export(f model.Flow) string {
	e := &exporter{ids: map[string]string{}}
	var b strings.Builder
	fmt.Fprintf(&b, "flowchart LR\n  %%%% Flow-I flow %s; labels follow Flow-I's Mermaid conventions\n", f.Path)
	if slices.ContainsFunc(f.Nodes, func(n model.Node) bool { return len(n.Children()) > 0 }) {
		b.WriteString("  %% containers are drawn as subgraphs; importing them back gives placeholders\n")
	}
	e.define(&b, f.Nodes, "  ")
	e.edges(&b, f.Nodes, "  ")
	return b.String()
}

type exporter struct {
	ids   map[string]string // flow node id → Mermaid id
	taken importers.Names
}

func reserved(id string) bool {
	return slices.Contains([]string{"end", "subgraph", "graph", "flowchart", "style", "class", "classdef", "click", "linkstyle", "direction"}, strings.ToLower(id))
}

func (e *exporter) id(flowID string) string {
	if id, ok := e.ids[flowID]; ok {
		return id
	}
	id := strings.TrimLeft(unsafeID.ReplaceAllString(flowID, "_"), "-")
	if id == "" || reserved(id) {
		id += "_"
	}
	id = e.taken.Unique(id)
	e.ids[flowID] = id
	return id
}

func (e *exporter) define(b *strings.Builder, nodes []model.Node, indent string) {
	groups := attachGroups(nodes)
	drawn := map[*attachGroup]bool{}
	for _, n := range nodes {
		if g, ok := groups[n.ID]; ok {
			if !drawn[g] {
				drawn[g] = true
				e.defineGroup(b, g, indent)
			}
			continue
		}
		e.defineNode(b, n, indent)
	}
}

func (e *exporter) defineNode(b *strings.Builder, n model.Node, indent string) {
	lanes := n.Children()
	if len(lanes) == 0 {
		fmt.Fprintf(b, "%s%s[\"%s\"]\n", indent, e.id(n.ID), escape(label(n)))
		return
	}
	fmt.Fprintf(b, "%ssubgraph %s [\"%s\"]\n", indent, e.id(n.ID), escape(string(n.Type)+" "+n.ID))
	for _, lane := range lanes {
		e.define(b, lane, indent+"  ")
		e.edges(b, lane, indent+"  ")
	}
	fmt.Fprintf(b, "%send\n", indent)
}

// defineGroup draws a request with the actions shown with it as a subgraph, so the
// grouping the canvas shows survives. Its direction follows the canvas: a stack drawn
// above and below the request runs top to bottom, actions beside it left to right.
func (e *exporter) defineGroup(b *strings.Builder, g *attachGroup, indent string) {
	fmt.Fprintf(b, "%ssubgraph %s [\"%s\"]\n", indent, e.id(g.key()), escape(g.core.ID+" with its actions"))
	fmt.Fprintf(b, "%s  direction %s\n", indent, g.direction())
	for _, n := range g.display() {
		e.defineNode(b, n, indent+"  ")
	}
	fmt.Fprintf(b, "%send\n", indent)
}

// edges draws needs, or sequence for nodes without them. Arrows between a group and
// the rest go to the group's subgraph, which is also what lets Mermaid honour the
// group's direction; arrows inside a group stay between its nodes.
func (e *exporter) edges(b *strings.Builder, nodes []model.Node, indent string) {
	groups := attachGroups(nodes)
	seen := map[string]bool{}
	end := func(flowID string, inside *attachGroup) string {
		if g, ok := groups[flowID]; ok && g != inside {
			return e.id(g.key())
		}
		return e.id(flowID)
	}
	for i, n := range nodes {
		deps := n.Needs
		if len(deps) == 0 && i > 0 {
			deps = []string{nodes[i-1].ID}
		}
		for _, d := range deps {
			var inside *attachGroup
			if g, ok := groups[d]; ok && groups[n.ID] == g {
				inside = g
			}
			arrow := end(d, inside) + " --> " + end(n.ID, inside)
			if end(d, inside) == end(n.ID, inside) || seen[arrow] {
				continue
			}
			seen[arrow] = true
			fmt.Fprintf(b, "%s%s\n", indent, arrow)
		}
	}
}

// attachGroup is a request or subflow and the actions the canvas draws with it.
type attachGroup struct {
	core    model.Node
	members []model.Node // in list order, the core among them
}

func (g *attachGroup) key() string { return "attach:" + g.core.ID }

// direction is top to bottom when every action is stacked above or below, else left to right.
func (g *attachGroup) direction() string {
	for _, n := range g.members {
		if n.ID != g.core.ID && n.AttachAt == "" {
			return "LR"
		}
	}
	return "TB"
}

// display lists the members in the order the canvas draws them: top to bottom for a
// stack, else list order.
func (g *attachGroup) display() []model.Node {
	if g.direction() == "LR" {
		return g.members
	}
	var above, below []model.Node
	for _, n := range g.members {
		switch {
		case n.ID == g.core.ID:
		case n.AttachAt == "below":
			below = append(below, n)
		default:
			above = append(above, n)
		}
	}
	return append(append(above, g.core), below...)
}

// attachGroups maps the id of every member of a group, the core included, to its group.
func attachGroups(nodes []model.Node) map[string]*attachGroup {
	groups := map[string]*attachGroup{}
	for _, n := range nodes {
		if n.Attach == "" || n.Attach == n.ID {
			continue
		}
		core := slices.IndexFunc(nodes, func(c model.Node) bool { return c.ID == n.Attach && c.Attach == "" })
		if core < 0 {
			continue
		}
		if groups[n.Attach] == nil {
			groups[n.Attach] = &attachGroup{core: nodes[core]}
		}
	}
	for _, n := range nodes {
		core := n.ID
		if n.Attach != "" {
			core = n.Attach
		}
		if g := groups[core]; g != nil {
			g.members = append(g.members, n)
			groups[n.ID] = g
		}
	}
	return groups
}

// label is a node's convention label.
func label(n model.Node) string {
	p := func(key string) string {
		for _, f := range n.Params {
			if f.Key == key {
				return fmt.Sprint(f.Value)
			}
		}
		return ""
	}
	switch {
	case n.Type == "request" && n.Request != nil:
		out := "req: " + cmp.Or(n.Request.Request.Path, string(n.Request.Request.ID))
		if n.Request.Version != "" {
			out += "@" + n.Request.Version
		}
		return out
	case n.Type == "subflow" && n.Subflow != nil:
		return "subflow: " + cmp.Or(n.Subflow.Flow.Path, string(n.Subflow.Flow.ID))
	case n.Type == "delay":
		return "delay: " + p("duration")
	case n.Type == "extract" && p("from") == "body" && p("path") != "" && len(n.Params) == 3:
		return "extract: " + p("name") + "=" + p("path")
	case n.Type == "set" && strings.HasPrefix(p("target"), "vars.") && len(n.Params) == 2:
		return "set: " + strings.TrimPrefix(p("target"), "vars.") + "=" + p("value")
	case n.Type == "assert" && len(n.Params) == 1:
		return "assert: " + p("check")
	case n.Type == "log" && len(n.Params) == 1:
		return "log: " + p("value")
	case n.Type == "note":
		return "note: " + p("text")
	}
	return string(n.Type) + " " + n.ID
}

// escape writes a label for a quoted Mermaid shape.
func escape(s string) string {
	return strings.NewReplacer(`"`, "#quot;", "\n", "<br>", ";", "#59;").Replace(s)
}
