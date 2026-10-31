// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// BlueprintChoice is a blueprint offered in the "New flow from blueprint"
// dialog, with the parameters the dialog asks for.
type BlueprintChoice struct {
	Collection     model.ID
	CollectionName string
	ID             model.ID
	Path, Name     string
	Description    string
	Params         []model.Param
	Phase          string       // "pre" or "post": which hooks the nodes can be (a hook blueprint)
	Nodes          []model.Node // what using it as hooks inserts
}

// Label is how the dialog's list shows the choice.
func (b BlueprintChoice) Label() string { return b.Name + " (" + b.CollectionName + ")" }

// Target is the library's handle for the blueprint.
func (b BlueprintChoice) Target() flowi.BlueprintTarget {
	return flowi.BlueprintTarget{Collection: b.Collection, Blueprint: b.ID}
}

// Blueprints lists the blueprints of the mounted collections, as of the
// last load.
func (ws *Workspace) Blueprints() []BlueprintChoice {
	var out []BlueprintChoice
	for _, c := range ws.Collections() {
		for _, b := range c.Blueprints {
			name := b.Name
			if name == "" {
				name = b.Path
			}
			out = append(out, BlueprintChoice{
				Collection: c.Manifest.ID, CollectionName: c.Manifest.Name, ID: b.ID, Path: b.Path, Name: name,
				Description: b.Description, Params: b.Params, Phase: phaseOf(b), Nodes: b.Body.Nodes,
			})
		}
	}
	return out
}

// CreateFlowFromBlueprint creates a flow at path in collection ("" means
// Scratch) from bp with the given parameter values, then opens it.
func (ws *Workspace) CreateFlowFromBlueprint(bp BlueprintChoice, collection model.ID, path string, values map[string]string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewFlowFromBlueprint(bp.Target(), id, path, values)
		return "f:" + string(t.Collection) + ":" + string(t.Flow), err
	}, open, fail)
}

// SaveFlowAsBlueprint saves a flow as a blueprint of its collection, then
// refreshes the lists.
func (ws *Workspace) SaveFlowAsBlueprint(flow flowi.FlowTarget, path, name string, done func(), fail func(error)) {
	ws.create(func() (string, error) {
		_, err := ws.b.CreateBlueprint(flow, path, name)
		return "", err
	}, func(string) {
		if done != nil {
			done()
		}
	}, fail)
}

// phaseOf is the hooks a blueprint's nodes can be: post unless it says pre.
func phaseOf(b model.Blueprint) string {
	if b.Phase == "pre" {
		return "pre"
	}
	return "post"
}

// BlueprintKind is what a blueprint makes.
type BlueprintKind int

// Kinds.
const (
	BlueprintFlow      BlueprintKind = iota // a flow, from its parameters
	BlueprintPostHooks                      // the post-hooks of a request
	BlueprintPreHooks                       // the pre-hooks of a request
)

// Kind says what the blueprint makes: hooks when it has no parameters (a
// request's hooks cannot be given any) and has nodes, else a flow.
func (b BlueprintChoice) Kind() BlueprintKind {
	switch {
	case len(b.Params) > 0 || len(b.Nodes) == 0:
		return BlueprintFlow
	case b.Phase == "pre":
		return BlueprintPreHooks
	}
	return BlueprintPostHooks
}

// DeleteBlueprint deletes bp, then refreshes the lists. When new requests
// are seeded from it, it also leaves the requests.seed_blueprint setting (to
// "default" when it was the only one): a name that no longer resolves would
// make every new request fail to seed. A name another collection's blueprint
// still answers to stays, as that one then seeds.
func (ws *Workspace) DeleteBlueprint(bp BlueprintChoice, done func(), fail func(error)) {
	sameName := false
	for _, other := range ws.Blueprints() {
		sameName = sameName || (other.Collection != bp.Collection && other.Path == bp.Path)
	}
	ws.create(func() (string, error) {
		if err := ws.b.DeleteBlueprint(bp.Target()); err != nil {
			return "", err
		}
		user, _ := seedSettingOf(ws.b.Settings(bp.Collection))
		if after, changed := seedWithout(user, bp, sameName); changed {
			return "", ws.b.SetUserSetting(seedKey, after)
		}
		return "", nil
	}, func(string) {
		if done != nil {
			done()
		}
	}, fail)
}

// seedWithout is the setting value without the entries that name bp, and
// whether any did. keepPath keeps the entries that name it by path, because
// a blueprint of another collection has that path too.
func seedWithout(current string, bp BlueprintChoice, keepPath bool) (string, bool) {
	var out []string
	changed := false
	for _, entry := range strings.Split(current, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		name := normaliseBlueprintName(entry)
		if name == string(bp.ID) || (name == bp.Path && !keepPath) {
			changed = true
			continue
		}
		out = append(out, entry)
	}
	switch {
	case !changed:
		return current, false
	case len(out) == 0:
		return "default", true // it was the only one: back to the built-in hooks
	}
	return strings.Join(out, ", "), true
}

// UseBlueprintAsDefault makes the hooks of bp what new requests start with
// for its phase, as saving hooks "as the default" does. done gets a note
// when another settings layer overrides the choice. A flow blueprint
// cannot be used so.
func (ws *Workspace) UseBlueprintAsDefault(bp BlueprintChoice, done func(note string), fail func(error)) {
	if bp.Kind() == BlueprintFlow {
		if fail != nil {
			fail(errors.New("only a blueprint of hooks (no parameters) can start new requests"))
		}
		return
	}
	known := ws.Blueprints()
	var note string // written by the worker before create's callbacks run
	ws.create(func() (string, error) {
		user, overriding := seedSettingOf(ws.b.Settings(bp.Collection))
		note = overriding
		return "", ws.b.SetUserSetting(seedKey, SeedAfterAdding(user, bp.Path, phaseName(bp.Phase), known))
	}, func(string) {
		if done != nil {
			done(note)
		}
	}, fail)
}
