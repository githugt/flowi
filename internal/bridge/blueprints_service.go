// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/blueprint"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// BlueprintParam is a value a blueprint asks for.
type BlueprintParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
}

// BlueprintView is a reusable blueprint of a mounted collection.
type BlueprintView struct {
	Key         string           `json:"key"` // collection id and blueprint id
	Collection  string           `json:"collection"`
	CollectionN string           `json:"collectionName"`
	Name        string           `json:"name"`
	Path        string           `json:"path"`
	Description string           `json:"description"`
	Kind        string           `json:"kind"` // flow, post-hooks or pre-hooks
	Params      []BlueprintParam `json:"params"`
	Nodes       int              `json:"nodes"`
	// BuiltIn marks a blueprint Flowi ships, which new requests start with
	// unless requests.seed_blueprint names others. It has no file and cannot
	// be deleted.
	BuiltIn bool `json:"builtIn,omitempty"`
}

// BlueprintsView is the blueprint library.
type BlueprintsView struct {
	Blueprints  []BlueprintView `json:"blueprints"`
	Collections []Choice        `json:"collections"` // where a flow made from one can go
}

// BlueprintsService is the library of blueprints.
type BlueprintsService struct{ app *App }

// newBlueprintsService builds the service for the blueprint library.
func newBlueprintsService(a *App) *BlueprintsService {
	s := &BlueprintsService{app: a}
	a.workspace.OnChange(func() { a.emit("blueprints", s.view()) })
	return s
}

// State is the blueprints of every mounted collection; they follow the
// workspace, and arrive again as the "blueprints" event when it reloads.
func (s *BlueprintsService) State() BlueprintsView {
	var v BlueprintsView
	s.app.actor.Do(func() { v = s.view() })
	return v
}

// WhereView is where a flow made from a tree row would go.
type WhereView struct {
	Collection string `json:"collection"` // collection id, "" when the row belongs to none
	Folder     string `json:"folder"`     // the folder clicked ("auth"), "" at the top
}

// Where says which collection and folder a flow made from the tree row `node` goes to.
func (s *BlueprintsService) Where(node string) WhereView {
	var w WhereView
	s.app.actor.Do(func() {
		n := s.app.workspace.Node(node)
		prefix, _ := folderPrefix(n)
		w = WhereView{Collection: string(n.Collection), Folder: strings.TrimSuffix(prefix, "/")}
	})
	return w
}

// CreateFlow makes a flow at path in collection from a blueprint and asks
// the window to open it ("open" event).
func (s *BlueprintsService) CreateFlow(key, collection, path string, values map[string]string) {
	s.app.actor.Do(func() {
		bp, ok := s.find(key)
		if !ok {
			s.app.emit("notice", Notice{Level: "error", Text: "that blueprint is gone"})
			return
		}
		s.app.workspace.CreateFlowFromBlueprint(bp, model.ID(collection), path, values,
			func(node string) { s.app.emit("open", node) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// Delete removes a blueprint.
func (s *BlueprintsService) Delete(key string) {
	s.app.actor.Do(func() {
		bp, ok := s.find(key)
		if !ok {
			return
		}
		s.app.workspace.DeleteBlueprint(bp, s.app.workspace.Load,
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// UseAsDefault makes the hooks of a blueprint what new requests start with.
func (s *BlueprintsService) UseAsDefault(key string) {
	s.app.actor.Do(func() {
		bp, ok := s.find(key)
		if !ok {
			return
		}
		s.app.workspace.UseBlueprintAsDefault(bp, func(note string) {
			text := "New requests now start with these hooks."
			if note != "" {
				text += " " + note
			}
			s.app.emit("notice", Notice{Level: "info", Text: text})
		}, func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

func (s *BlueprintsService) find(key string) (vm.BlueprintChoice, bool) {
	for _, bp := range s.app.workspace.Blueprints() {
		if blueprintKey(bp) == key {
			return bp, true
		}
	}
	return vm.BlueprintChoice{}, false
}

func blueprintKey(bp vm.BlueprintChoice) string { return fmt.Sprintf("%s/%s", bp.Collection, bp.ID) }

func (s *BlueprintsService) view() BlueprintsView {
	v := BlueprintsView{Blueprints: []BlueprintView{}, Collections: []Choice{}}
	for _, c := range s.app.workspace.Collections() {
		v.Collections = append(v.Collections, Choice{ID: string(c.Manifest.ID), Label: c.Manifest.Name, Path: c.Manifest.Name})
	}
	v.Blueprints = append(v.Blueprints, builtInBlueprints()...)
	for _, bp := range s.app.workspace.Blueprints() {
		kind := "flow"
		switch bp.Kind() {
		case vm.BlueprintPostHooks:
			kind = "post-hooks"
		case vm.BlueprintPreHooks:
			kind = "pre-hooks"
		}
		b := BlueprintView{
			Key: blueprintKey(bp), Collection: string(bp.Collection), CollectionN: bp.CollectionName, Name: bp.Name, Path: bp.Path,
			Description: bp.Description, Kind: kind, Params: []BlueprintParam{}, Nodes: len(bp.Nodes),
		}
		for _, p := range bp.Params {
			b.Params = append(b.Params, BlueprintParam{Name: p.Name, Description: p.Description, Type: string(p.Type), Default: p.Default})
		}
		v.Blueprints = append(v.Blueprints, b)
	}
	return v
}

// builtInBlueprints are the hooks new requests start with by default, one
// pair per kind of request (flowi-lib's requests.seed_blueprint "default").
func builtInBlueprints() []BlueprintView {
	shipped := []struct {
		id string
		bp model.Blueprint
	}{
		{id: "http-post", bp: blueprint.Default()},
		{id: "async-pre", bp: blueprint.DefaultAsyncPre()},
		{id: "async-post", bp: blueprint.DefaultAsync()},
		{id: "grpc-pre", bp: blueprint.DefaultGRPCPre()},
		{id: "grpc-post", bp: blueprint.DefaultGRPC()},
	}
	out := make([]BlueprintView, 0, len(shipped))
	for _, b := range shipped {
		kind := "post-hooks"
		if b.bp.Phase == "pre" {
			kind = "pre-hooks"
		}
		out = append(out, BlueprintView{
			Key: "built-in/" + b.id, CollectionN: "Built in", Name: b.bp.Name, Description: b.bp.Description,
			Kind: kind, Params: []BlueprintParam{}, Nodes: len(b.bp.Body.Nodes), BuiltIn: true,
		})
	}
	return out
}
