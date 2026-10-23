// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"maps"
	"strings"
	"sync"

	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/template"
)

// flowScope is one flow being run: a top-level run, a subflow, a hook flow
// or an inline hook step. Everything in it is fixed when it starts.
type flowScope struct {
	collection    model.Collection
	flow          *model.Flow // nil for inline hook steps
	env           environment
	inputs        map[string]any
	prefer        string     // the flow's prefer_version
	onBranchError string     // the flow's execution.on_branch_error ("" → fail_fast)
	hook          *execution // the request whose hooks run here; nil otherwise
	phase         nodes.Phase
	iteration     int // flow-level run: iteration (1-based); 0 without run:
}

// frame is where a node list runs: its scope for vars, nodes.<id>, item
// and cookies, and where its node instances sit in the run's tree.
// Containers make child frames per lane (design §7.3).
type frame struct {
	fs      *flowScope
	vars    *nodes.Vars
	outputs *outputs
	item    any
	index   int
	hasItem bool
	jar     *httpx.Jar
	noJar   bool // http.cookies: off — this lane sends and keeps no cookies
	// prefix is prepended to node ids to make instance paths; parentPath
	// and lane say where the frame sits for NodeStarted events.
	prefix, parentPath, lane string
	depth                    int
}

// newRootFrame is the top frame of a flow scope.
func newRootFrame(fs *flowScope, jar *httpx.Jar) *frame {
	return &frame{fs: fs, vars: nodes.NewVars(nil), outputs: newOutputs(nil), jar: jar}
}

// child is the frame for one lane of the container at containerPath.
// Iteration labels ("[2]") attach to the container's path (users[2]/…);
// named lanes become a path segment (par/b1/…).
func (f *frame) child(l nodes.Lane, jar *httpx.Jar, containerPath string) *frame {
	c := *f
	c.parentPath, c.lane = containerPath, l.Label
	if strings.HasPrefix(l.Label, "[") {
		c.prefix = containerPath + l.Label + "/"
	} else {
		c.prefix = containerPath + "/" + l.Label + "/"
	}
	if l.OwnVars {
		c.vars = nodes.NewVars(f.vars)
	}
	if l.OwnNodes {
		c.outputs = newOutputs(f.outputs)
	}
	if l.HasItem {
		c.item, c.index, c.hasItem = l.Item, l.Index, true
	}
	if jar != nil {
		c.jar, c.noJar = jar, false
	}
	return &c
}

// cookieJar is the jar requests in this frame use; nil means none.
func (f *frame) cookieJar() *httpx.Jar {
	if f.noJar {
		return nil
	}
	return f.jar
}

// path is the instance path of node id in this frame.
func (f *frame) path(id string) string { return f.prefix + id }

// scope is what templates in this frame can read. Inside hooks it is the
// request's scope (env, secret, with, vars, request, response) with this
// frame's nodes.
func (f *frame) scope(r *runner) template.Scope {
	if f.fs.hook != nil {
		return f.hookScope(r)
	}
	fs := f.fs
	values := map[string]any{
		"env": fs.env.values, "secret": r.secrets.namespace(fs.collection.Manifest.ID, fs.env),
		"inputs": fs.inputs, "vars": f.vars.Snapshot(), "nodes": f.outputs.snapshot(),
		"run": map[string]any{"id": r.runID, "iteration": fs.iteration},
	}
	if fs.inputs == nil {
		values["inputs"] = map[string]any{}
	}
	if f.hasItem {
		values["item"], values["index"] = f.item, f.index
	}
	return template.Scope{Values: values, Bare: []string{"vars", "inputs", "env"}, Gen: r.gen}
}

// outputs holds nodes.<id> for one frame layer: written once per id,
// read merged with the parent layers. mu guards byID.
type outputs struct {
	mu     sync.RWMutex
	byID   map[string]any
	parent *outputs
}

func newOutputs(parent *outputs) *outputs {
	return &outputs{byID: map[string]any{}, parent: parent}
}

// put records a node's outputs. Ids are unique per flow (validated) and
// repeated bodies get their own layer, so a second write is a bug in the
// engine, not bad input.
func (o *outputs) put(id string, v map[string]any) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, dup := o.byID[id]; dup {
		panic("engine: outputs of node " + id + " written twice")
	}
	o.byID[id] = v
}

// snapshot merges every layer, nearer layers winning.
func (o *outputs) snapshot() map[string]any {
	out := map[string]any{}
	if o.parent != nil {
		out = o.parent.snapshot()
	}
	o.mu.RLock()
	defer o.mu.RUnlock()
	maps.Copy(out, o.byID)
	return out
}

// own is a copy of this layer alone (a lane's nodes).
func (o *outputs) own() map[string]any {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return maps.Clone(o.byID)
}

func (f *frame) hookScope(r *runner) template.Scope {
	sc := f.fs.hook.scope(false)
	values := maps.Clone(sc.Values)
	values["nodes"] = f.outputs.snapshot()
	values["run"] = map[string]any{"id": r.runID, "iteration": f.fs.iteration}
	if f.hasItem {
		values["item"], values["index"] = f.item, f.index
	}
	sc.Values = values
	return sc
}
