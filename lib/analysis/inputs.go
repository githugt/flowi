// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"

	"github.com/githugt/flowi-lib/internal/sites"
	"github.com/githugt/flowi-lib/model"
)

// need is an input a request version's templates read.
type need struct {
	name string
	expr string // as written: with.tenant, vars.client_id, email
	root string // "with", "vars", or "" for a bare name
}

// scopeRoots are the names a request template can start with that are not
// bare inputs.
func scopeRoots() []string {
	return []string{"env", "secret", "with", "vars", "var", "inputs", "nodes", "request", "response", "status", "run", "item", "index"}
}

// needs lists what version v's templates read from with, vars or a bare
// name (which resolves through with, vars, inputs, then env).
func needs(file string, v model.RequestVersion) []need {
	var out []need
	add := func(n need) {
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	for _, ref := range parseRefs(sites.VersionTemplates(file, v)) {
		path := ref.path
		switch {
		case (path[0] == "with" || path[0] == "vars") && len(path) > 1:
			add(need{name: path[1], expr: path[0] + "." + path[1], root: path[0]})
		case !slices.Contains(scopeRoots(), path[0]):
			add(need{name: path[0], expr: path[0]})
		}
	}
	return out
}

// needsOf is needs, worked out once per request version.
func (ix *index) needsOf(c model.Collection, r model.Request, v model.RequestVersion) []need {
	key := versionKey{c.Manifest.ID, r.ID, v.Label}
	if n, ok := ix.needsMemo[key]; ok {
		return n
	}
	n := needs(r.File, v)
	ix.needsMemo[key] = n
	return n
}

// filled is what can supply a request node's inputs. A request runs with
// its own variables and no flow inputs (engine/execute.go), so the flow's
// vars and inputs are not among them.
type filled struct {
	with, vars, env map[string]bool // vars: written by pre-hooks; env: non-secret keys
}

func (f filled) has(n need) bool {
	switch n.root {
	case "with":
		return f.with[n.name]
	case "vars":
		return f.vars[n.name]
	}
	return f.with[n.name] || f.vars[n.name] || f.env[n.name]
}

// missingInputs are the needs of version v that node n does not fill: not
// in its with, not a variable r's pre-hooks write, not a non-secret key of
// any of r's collection's environments.
func missingInputs(ix *index, n sites.Located[model.Node], c model.Collection, r model.Request, v model.RequestVersion) []Read {
	f := fillersOf(ix, n.Value, c, r, v)
	var out []Read
	for _, nd := range ix.needsOf(c, r, v) {
		if f.has(nd) {
			continue
		}
		out = append(out, Read{Kind: ReadInput, Expr: nd.expr, Target: nd.name, Reader: n.Value.ID, File: n.File, Path: n.Path,
			Status: Broken, Reason: "nothing here fills it"})
	}
	return out
}

func fillersOf(ix *index, n model.Node, c model.Collection, r model.Request, v model.RequestVersion) filled {
	f := filled{with: map[string]bool{}, vars: ix.preHookVars(c, r, v), env: map[string]bool{}}
	for _, kv := range n.Request.With {
		f.with[kv.Key] = true
	}
	for _, env := range c.Environments {
		for _, ev := range env.Vars {
			if !ev.Secret { // secret values are read as secret.<name>, never bare
				f.env[ev.Name] = true
			}
		}
	}
	return f
}
