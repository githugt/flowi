// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/internal/sites"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// What a traced name refers to.
const (
	TraceVariable = "variable" // vars.token
	TraceOutput   = "output"   // nodes.login.body.token
	TraceInput    = "input"    // inputs.region
)

// ValueTrace says where one value of a flow comes from and who uses it, from the flow as written:
// it needs no run. A finished run adds the value at each hop.
type ValueTrace struct {
	// Name is the traced name in full (vars.token, nodes.login.body.token, inputs.region).
	Name string `json:"name"`
	Kind string `json:"kind"`
	// Producers are the nodes that make the value, in flow order. A flow input has none.
	Producers []string `json:"producers"`
	// Generated is true when a generator fills the variable: the value is fresh, not taken from another step.
	Generated bool `json:"generated,omitempty"`
	// Readers are the nodes whose templates read the value, in flow order.
	Readers []string `json:"readers"`
}

// Trace finds the makers and readers of name in f. name may be written as vars.token or token (a variable),
// inputs.region, or nodes.login.body.token or login.body.token (what a node produced), with or without {{ }}.
// A name nothing makes or reads gives an empty trace: the caller decides whether that is an error.
func Trace(f model.Flow, name string) ValueTrace {
	flat := flattenNodes(f.Nodes, nil)
	ids := map[string]bool{}
	for _, n := range flat {
		ids[n.ID] = true
	}
	path := splitTraceName(name, ids)
	t := ValueTrace{Name: strings.Join(path, "."), Kind: traceKind(path), Producers: []string{}, Readers: []string{}}
	t.Producers, t.Generated = producersOf(t, path, flat)
	t.Readers = readersOf(f, flat, path)
	return t
}

// splitTraceName turns the forms a person writes into the path a template reference has.
func splitTraceName(name string, ids map[string]bool) []string {
	name = strings.TrimSpace(name)
	name = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(name, "{{"), "}}"))
	path := strings.Split(name, ".")
	switch {
	case path[0] == "vars" || path[0] == "inputs" || path[0] == "nodes":
		return path
	case len(path) >= 2 && ids[path[0]]:
		return append([]string{"nodes"}, path...)
	}
	return append([]string{"vars"}, path...)
}

func traceKind(path []string) string {
	switch path[0] {
	case "inputs":
		return TraceInput
	case "nodes":
		return TraceOutput
	}
	return TraceVariable
}

// producersOf lists the nodes that make the traced value.
func producersOf(t ValueTrace, path []string, flat []model.Node) (producers []string, generated bool) {
	producers = []string{}
	switch t.Kind {
	case TraceOutput:
		if len(path) >= 2 {
			producers = append(producers, path[1])
		}
	case TraceVariable:
		for _, n := range flat {
			if !slices.Contains(sites.OwnWrites(n), "vars."+path[1]) {
				continue
			}
			producers = append(producers, n.ID)
			generated = generated || sites.GeneratedVars(n)[path[1]]
		}
	}
	return producers, generated
}

// readersOf lists the nodes with a template that reads the traced path or something inside or around it.
func readersOf(f model.Flow, flat []model.Node, path []string) []string {
	out := []string{}
	for _, n := range flat {
		if slices.Contains(out, n.ID) || !readsPath(f, n, path) {
			continue
		}
		out = append(out, n.ID)
	}
	return out
}

func readsPath(f model.Flow, n model.Node, path []string) bool {
	for _, site := range sites.NodeTemplates(sites.Located[model.Node]{File: f.File, Value: n}) {
		refs, err := template.References(site.Src, site.Expression)
		if err != nil {
			continue // the validator reports a template that does not compile
		}
		for _, ref := range refs {
			if overlaps(ref.Path, path) {
				return true
			}
		}
	}
	return false
}

// overlaps is true when one path is the start of the other: reading nodes.login.body covers the token inside it,
// and reading the token is a read of what the body holds.
func overlaps(a, b []string) bool {
	n := min(len(a), len(b))
	return slices.Equal(a[:n], b[:n])
}
