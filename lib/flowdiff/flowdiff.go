// SPDX-License-Identifier: Apache-2.0

// Package flowdiff compares two versions of a flow step by step. Steps have stable ids, so a step that was renamed,
// moved or rewired is reported as that, not as a deleted block of text and an added one.
//
// Before: a reviewer of a pull request saw the raw YAML of the flow file, where one step moved to another place
// showed as dozens of removed and added lines.
package flowdiff

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/model"
)

// Kind says what happened to a step (or to the flow itself).
type Kind string

// The kinds of change, in the order a step's changes are listed.
const (
	Added   Kind = "added"
	Removed Kind = "removed"
	Retyped Kind = "retyped"
	Renamed Kind = "renamed"
	Moved   Kind = "moved"
	Changed Kind = "changed"
)

// Change is one difference. Step is the id of the step it is about ("" for the flow itself) and Path the same step
// written as parent/child through the containers that hold it. Field names what changed in a Changed or Retyped
// entry; From and To are the old and new values as text ("" when there was none).
type Change struct {
	Step  string
	Path  string
	Kind  Kind
	Field string
	From  string
	To    string
}

// String is the change as one line of text.
func (c Change) String() string {
	who := "flow"
	if c.Step != "" {
		who = "step " + c.Path
	}
	switch c.Kind {
	case Added:
		if c.Field != "" {
			return fmt.Sprintf("%s: %s added: %s", who, c.Field, show(c.To))
		}
		return fmt.Sprintf("%s added (%s)", who, c.To)
	case Removed:
		if c.Field != "" {
			return fmt.Sprintf("%s: %s removed: %s", who, c.Field, show(c.From))
		}
		return fmt.Sprintf("%s removed (%s)", who, c.From)
	case Moved:
		return fmt.Sprintf("%s moved from %s to %s", who, c.From, c.To)
	case Renamed:
		return fmt.Sprintf("%s renamed from %s to %s", who, show(c.From), show(c.To))
	case Retyped:
		return fmt.Sprintf("%s changed type from %s to %s", who, c.From, c.To)
	}
	return fmt.Sprintf("%s: %s changed from %s to %s", who, c.Field, show(c.From), show(c.To))
}

func show(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}

// Diff lists what changed from a to b. Steps are matched by id wherever they sit in the flow. The result is in a
// fixed order: the flow's own changes, then steps in the order of b (removed steps after, in the order of a).
func Diff(a, b model.Flow) []Change {
	out := flowChanges(a, b)
	sa, sb := index(a.Nodes), index(b.Nodes)
	moved := movedSteps(sa, sb)
	for _, s := range sb.order {
		old, ok := sa.byID[s]
		now := sb.byID[s]
		if !ok {
			out = append(out, Change{Step: s, Path: now.path, Kind: Added, To: describe(now.node)})
			continue
		}
		out = append(out, stepChanges(old, now, moved[s])...)
	}
	for _, s := range sa.order {
		if _, ok := sb.byID[s]; !ok {
			old := sa.byID[s]
			out = append(out, Change{Step: s, Path: old.path, Kind: Removed, From: describe(old.node)})
		}
	}
	return out
}

// describe is how an added or removed step is named: its type and its name when it has one.
func describe(n model.Node) string {
	if n.Name != "" {
		return fmt.Sprintf("%s %q", n.Type, n.Name)
	}
	return string(n.Type)
}

// entry is a step with where it sits: the list that holds it (parent id and which list of it) and its position.
type entry struct {
	node   model.Node
	path   string
	parent string // "" for the flow's own list; else "<parent id>/<list label>"
	pos    int
}

type steps struct {
	byID  map[string]entry
	order []string
	lists map[string][]string // list key -> ids in order
}

func index(nodes []model.Node) steps {
	s := steps{byID: map[string]entry{}, lists: map[string][]string{}}
	var walk func(list []model.Node, parent, prefix string)
	walk = func(list []model.Node, parent, prefix string) {
		for i, n := range list {
			path := prefix + n.ID
			s.byID[n.ID] = entry{node: n, path: path, parent: parent, pos: i}
			s.order = append(s.order, n.ID)
			s.lists[parent] = append(s.lists[parent], n.ID)
			for _, l := range labelled(n) {
				walk(l.nodes, n.ID+"/"+l.label, path+"/")
			}
		}
	}
	walk(nodes, "", "")
	return s
}

type labelledList struct {
	label string
	nodes []model.Node
}

// labelled are a node's nested lists with a name for each, so a step moved from "then" to "else" is a move.
func labelled(n model.Node) []labelledList {
	var out []labelledList
	add := func(label string, nodes []model.Node) {
		if len(nodes) > 0 {
			out = append(out, labelledList{label, nodes})
		}
	}
	add("body", n.Body)
	for _, b := range n.Branches {
		add("branch "+b.ID, b.Nodes)
	}
	add("then", n.Then)
	add("else", n.Else)
	for i, c := range n.Cases {
		add(fmt.Sprintf("case %d", i+1), c.Nodes)
	}
	return out
}

// movedSteps are the ids of steps in both versions whose place changed: they sit in another list, or, in the same
// list, are not part of the longest run of steps that kept their relative order (so one step taken to the end is
// the one reported, not every step it passed).
func movedSteps(a, b steps) map[string]string {
	moved := map[string]string{}
	place := func(s steps, id string) string {
		e := s.byID[id]
		return where(e)
	}
	for id, old := range a.byID {
		if now, ok := b.byID[id]; ok && old.parent != now.parent {
			moved[id] = place(a, id) + "|" + place(b, id)
		}
	}
	for parent, bl := range b.lists {
		var common []string // ids in both versions that stayed in this list
		for _, id := range bl {
			if old, ok := a.byID[id]; ok && old.parent == parent {
				common = append(common, id)
			}
		}
		var inA []string
		for _, id := range a.lists[parent] {
			if slices.Contains(common, id) {
				inA = append(inA, id)
			}
		}
		keep := lcs(inA, common)
		for _, id := range common {
			if !keep[id] {
				moved[id] = place(a, id) + "|" + place(b, id)
			}
		}
	}
	return moved
}

func where(e entry) string {
	if e.parent == "" {
		return fmt.Sprintf("position %d", e.pos+1)
	}
	return fmt.Sprintf("%s, position %d", strings.Replace(e.parent, "/", " (", 1)+")", e.pos+1)
}

// lcs is the set of items in a longest common subsequence of x and y (both hold the same distinct items).
func lcs(x, y []string) map[string]bool {
	n, m := len(x), len(y)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if x[i] == y[j] {
				t[i][j] = t[i+1][j+1] + 1
			} else {
				t[i][j] = max(t[i+1][j], t[i][j+1])
			}
		}
	}
	keep := map[string]bool{}
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case x[i] == y[j]:
			keep[x[i]] = true
			i++
			j++
		case t[i+1][j] >= t[i][j+1]:
			i++
		default:
			j++
		}
	}
	return keep
}

func stepChanges(a, b entry, move string) []Change {
	var out []Change
	c := collector{step: b.node.ID, path: b.path, out: &out}
	if a.node.Type != b.node.Type {
		out = append(out, Change{Step: c.step, Path: c.path, Kind: Retyped, From: string(a.node.Type), To: string(b.node.Type)})
	}
	if a.node.Name != b.node.Name {
		out = append(out, Change{Step: c.step, Path: c.path, Kind: Renamed, From: a.node.Name, To: b.node.Name})
	}
	if move != "" {
		from, to, _ := strings.Cut(move, "|")
		out = append(out, Change{Step: c.step, Path: c.path, Kind: Moved, From: from, To: to})
	}
	x, y := a.node, b.node
	c.text("description", x.Description, y.Description)
	c.list("runs after", x.Needs, y.Needs)
	c.list("runs after failure of", x.AfterFailureOf, y.AfterFailureOf)
	c.list("runs after outcome", x.AfterOutcome, y.AfterOutcome)
	c.text("unlinked", fmt.Sprint(x.Unlinked), fmt.Sprint(y.Unlinked), "false")
	c.text("when", x.When, y.When)
	c.text("concurrent", fmt.Sprint(x.Concurrent), fmt.Sprint(y.Concurrent), "false")
	c.text("on_error", x.OnError, y.OnError)
	c.text("timeout", dur(x.Timeout), dur(y.Timeout), "0s")
	c.text("attach", x.Attach, y.Attach)
	c.text("attach_at", x.AttachAt, y.AttachAt)
	c.text("colour", x.Color, y.Color)
	requestChanges(&c, x.Request, y.Request)
	subflowChanges(&c, x.Subflow, y.Subflow)
	c.keyed("value", valueMap(x.Values), valueMap(y.Values))
	c.keyed("param", fieldMap(x.Params), fieldMap(y.Params))
	c.list("branches", branchIDs(x), branchIDs(y))
	c.keyed("case", caseMap(x.Cases), caseMap(y.Cases))
	return out
}

func dur(d time.Duration) string { return d.String() }

func requestChanges(c *collector, a, b *model.RequestCall) {
	if a == nil && b == nil {
		return
	}
	var x, y model.RequestCall
	if a != nil {
		x = *a
	}
	if b != nil {
		y = *b
	}
	c.text("request", refText(x.Request), refText(y.Request))
	c.text("request version", x.Version, y.Version)
	c.text("overlay", x.Overlay, y.Overlay)
	c.text("open", x.Open, y.Open)
	c.text("over", x.Over, y.Over)
	c.text("close", fmt.Sprint(x.Close), fmt.Sprint(y.Close), "false")
	c.text("hooks", hookText(x.Hooks), hookText(y.Hooks), "all")
	c.keyed("with", pairs(x.With), pairs(y.With))
	c.keyed("header", pairs(x.Headers), pairs(y.Headers))
	c.keyed("outcome", outcomes(x.Outcomes), outcomes(y.Outcomes))
}

func subflowChanges(c *collector, a, b *model.SubflowCall) {
	if a == nil && b == nil {
		return
	}
	var x, y model.SubflowCall
	if a != nil {
		x = *a
	}
	if b != nil {
		y = *b
	}
	c.text("subflow", refText(x.Flow), refText(y.Flow))
	c.keyed("with", pairs(x.With), pairs(y.With))
}

func refText(r model.Ref) string {
	switch {
	case r.Param != "":
		return "parameter " + r.Param
	case r.Path != "" && r.Collection != "":
		return r.Collection + ":" + r.Path
	case r.Path != "":
		return r.Path
	}
	return string(r.ID)
}

func hookText(h model.HookControl) string {
	s := cmp.Or(h.Mode, "all")
	if len(h.Skip) > 0 {
		s += " skipping " + strings.Join(h.Skip, ", ")
	}
	return s
}

func pairs(kv []model.KeyValue) map[string]string {
	m := make(map[string]string, len(kv))
	for _, p := range kv {
		m[p.Key] = p.Value
	}
	return m
}

func fieldMap(fs []model.Field) map[string]string {
	m := make(map[string]string, len(fs))
	for _, f := range fs {
		m[f.Key] = fmt.Sprint(f.Value)
	}
	return m
}

func valueMap(vs []model.SetValue) map[string]string {
	m := make(map[string]string, len(vs))
	for _, v := range vs {
		parts := []string{cmp.Or(v.From, "expr")}
		switch v.From {
		case "response":
			parts = append(parts, v.Where, v.Path, v.Header, v.Cookie, v.Regex)
		default:
			parts = append(parts, fmt.Sprint(v.Value))
		}
		m[v.Name] = strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), " ")
	}
	return m
}

func outcomes(os []model.Outcome) map[string]string {
	m := make(map[string]string, len(os))
	for _, o := range os {
		s := o.When
		if o.Disabled {
			s += " (disabled)"
		}
		m[o.Name] = s
	}
	return m
}

func caseMap(cs []model.Case) map[string]string {
	m := make(map[string]string, len(cs))
	for i, c := range cs {
		m[fmt.Sprintf("%d", i+1)] = c.When
	}
	return m
}

func branchIDs(n model.Node) []string {
	ids := make([]string, len(n.Branches))
	for i, b := range n.Branches {
		ids[i] = b.ID
	}
	return ids
}

// collector gathers the Changed, Added and Removed lines of one step (or of the flow when step is "").
type collector struct {
	step, path string
	out        *[]Change
}

// text records a change of one value; an unset value may be written as zero ("false" for a flag), so a flag that
// went from unset to false is not a change.
func (c *collector) text(field, from, to string, zero ...string) {
	if from == to {
		return
	}
	if len(zero) > 0 && (from == "" || from == zero[0]) && (to == "" || to == zero[0]) {
		return
	}
	*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Changed, Field: field, From: from, To: to})
}

// list records the entries added to or removed from a list of names (order is not reported: it does not change
// what a step does).
func (c *collector) list(field string, from, to []string) {
	for _, s := range to {
		if !slices.Contains(from, s) {
			*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Added, Field: field, To: s})
		}
	}
	for _, s := range from {
		if !slices.Contains(to, s) {
			*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Removed, Field: field, From: s})
		}
	}
}

// keyed records, key by key, what was added, removed or changed in a set of named values.
func (c *collector) keyed(field string, from, to map[string]string) {
	for _, k := range slices.Sorted(maps.Keys(to)) {
		old, had := from[k]
		switch {
		case !had:
			*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Added, Field: field + " " + k, To: to[k]})
		case old != to[k]:
			*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Changed, Field: field + " " + k, From: old, To: to[k]})
		}
	}
	for _, k := range slices.Sorted(maps.Keys(from)) {
		if _, has := to[k]; !has {
			*c.out = append(*c.out, Change{Step: c.step, Path: c.path, Kind: Removed, Field: field + " " + k, From: from[k]})
		}
	}
}

func flowChanges(a, b model.Flow) []Change {
	var out []Change
	c := collector{out: &out}
	c.text("name", a.Name, b.Name)
	c.text("description", a.Description, b.Description)
	c.list("tag", a.Tags, b.Tags)
	c.text("prefer_version", a.PreferVersion, b.PreferVersion)
	c.keyed("input", inputMap(a.Inputs), inputMap(b.Inputs))
	c.keyed("output", outputMap(a.Outputs), outputMap(b.Outputs))
	c.text("max concurrency", fmt.Sprint(a.Execution.MaxConcurrency), fmt.Sprint(b.Execution.MaxConcurrency), "0")
	c.text("rate limit", a.Execution.RateLimit, b.Execution.RateLimit)
	c.keyed("host rate limit", pairs(a.Execution.HostRateLimits), pairs(b.Execution.HostRateLimits))
	c.text("on branch error", a.Execution.OnBranchError, b.Execution.OnBranchError)
	c.text("run till", runTill(a.Run), runTill(b.Run))
	return out
}

func inputMap(in []model.Input) map[string]string {
	m := make(map[string]string, len(in))
	for _, i := range in {
		text := i.Type
		if i.Default != "" {
			text = strings.TrimSpace(text + " default " + i.Default)
		}
		m[i.Name] = text
	}
	return m
}

func outputMap(out []model.Output) map[string]string {
	m := make(map[string]string, len(out))
	for _, o := range out {
		m[o.Name] = strings.TrimSpace(o.Type + " " + o.Value)
	}
	return m
}

func runTill(r *model.RunTill) string {
	if r == nil {
		return ""
	}
	return fmt.Sprintf("until %q, at most %d times, %s, every %s", r.Until, r.MaxIterations, r.MaxDuration, r.Interval)
}
