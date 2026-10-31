// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/blueprint"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Values of requests.seed_blueprint that are not blueprint names.
const (
	seedDefault = "default"
	seedOff     = "off"
)

// SeedRequestHooks gives a new request its starting hooks. The blueprints
// named by requests.seed_blueprint (a comma-separated list; "default" is the
// built-in blueprint.Default, "off" seeds nothing, anything else is a
// blueprint's path or id) contribute their nodes as one inline hook step each,
// to the pre or post hooks by the blueprint's phase. Inline hooks travel with
// the request when it is moved or saved into another collection.
//
// A phase in which the request already has hooks is left alone. A blueprint
// that is not found, or has parameters (nothing could fill them), is an error
// and nothing is written.
func (w *Workspace) SeedRequestHooks(t RequestTarget) error {
	_, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	names := w.seedSetting(t.Collection)
	if len(names) == 1 && names[0] == seedOff {
		return nil // "off" turns seeding off only on its own; in a list it names nothing
	}
	hooks := r.Hooks
	var outcomes []model.Outcome
	for _, name := range names {
		if name == seedOff {
			continue
		}
		bps, err := w.seedBlueprints(name, t.Collection, kindOf(r))
		if err != nil {
			return err
		}
		for _, bp := range bps {
			if len(bp.Params) > 0 {
				return diagErr(diag.CodeBlueprintUse, map[string]string{"blueprint": name, "detail": "it declares parameters, which a request's hooks cannot be given"})
			}
			if outcomes == nil && len(bp.Outcomes) > 0 {
				outcomes = slices.Clone(bp.Outcomes) // the first blueprint that lists outcomes decides them
			}
			step := model.HookStep{Inline: slices.Clone(bp.Body.Nodes)}
			if bp.Phase == "pre" && len(r.Hooks.Pre) == 0 {
				hooks.Pre = append(hooks.Pre, step)
			} else if bp.Phase != "pre" && len(r.Hooks.Post) == 0 {
				hooks.Post = append(hooks.Post, step)
			}
		}
	}
	seeded := seedOutcomes(&r, outcomes)
	if len(hooks.Pre) == len(r.Hooks.Pre) && len(hooks.Post) == len(r.Hooks.Post) && !seeded {
		return nil
	}
	r.Hooks = hooks
	return w.SaveRequest(t, r, SaveOptions{})
}

// CreateHookBlueprint saves nodes, the hooks of one phase ("pre" or "post")
// that a request was given, as blueprints/<path>.blueprint.yaml in
// collection, so that other requests can start with them (see
// SeedRequestHooks). Path rules are as for NewRequest (FLW-WRITE-002).
func (w *Workspace) CreateHookBlueprint(collection model.ID, path, name, phase string, nodes []model.Node) (BlueprintTarget, error) {
	if phase != "pre" && phase != "post" {
		return BlueprintTarget{}, diagErr(diag.CodeBlueprintUse, map[string]string{"blueprint": name, "detail": `the phase is "pre" or "post"`})
	}
	if len(nodes) == 0 {
		return BlueprintTarget{}, diagErr(diag.CodeBlueprintUse, map[string]string{"blueprint": name, "detail": "there are no nodes to save"})
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return BlueprintTarget{}, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	file, err := newEntityFile(c, "blueprints", path, blueprintSuffix)
	if err != nil {
		return BlueprintTarget{}, err
	}
	bp := model.Blueprint{Meta: model.Meta{ID: w.newID(), Name: name}, Path: path, Phase: phase, File: file,
		Body: model.Flow{Nodes: slices.Clone(nodes)}}
	data, err := w.store.EncodeBlueprint(bp)
	if err != nil {
		return BlueprintTarget{}, fmt.Errorf("encoding %s: %w", file, err)
	}
	if err := w.writeAndReload(collection, file, data, SaveOptions{}); err != nil {
		return BlueprintTarget{}, err
	}
	return BlueprintTarget{Collection: collection, Blueprint: bp.ID}, nil
}

// seedSetting is the names in requests.seed_blueprint; empty means default.
func (w *Workspace) seedSetting(collection model.ID) []string {
	v, _ := w.Settings(Scope{Collection: collection}).Get("requests.seed_blueprint")
	raw, _ := v.Value.(string)
	var names []string
	for _, n := range strings.Split(raw, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return []string{seedDefault}
	}
	return names
}

// seedBlueprint is the built-in default or the stored blueprint named which,
// looked for in the request's own collection first so that a copy of it in
// another mounted collection does not make the name ambiguous.
func (w *Workspace) seedBlueprints(which string, collection model.ID, kind requestKind) ([]model.Blueprint, error) {
	switch {
	case which == seedDefault && kind == kindGRPC:
		// A gRPC call has a status code, not an HTTP status.
		return []model.Blueprint{blueprint.DefaultGRPCPre(), blueprint.DefaultGRPC()}, nil
	case which == seedDefault && kind == kindDataset:
		// A query has an outcome and rows, not an HTTP status.
		return []model.Blueprint{blueprint.DefaultAsyncPre(), blueprint.DefaultDataset()}, nil
	case which == seedDefault && kind == kindAsync:
		// The built-in default checks the HTTP status, which a message request lacks.
		return []model.Blueprint{blueprint.DefaultAsyncPre(), blueprint.DefaultAsync()}, nil
	case which == seedDefault:
		return []model.Blueprint{blueprint.Default()}, nil
	}
	bp, err := w.seedBlueprint(which, collection)
	return []model.Blueprint{bp}, err
}

// requestKind is what a request's opening version is: HTTP, async, gRPC or a dataset query.
type requestKind int

const (
	kindHTTP requestKind = iota
	kindAsync
	kindGRPC
	kindDataset
)

// kindOf is the kind of the version a request opens on.
func kindOf(r model.Request) requestKind {
	if len(r.Versions) == 0 {
		return kindHTTP
	}
	opening := r.Versions[0]
	for _, v := range r.Versions {
		if r.Default == "" || v.Label == r.Default {
			opening = v
			break
		}
	}
	switch opening.Family() {
	case model.FamilyDataset:
		return kindDataset
	case model.FamilyRPC:
		return kindGRPC
	case model.FamilyMessaging:
		return kindAsync
	}
	return kindHTTP
}

func (w *Workspace) seedBlueprint(which string, collection model.ID) (model.Blueprint, error) {
	target, err := w.FindBlueprint(which, string(collection))
	if err != nil {
		target, err = w.FindBlueprint(which, "")
	}
	if err != nil {
		return model.Blueprint{}, err
	}
	return w.Blueprint(target)
}

// seedOutcomes gives the opening version of r the outcomes, unless it writes its own, and says whether it did.
func seedOutcomes(r *model.Request, outcomes []model.Outcome) bool {
	if len(outcomes) == 0 || len(r.Versions) == 0 {
		return false
	}
	i := 0
	for k, v := range r.Versions {
		if r.Default == "" || v.Label == r.Default {
			i = k
			break
		}
	}
	if len(r.Versions[i].Outcomes) > 0 {
		return false
	}
	r.Versions = slices.Clone(r.Versions)
	r.Versions[i].Outcomes = slices.Clone(outcomes)
	return true
}
