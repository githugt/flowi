// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
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
	if err := w.sendable(c, o.Env); err != nil {
		return nil, err
	}
	if err := checkInputs(f, o.Inputs); err != nil {
		return nil, err
	}
	if err := w.unlockFor(c, o.Env); err != nil {
		return nil, err
	}
	in := engine.RunInput{Collection: c, Flow: f, Env: o.Env, PreferVersion: o.PreferVersion, Inputs: o.Inputs, Seed: o.Seed, Grace: o.Grace, Only: o.Only, Prior: o.Prior}
	return engine.StartRun(ctx, w.engineDeps(), in), nil
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
