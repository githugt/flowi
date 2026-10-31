// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"regexp"
	"slices"
	"strings"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/report"
)

// Coverage says how much of an API description the flows use: which operations some flow calls, which of their
// documented statuses a run has ever got, and which response fields some flow reads.
type Coverage struct {
	Operations []OperationCoverage
}

// OperationCoverage is one operation of the description: a request version that came from it.
type OperationCoverage struct {
	Collection string
	Request    string // the request's path
	Version    string
	Used       []string     // the flows and hooks that call it
	Statuses   []StatusSeen // the statuses the description documents
	Read       []string     // response fields a flow reads, as JSONPaths ($.user.id)
	Unread     []string     // response fields no flow reads
}

// StatusSeen is a documented status and whether a recorded run ever got it.
type StatusSeen struct {
	Status int
	Seen   bool
}

// Totals counts what is covered.
type Totals struct {
	Operations, Used       int
	Statuses, SeenStatuses int
	Fields, ReadFields     int
}

// Totals adds the operations up.
func (c Coverage) Totals() Totals {
	var t Totals
	for _, op := range c.Operations {
		t.Operations++
		if len(op.Used) > 0 {
			t.Used++
		}
		t.Statuses += len(op.Statuses)
		for _, s := range op.Statuses {
			if s.Seen {
				t.SeenStatuses++
			}
		}
		t.Fields += len(op.Read) + len(op.Unread)
		t.ReadFields += len(op.Read)
	}
	return t
}

// Cover measures the flows of cs against the operations of their API descriptions. An operation is a request
// version that came from a description (it has an operation id) and has response schemas. Statuses are seen in
// runs, the saved run reports passed in; fields are read by the flows' templates, extracts and checks.
func Cover(cs []model.Collection, src Contracts, runs []report.Run) Coverage {
	ix := newIndex(cs)
	seen := statusesSeen(runs)
	var out Coverage
	for _, c := range cs {
		for _, r := range c.Requests {
			if r.Operation == "" {
				continue
			}
			for _, v := range r.Versions {
				contract, _ := src.Contract(c, r, v.Label)
				if contract.Source != SourceSchema {
					continue
				}
				out.Operations = append(out.Operations, operationCoverage(ix, c, r, v, contract, seen))
			}
		}
	}
	return out
}

func operationCoverage(ix *index, c model.Collection, r model.Request, v model.RequestVersion, contract Contract, seen map[seenKey]map[int]bool) OperationCoverage {
	op := OperationCoverage{Collection: c.Manifest.Name, Request: r.Path, Version: v.Label}
	cons := consumers(ix, c, r, v)
	var reads []string
	for _, k := range cons {
		if name := consumerLabel(k); name != "" && !slices.Contains(op.Used, name) {
			op.Used = append(op.Used, name)
		}
		for _, rd := range k.Reads {
			if rd.Kind == ReadBody || rd.Kind == ReadJSONPath {
				reads = append(reads, normalisePath(rd.Target))
			}
		}
	}
	slices.Sort(op.Used)
	for _, s := range contract.Samples {
		op.Statuses = append(op.Statuses, StatusSeen{Status: s.Status, Seen: seen[seenKey{request: r.ID, version: v.Label}][s.Status]})
	}
	slices.SortFunc(op.Statuses, func(a, b StatusSeen) int { return a.Status - b.Status })
	op.Read, op.Unread = splitFields(successFields(contract.Samples), reads)
	return op
}

// consumerLabel is the flow a consumer is written in. A hook that runs around many requests does not make
// any of them "used by a flow", and the request's own extract and assert have no flow.
func consumerLabel(c Consumer) string { return c.Flow }

// successFields lists the leaf fields of the 2xx response schemas, as JSONPaths, sorted and without repeats.
func successFields(samples []Sample) []string {
	var out []string
	for _, s := range samples {
		if s.Status >= 200 && s.Status < 300 {
			out = append(out, schemaLeaves(s.Schema, "$")...)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// schemaLeaves lists the paths of the scalar fields of a JSON Schema, with list items written [*]. Every branch
// of allOf, anyOf and oneOf counts, since any of them may be what the server sends.
func schemaLeaves(schema any, path string) []string {
	m, ok := schema.(map[string]any)
	if !ok {
		return []string{path}
	}
	var out []string
	for _, key := range []string{"allOf", "anyOf", "oneOf"} {
		if branches, ok := m[key].([]any); ok {
			for _, b := range branches {
				out = append(out, schemaLeaves(b, path)...)
			}
		}
	}
	if props, ok := m["properties"].(map[string]any); ok {
		for name, sub := range props {
			out = append(out, schemaLeaves(sub, path+"."+name)...)
		}
	}
	if items, ok := m["items"]; ok {
		out = append(out, schemaLeaves(items, path+"[*]")...)
	}
	if len(out) == 0 && path != "$" {
		return []string{path}
	}
	return out
}

var (
	indexStep   = regexp.MustCompile(`\[\d+\]`)
	numericStep = regexp.MustCompile(`\.\d+(\.|$)`)
)

// normalisePath writes a read's JSONPath the way schemaLeaves does: list positions become [*].
func normalisePath(p string) string {
	p = indexStep.ReplaceAllString(p, "[*]")
	for numericStep.MatchString(p) {
		p = numericStep.ReplaceAllString(p, "[*]$1")
	}
	return p
}

// splitFields puts each field in read when some read names it or a part of it above it ($.user covers
// $.user.id), and in unread otherwise.
func splitFields(fields, reads []string) (read, unread []string) {
	for _, f := range fields {
		if slices.ContainsFunc(reads, func(r string) bool { return f == r || strings.HasPrefix(f, r+".") || strings.HasPrefix(f, r+"[") }) {
			read = append(read, f)
		} else {
			unread = append(unread, f)
		}
	}
	return read, unread
}

// seenKey names a request version in a run report, which knows the request's id and not its collection.
type seenKey struct {
	request model.ID
	version string
}

// statusesSeen collects, for each request version, every status a recorded run got from it.
func statusesSeen(runs []report.Run) map[seenKey]map[int]bool {
	out := map[seenKey]map[int]bool{}
	for _, run := range runs {
		used := map[string]seenKey{}
		for _, u := range run.Versions {
			used[u.Path] = seenKey{request: model.ID(u.RequestID), version: u.Version}
		}
		walkNodes(run.Nodes, func(n report.Node) {
			key, ok := used[n.Path]
			if !ok {
				return
			}
			for _, a := range n.Attempts {
				if a.Status == 0 {
					continue
				}
				if out[key] == nil {
					out[key] = map[int]bool{}
				}
				out[key][a.Status] = true
			}
		})
	}
	return out
}

// walkNodes calls f for every node, inside lanes and hook flows too.
func walkNodes(nodes []report.Node, f func(report.Node)) {
	for _, n := range nodes {
		f(n)
		for _, l := range n.Lanes {
			walkNodes(l.Nodes, f)
		}
		for _, h := range n.Hooks {
			walkNodes(h.Nodes, f)
		}
	}
}
