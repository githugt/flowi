// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

// HookPhase is the half of a request's pipeline a hook belongs to.
type HookPhase string

// Phases.
const (
	PhasePre  HookPhase = "pre"
	PhasePost HookPhase = "post"
)

// HookEditor edits the hooks a request declares itself: the nodes of its
// inline hook steps, one flow document per phase, so the node commands, undo
// and the inspector are the flow editor's. The nodes of every inline step of
// a phase are edited as one list and written back as one inline step. Hook
// steps that name a flow, and the hooks of other levels (collection, folder,
// version), are not edited here: flow steps are kept where they were and the
// others are only counted.
type HookEditor struct {
	req  *Request
	docs map[HookPhase]*FlowDoc
	// kept are the phase's flow steps with their place among the original
	// steps; firstInline is where the edited nodes go among them.
	kept        map[HookPhase][]keptStep
	firstInline map[HookPhase]int
}

type keptStep struct {
	at   int
	step model.HookStep
}

// hookBackend is the backend for the hook documents: they have no file, so
// they have no layout to load.
type hookBackend struct{ Backend }

func (hookBackend) FlowLayout(flowi.FlowTarget) (model.Layout, diag.List, error) {
	return model.Layout{}, nil, nil
}

// NewHookEditor opens the hooks of req.
func NewHookEditor(b Backend, w *Worker, req *Request) *HookEditor {
	e := &HookEditor{req: req, docs: map[HookPhase]*FlowDoc{}, kept: map[HookPhase][]keptStep{}, firstInline: map[HookPhase]int{}}
	hooks := req.Hooks()
	for phase, steps := range map[HookPhase][]model.HookStep{PhasePre: hooks.Pre, PhasePost: hooks.Post} {
		e.docs[phase] = NewFlowDoc(hookBackend{b}, w, req.Collection(), e.split(phase, steps))
	}
	return e
}

// split separates the steps a phase's document edits from the ones it keeps,
// and returns the document's flow.
func (e *HookEditor) split(phase HookPhase, steps []model.HookStep) model.Flow {
	var nodes []model.Node
	taken := map[string]bool{}
	e.firstInline[phase] = len(steps)
	for i, s := range steps {
		if s.Flow != nil {
			e.kept[phase] = append(e.kept[phase], keptStep{at: i, step: s})
			continue
		}
		e.firstInline[phase] = min(e.firstInline[phase], i)
		// Each inline step ran with its own nodes.<id>; in one list the ids
		// must differ, so a repeated one (two blueprints with the same
		// nodes) is renamed.
		nodes = append(nodes, freshNodes(s.Inline, taken)...)
	}
	title := map[HookPhase]string{PhasePre: "Pre-hooks", PhasePost: "Post-hooks"}[phase]
	return model.Flow{Meta: model.Meta{ID: model.ID("hooks-" + string(phase)), Name: title}, Nodes: nodes}
}

// Doc is the document of a phase's nodes.
func (e *HookEditor) Doc(p HookPhase) *FlowDoc { return e.docs[p] }

// Dirty reports whether any phase has unsaved edits.
func (e *HookEditor) Dirty() bool { return e.docs[PhasePre].Dirty() || e.docs[PhasePost].Dirty() }

// Others counts the hook steps of a phase that run but are not edited here:
// flow steps of the request and every step of the other levels.
func (e *HookEditor) Others(p HookPhase) int {
	chain := engine.ChainFor(e.req.Collection(), e.req.Draft(), e.req.Current())
	all := len(chain.Pre)
	own := len(e.req.Hooks().Pre)
	if p == PhasePost {
		all, own = len(chain.Post), len(e.req.Hooks().Post)
	}
	inlineOwn := own - len(e.kept[p])
	return all - inlineOwn
}

// MoveNode moves a node to index of a phase's list, which may be the other
// phase. A node that changes phase keeps its content and gets a free id.
func (e *HookEditor) MoveNode(from HookPhase, id string, to HookPhase, index int) {
	if from == to {
		_ = e.docs[from].Move(id, "$.nodes", index)
		return
	}
	n, ok := e.docs[from].Node(id)
	if !ok {
		return
	}
	moved := freshNodes([]model.Node{n}, idsOf(e.docs[to]))[0]
	e.docs[from].Delete(id)
	e.docs[to].Insert(moved.Type, "$.nodes", index, func(x *model.Node) { *x = moved })
}

// idsOf is every node and branch id of a document.
func idsOf(d *FlowDoc) map[string]bool {
	taken := map[string]bool{}
	allIDs(d.Draft().Nodes, taken)
	return taken
}

// freshNodes copies nodes deeply, giving each node and branch whose id is in
// taken a free one, and follows the renames in the copies' needs and
// {{nodes.<id>…}} templates. The ids used are added to taken.
func freshNodes(nodes []model.Node, taken map[string]bool) []model.Node {
	out := cloneNodes(nodes)
	renames := map[string]string{}
	var visit func(list []model.Node)
	visit = func(list []model.Node) {
		for i := range list {
			list[i].ID = claimID(list[i].ID, taken, renames)
			for b := range list[i].Branches {
				list[i].Branches[b].ID = claimID(list[i].Branches[b].ID, taken, renames)
			}
			for _, child := range listsOf(&list[i]) {
				visit(*child)
			}
		}
	}
	visit(out)
	for old, fresh := range renames {
		renameNeeds(out, old, fresh)
		renameReferences(out, old, fresh)
	}
	return out
}

// claimID returns id if it is free, else a free id built on it, and marks it
// taken.
func claimID(id string, taken map[string]bool, renames map[string]string) string {
	if !taken[id] {
		taken[id] = true
		return id
	}
	fresh := freeID(id, taken)
	taken[fresh] = true
	renames[id] = fresh
	return fresh
}

// renameReferences rewrites {{nodes.old…}} in the copies' templates.
func renameReferences(list []model.Node, old, fresh string) {
	for i := range list {
		list[i].When = strings.ReplaceAll(list[i].When, "nodes."+old+".", "nodes."+fresh+".")
		for j, f := range list[i].Params {
			if text, ok := f.Value.(string); ok {
				list[i].Params[j].Value = strings.ReplaceAll(text, "nodes."+old+".", "nodes."+fresh+".")
			}
		}
		for _, child := range listsOf(&list[i]) {
			renameReferences(*child, old, fresh)
		}
	}
}

// Save writes the edited nodes into the request's hooks and saves the
// request (which saves any other unsaved edit of it too). done gets the
// outcome on the UI goroutine: nil, an error that IsDropsComments recognises
// (ask, then Save again with dropComments), or any other failure; after a
// failure the edits are still unsaved. A request that changed on disk since
// the editor opened is not overwritten.
func (e *HookEditor) Save(dropComments bool, done func(error)) {
	if e.req.ChangedOnDisk() {
		done(errors.New("this request changed on disk since the editor opened: exit, then edit it again"))
		return
	}
	preDraft, preRev := e.docs[PhasePre].Snapshot()
	postDraft, postRev := e.docs[PhasePost].Snapshot()
	e.req.SaveHooks(model.Hooks{Pre: e.stepsOf(PhasePre, preDraft), Post: e.stepsOf(PhasePost, postDraft)}, dropComments, func(err error) {
		if err == nil {
			e.docs[PhasePre].MarkSavedAt(preDraft, preRev)
			e.docs[PhasePost].MarkSavedAt(postDraft, postRev)
		}
		done(err)
	})
}

// steps is a phase's steps as they are written: the kept flow steps in their
// places and the edited nodes as one inline step where the first inline step
// was.
func (e *HookEditor) stepsOf(p HookPhase, draft model.Flow) []model.HookStep {
	var out []model.HookStep
	nodes := draft.Nodes
	placed := len(nodes) == 0
	for _, k := range e.kept[p] {
		if !placed && k.at > e.firstInline[p] {
			out, placed = append(out, model.HookStep{Inline: cloneNodes(nodes)}), true
		}
		out = append(out, k.step)
	}
	if !placed {
		out = append(out, model.HookStep{Inline: cloneNodes(nodes)})
	}
	return out
}

// Blueprints are the blueprints that can be used as hooks: those without
// parameters, which nothing could fill.
func (e *HookEditor) Blueprints(ws *Workspace) []BlueprintChoice {
	var out []BlueprintChoice
	for _, b := range ws.Blueprints() {
		if len(b.Params) == 0 && len(b.Nodes) > 0 {
			out = append(out, b)
		}
	}
	return out
}

// InsertBlueprint appends the blueprint's nodes to the end of its phase, with
// ids the phase does not use yet.
func (e *HookEditor) InsertBlueprint(bp BlueprintChoice) {
	doc := e.docs[HookPhase(bp.Phase)]
	if doc == nil {
		return
	}
	for _, n := range freshNodes(bp.Nodes, idsOf(doc)) {
		doc.Insert(n.Type, "$.nodes", len(doc.Draft().Nodes), func(x *model.Node) { *x = n })
	}
}

// seedKey is the setting that says what a new request starts with.
const seedKey = "requests.seed_blueprint"

// SaveAsBlueprint saves a phase's nodes as the blueprint path of the
// request's collection, and with makeDefault makes it what new requests start
// with in that phase (the user setting). done runs once it is saved, with a
// note when the default will not take effect (a setting of the workspace or
// collection overrides the user's); fail gets any error, and a failure to set
// the default after saving leaves the blueprint saved.
func (e *HookEditor) SaveAsBlueprint(ws *Workspace, phase HookPhase, path, name string, makeDefault bool, done func(note string), fail func(error)) {
	nodes := slices.Clone(e.docs[phase].Draft().Nodes)
	if len(nodes) == 0 {
		if fail != nil {
			fail(errors.New("there are no " + string(phase) + "-hooks to save"))
		}
		return
	}
	collection, known := e.req.Collection().Manifest.ID, ws.Blueprints()
	var note string // written by the worker before create's callbacks run
	ws.create(func() (string, error) {
		if _, err := ws.b.CreateHookBlueprint(collection, path, name, string(phase), nodes); err != nil {
			return "", err
		}
		if !makeDefault {
			return "", nil
		}
		user, overriding := seedSettingOf(ws.b.Settings(collection))
		note = overriding
		return "", ws.b.SetUserSetting(seedKey, SeedAfterAdding(user, path, string(phase), known))
	}, func(string) {
		if done != nil {
			done(note)
		}
	}, fail)
}

// SeedAfterAdding is the value of requests.seed_blueprint after the blueprint
// add, for the hooks of phase, becomes what new requests start with there:
// the entries for the other phase stay, those for this phase go. "default"
// stands for the built-in post hooks; known tells the phase of the others
// (an unknown name is kept), and "off" is treated as default.
func SeedAfterAdding(current, add, phase string, known []BlueprintChoice) string {
	var out []string
	for _, entry := range strings.Split(current, ",") {
		entry = normaliseBlueprintName(strings.TrimSpace(entry))
		switch {
		case entry == "" || entry == "off":
			continue
		case entry == "default" && phase == "post":
			continue
		case entry == "default":
		default:
			if i := slices.IndexFunc(known, func(b BlueprintChoice) bool { return b.Path == entry || string(b.ID) == entry }); i >= 0 && phaseName(known[i].Phase) == phase {
				continue
			}
		}
		out = append(out, entry)
	}
	if len(out) == 0 && phase != "post" {
		out = append(out, "default") // the built-in post hooks stay unless a post blueprint replaces them
	}
	return strings.Join(append(out, add), ", ")
}

func phaseName(p string) string {
	if p == "pre" {
		return "pre"
	}
	return "post"
}

// seedSettingOf is the user's own value of requests.seed_blueprint (not the
// effective one, which another layer may have set) and, if a layer above the
// user's sets it, a note saying which.
func seedSettingOf(r settings.Resolved) (user, overriding string) {
	for _, v := range r.Explain(seedKey) {
		text, _ := v.Value.(string)
		switch v.Layer {
		case settings.LayerUser:
			user = text
		case settings.LayerDefault:
		default:
			if overriding == "" {
				overriding = "requests.seed_blueprint is also set in " + v.Source + ", which overrides your user setting."
			}
		}
	}
	return user, overriding
}

// normaliseBlueprintName is a blueprint's path as the library accepts it in
// the setting: "blueprints/x.blueprint.yaml" and "x" name the same one.
func normaliseBlueprintName(s string) string {
	return strings.TrimSuffix(strings.TrimPrefix(s, "blueprints/"), ".blueprint.yaml")
}
