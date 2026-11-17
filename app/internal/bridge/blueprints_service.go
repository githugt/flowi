// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// BlueprintParam is a value a blueprint asks for.
type BlueprintParam struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default"`
}

// BlueprintStep is one step of a blueprint, as a line the library can list.
type BlueprintStep struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Detail string `json:"detail"` // the step's first setting: what it logs, checks or calls
}

// blueprintSteps lists the top-level steps of a blueprint.
func blueprintSteps(nodes []model.Node) []BlueprintStep {
	steps := make([]BlueprintStep, 0, len(nodes))
	for _, n := range nodes {
		step := BlueprintStep{Name: n.Name, Type: string(n.Type)}
		if step.Name == "" {
			step.Name = n.ID
		}
		if len(n.Params) > 0 {
			step.Detail = fmt.Sprint(n.Params[0].Value)
		}
		steps = append(steps, step)
	}
	return steps
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
	Steps       []BlueprintStep  `json:"steps"`
	// Group is where the library lists it: the kind of request whose default it is ("http", "async", "grpc"
	// or "dataset"), "hooks" for other reusable hooks, or "flow" for a flow template.
	Group string `json:"group"`
	// ReplacesDefault marks a collection's own version of a built-in default: deleting it brings the built-in back.
	ReplacesDefault bool `json:"replacesDefault,omitempty"`
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

// Source is the text of a blueprint, for the editor.
func (s *BlueprintsService) Source(key string) (string, error) {
	bp, err := s.editable(key)
	if err != nil {
		return "", err
	}
	text, err := s.app.backend.BlueprintSource(bp.Target())
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	return text, nil
}

// SaveSource writes the edited text of a blueprint. With problems in the text
// nothing is written and they are returned, one per line; "" means saved.
func (s *BlueprintsService) SaveSource(key, text string) (string, error) {
	bp, err := s.editable(key)
	if err != nil {
		return "", err
	}
	diags, err := s.app.backend.SaveBlueprintSource(bp.Target(), text)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	if diags.HasErrors() {
		return vm.ErrorText(diags.Err()), nil
	}
	s.app.actor.Do(s.app.workspace.Load)
	return "", nil
}

// New makes a hook blueprint named name in collection, for the phase "pre" or
// "post", with one step to start from, and returns its key for the editor.
func (s *BlueprintsService) New(collection, name, phase string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", errors.New("a blueprint needs a name")
	}
	start := []model.Node{{ID: "log", Type: "log", Params: []model.Field{{Key: "value", Value: "Hello from " + name}}}}
	path := strings.ReplaceAll(strings.ToLower(name), " ", "-")
	t, err := s.app.backend.CreateHookBlueprint(model.ID(collection), path, name, phase, start)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	s.app.actor.Do(s.app.workspace.Load)
	return fmt.Sprintf("%s/%s", t.Collection, t.Blueprint), nil
}

// Customize gives collection its own, editable copy of the built-in default
// `key` ("built-in/http-post") and returns the copy's key, for the editor.
// When the collection already has one, that one's key is returned.
func (s *BlueprintsService) Customize(key, collection string) (string, error) {
	group, phase, ok := strings.Cut(strings.TrimPrefix(key, "built-in/"), "-")
	if !ok {
		return "", errors.New("that is not a built-in blueprint")
	}
	var existing string
	s.app.actor.Do(func() {
		for _, bp := range s.app.workspace.Blueprints() {
			if string(bp.Collection) == collection && bp.Path == flowi.DefaultBlueprintPath(group, phase) {
				existing = blueprintKey(bp)
			}
		}
	})
	if existing != "" {
		return existing, nil
	}
	t, err := s.app.backend.CustomizeDefault(model.ID(collection), group, phase)
	if err != nil {
		return "", errors.New(vm.ErrorText(err))
	}
	s.app.actor.Do(s.app.workspace.Load)
	return fmt.Sprintf("%s/%s", t.Collection, t.Blueprint), nil
}

// editable finds a collection's blueprint by key; a built-in has no file to edit.
func (s *BlueprintsService) editable(key string) (vm.BlueprintChoice, error) {
	var bp vm.BlueprintChoice
	var ok bool
	s.app.actor.Do(func() { bp, ok = s.find(key) })
	if !ok {
		return bp, errors.New("that blueprint is gone, or is built in and cannot be edited")
	}
	return bp, nil
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
			Description: bp.Description, Kind: kind, Params: []BlueprintParam{}, Nodes: len(bp.Nodes), Steps: blueprintSteps(bp.Nodes),
			Group: "hooks",
		}
		if kind == "flow" {
			b.Group = "flow"
		} else if of, ok := strings.CutPrefix(bp.Path, "defaults/"); ok {
			// defaults/<kind>-<phase>: the collection's own version of a built-in default.
			if k, _, _ := strings.Cut(of, "-"); slices.Contains(flowi.DefaultKinds(), k) {
				b.Group, b.ReplacesDefault = k, true
			}
		}
		for _, p := range bp.Params {
			b.Params = append(b.Params, BlueprintParam{Name: p.Name, Description: p.Description, Type: string(p.Type), Default: p.Default})
		}
		v.Blueprints = append(v.Blueprints, b)
	}
	return v
}

// builtInBlueprints are the hooks new requests start with by default, per
// kind of request (flowi-lib's requests.seed_blueprint "default").
func builtInBlueprints() []BlueprintView {
	var out []BlueprintView
	for _, group := range flowi.DefaultKinds() {
		for _, bp := range flowi.DefaultBlueprints(group) {
			kind, phase := "post-hooks", "post"
			if bp.Phase == "pre" {
				kind, phase = "pre-hooks", "pre"
			}
			out = append(out, BlueprintView{
				Key: "built-in/" + group + "-" + phase, CollectionN: "Built in", Name: bp.Name, Description: bp.Description,
				Kind: kind, Params: []BlueprintParam{}, Nodes: len(bp.Body.Nodes), Steps: blueprintSteps(bp.Body.Nodes), Group: group, BuiltIn: true,
			})
		}
	}
	return out
}
