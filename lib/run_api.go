// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/flowioverlay"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

// FlowTarget identifies a flow of a mounted collection.
type FlowTarget struct {
	Collection model.ID
	Flow       model.ID
}

// RunOptions configure RunFlow.
type RunOptions struct {
	Env           string            // environment name; "" uses defaults.environment
	PreferVersion string            // beats the flow's prefer_version (design §4.4)
	Inputs        map[string]string // values for the flow's declared inputs, as templates
	Seed          uint64            // 0 picks a random seed (reported in the result)
	Grace         time.Duration     // 0 uses the run.grace setting
	// Only runs just these top-level nodes, with Prior as the outputs of
	// the others (see engine.RunInput). nil runs the whole flow.
	Only  []string
	Prior map[string]map[string]any
	// PriorVars are the flow's variables as an earlier run ended (see
	// engine.RunInput.PriorVars).
	PriorVars map[string]any
	// Skip lists node ids that do not run this time (see engine.RunInput.Skip).
	Skip []string
	// Overlay is a flowi overlay (or a stack, "vip+slow") laid over every request step that names none of its
	// own. An unknown name is FLW-OVL-002 and nothing runs.
	Overlay string
	// SkipSchemas stops the run from checking and learning response schemas, so it leaves the stored ones exactly
	// as they are (a drift check compares against them).
	SkipSchemas bool
	// Transport, when set, sends every HTTP call instead of the network: a run played back from a recording
	// (see OpenBundle).
	Transport http.RoundTripper
	// Secrets gives values for the environment's secrets by name, in place of the vault and of
	// FLOWI_SECRET_* variables. A replay passes the mask for each, so no vault is opened.
	Secrets map[string]string
}

// FindFlow resolves a flow reference as users type it: an id, or a path
// ("user-creation", optionally with flows/ and .arazzo.yaml). collection
// narrows the search by name or id. Problems are *diag.ListError:
// FLW-FLOW-002 (no match), FLW-FLOW-003 (several collections match),
// FLW-WS-005 (unknown collection).
func (w *Workspace) FindFlow(ref, collection string) (FlowTarget, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.findFlow(ref, collection)
}

// findFlow is FindFlow for callers that hold mu.
func (w *Workspace) findFlow(ref, collection string) (FlowTarget, error) {
	candidates, err := w.collectionsNamed(collection)
	if err != nil {
		return FlowTarget{}, err
	}
	path := strings.TrimSuffix(strings.TrimPrefix(ref, "flows/"), ".arazzo.yaml")
	var found []FlowTarget
	for _, c := range candidates {
		if i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return string(f.ID) == ref || f.Path == path }); i >= 0 {
			found = append(found, FlowTarget{Collection: c.Manifest.ID, Flow: c.Flows[i].ID})
		}
	}
	switch len(found) {
	case 0:
		return FlowTarget{}, diagErr(diag.CodeFlowNotFound, map[string]string{"ref": ref})
	case 1:
		return found[0], nil
	}
	return FlowTarget{}, diagErr(diag.CodeFlowAmbiguous, map[string]string{"ref": ref, "count": strconv.Itoa(len(found))})
}

// FindFlowsTagged lists the flows that carry tag, from every collection or only the one collection names, in
// collection and file order. A tag nothing carries is FLW-FLOW-004, so a misspelt tag in CI fails instead of
// passing with nothing run.
func (w *Workspace) FindFlowsTagged(tag, collection string) ([]FlowTarget, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	candidates, err := w.collectionsNamed(collection)
	if err != nil {
		return nil, err
	}
	var found []FlowTarget
	for _, c := range candidates {
		for _, f := range c.FlowsTagged(tag) {
			found = append(found, FlowTarget{Collection: c.Manifest.ID, Flow: f.ID})
		}
	}
	if len(found) == 0 {
		return nil, diagErr(diag.CodeFlowTagEmpty, map[string]string{"tag": tag})
	}
	return found, nil
}

// RunFlow starts running a flow. The flow's collection (and the
// collections it depends on) must validate without errors, the
// environment must exist and the inputs must match the flow's declared
// inputs (FLW-RUN-009, FLW-RUN-010); otherwise a *diag.ListError is
// returned and nothing runs. The run's report is saved in the workspace
// data dir. The run goes on until its Events are drained and Wait returns.
func (w *Workspace) RunFlow(ctx context.Context, t FlowTarget, o RunOptions) (*engine.Run, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	o = w.asReplay(t.Collection, o)
	o = w.withDocumentEnv(t.Collection, o)
	if _, missing := flowioverlay.Resolve(c.Overlays, o.Overlay, ""); o.Overlay != "" && missing != "" {
		return nil, diagErr(diag.CodeOverlayNotFound, map[string]string{"name": missing})
	}
	if err := w.sendable(c, o.Env); err != nil {
		return nil, err
	}
	if err := checkInputs(f, o.Inputs); err != nil {
		return nil, err
	}
	environ := w.runEnviron(c, o)
	if err := w.unlockWith(c, o.Env, environ); err != nil {
		return nil, err
	}
	in := engine.RunInput{Collection: c, Flow: f, Env: o.Env, PreferVersion: o.PreferVersion, Inputs: o.Inputs, Seed: o.Seed, Grace: o.Grace, Only: o.Only, Prior: o.Prior, PriorVars: o.PriorVars, Skip: o.Skip, Overlay: o.Overlay}
	deps := w.engineDeps()
	deps.Transport, deps.Environ = o.Transport, environ
	if o.SkipSchemas {
		deps.Schemas = nil
	}
	if len(o.Secrets) > 0 {
		// A vault of its own, without a file: the secrets the options give win, and nothing else is read.
		deps.Vault = vault.New(vault.Options{WorkspaceID: w.Model().ID, Environ: environ})
	}
	return engine.StartRun(ctx, deps, in), nil
}

// runEnviron is the process environment a run sees: the workspace's, with a FLOWI_SECRET_* entry for each
// secret the options give.
func (w *Workspace) runEnviron(c model.Collection, o RunOptions) []string {
	environ := slices.Clone(w.opts.Environ)
	env := o.Env
	if env == "" {
		v, _ := w.Settings(Scope{Collection: c.Manifest.ID}).Get("defaults.environment")
		env, _ = v.Value.(string)
	}
	for _, name := range slices.Sorted(maps.Keys(o.Secrets)) {
		environ = append(environ, vault.OverrideName(env, name)+"="+o.Secrets[name])
	}
	return environ
}

func (w *Workspace) flowFor(t FlowTarget) (model.Collection, model.Flow, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(t.Collection)
	if !ok {
		return model.Collection{}, model.Flow{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(t.Collection)})
	}
	i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.ID == t.Flow })
	if i < 0 {
		return model.Collection{}, model.Flow{}, diagErr(diag.CodeFlowNotFound, map[string]string{"ref": string(t.Flow)})
	}
	return c, c.Flows[i], nil
}

// checkInputs applies the engine's input rules before anything runs, so
// a wrong --input is a usage error rather than a failed run.
func checkInputs(f model.Flow, given map[string]string) error {
	var problems diag.List
	for _, name := range slices.Sorted(maps.Keys(given)) {
		if !slices.ContainsFunc(f.Inputs, func(in model.Input) bool { return in.Name == name }) {
			problems = append(problems, diag.New(diag.CodeRunInputUnknown, diag.Location{}, map[string]string{"name": name, "flow": f.Path}))
		}
	}
	for _, in := range f.Inputs {
		if _, ok := given[in.Name]; !ok && in.Default == "" {
			problems = append(problems, diag.New(diag.CodeRunInputMissing, diag.Location{}, map[string]string{"name": in.Name, "flow": f.Path}))
		}
	}
	return problems.Err()
}

// CheckSkips says what skipping the steps in skip would break: one
// FLW-RUN-022 for each step that reads something a skipped step would have
// made. It is an error when the read has no default and a warning when it has.
// Holding the skipped steps' last values (RunOptions.Prior with Skip) makes the
// reads work again; this check does not know whether the caller will.
func (w *Workspace) CheckSkips(t FlowTarget, skip []string) (diag.List, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	var out diag.List
	for _, r := range analysis.SkipReaders(f, skip) {
		d := diag.New(diag.CodeRunSkipRead, diag.Location{File: f.File}, map[string]string{"reader": r.Reader, "reads": r.Reads, "skipped": r.Skipped})
		if !r.Required {
			d.Severity = diag.SeverityWarning
		}
		out = append(out, d)
	}
	return out, nil
}

// withDocumentEnv gives a run in a collection opened from an Arazzo document that chose no environment the
// document's own, so its flows reach the server the document names.
func (w *Workspace) withDocumentEnv(collection model.ID, o RunOptions) RunOptions {
	w.mu.RLock()
	env := w.documentEnvs[collection]
	w.mu.RUnlock()
	if o.Env == "" && env != "" {
		o.Env = env
	}
	return o
}
