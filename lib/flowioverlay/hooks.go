// SPDX-License-Identifier: Apache-2.0

package flowioverlay

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// hookPhase is the list of hook steps a version has under the name after
// "hooks." in an overlay path (pre, post or message).
func hookPhase(name string) (func(*model.Hooks) *[]model.HookStep, bool) {
	switch name {
	case "pre":
		return func(h *model.Hooks) *[]model.HookStep { return &h.Pre }, true
	case "post":
		return func(h *model.Hooks) *[]model.HookStep { return &h.Post }, true
	case "message":
		return func(h *model.Hooks) *[]model.HookStep { return &h.Message }, true
	}
	return nil, false
}

// phaseNames is the phases in the order a version runs them.
func phaseNames() []string { return []string{"pre", "post", "message"} }

// applyHooks adds a reusable flow to a hook list or takes one out. Only flows
// can be named: an inline hook is a set of nodes, which an overlay does not
// carry.
func applyHooks(v *model.RequestVersion, op model.OverlayOp, phase string) (string, string) {
	list, ok := hookPhase(phase)
	if !ok {
		return "hooks is hooks.pre, hooks.post or hooks.message", ""
	}
	ref, err := hookRef(op.Value)
	if err != nil {
		return err.Error(), ""
	}
	steps := list(&v.Hooks)
	i := slices.IndexFunc(*steps, func(s model.HookStep) bool { return sameFlow(s, ref) })
	switch op.Verb {
	case model.OverlayAdd:
		if i >= 0 {
			return "the flow " + quote(string(ref.ID)) + " is already a " + phase + " hook of this version", ""
		}
		*steps = append(*steps, model.HookStep{Flow: &ref})
		return "", ""
	case model.OverlayRemove:
		if i < 0 {
			return "the flow " + quote(string(ref.ID)) + " is not a " + phase + " hook of this version", ""
		}
		*steps = slices.Delete(*steps, i, i+1)
		return "", ""
	}
	return "does not take " + op.Verb, ""
}

func sameFlow(s model.HookStep, ref model.Ref) bool {
	return s.Flow != nil && s.Flow.ID == ref.ID && s.Flow.Collection == ref.Collection
}

// hookRef reads the flow an operation names: a flow id, or a map with the
// flow's "flow" id and optionally "collection" and "path" (a readable hint).
func hookRef(value any) (model.Ref, error) {
	switch t := value.(type) {
	case string:
		if t == "" {
			return model.Ref{}, fmt.Errorf("name the flow by its id")
		}
		return model.Ref{ID: model.ID(t)}, nil
	case map[string]any:
		id := text(t["flow"])
		if id == "" {
			return model.Ref{}, fmt.Errorf("a hook needs flow: <id>")
		}
		return model.Ref{ID: model.ID(id), Collection: text(t["collection"]), Path: text(t["path"])}, nil
	}
	return model.Ref{}, fmt.Errorf("a hook is a flow id, or a map with flow: <id>")
}

// hookValue is the overlay value that names ref (an id alone when nothing
// else says where it is).
func hookValue(ref model.Ref) any {
	if ref.Collection == "" && ref.Path == "" {
		return string(ref.ID)
	}
	v := map[string]any{"flow": string(ref.ID)}
	if ref.Collection != "" {
		v["collection"] = ref.Collection
	}
	if ref.Path != "" {
		v["path"] = ref.Path
	}
	return v
}

func cloneHooks(h model.Hooks) model.Hooks {
	return model.Hooks{Pre: slices.Clone(h.Pre), Post: slices.Clone(h.Post), Message: slices.Clone(h.Message)}
}

// diffHooks turns the change of the flow hooks into operations. Inline hooks
// cannot be carried: a change to them is the returned note.
func diffHooks(base, edited model.Hooks) (ops []model.OverlayOp, unsupported string) {
	b, e := base, edited
	for _, phase := range phaseNames() {
		list, _ := hookPhase(phase)
		bs, es := *list(&b), *list(&e)
		if !reflect.DeepEqual(inlineOf(bs), inlineOf(es)) {
			unsupported = "inline hooks (an overlay can add or remove a reusable flow, not nodes written into the request)"
		}
		ops = append(ops, hookOps(phase, bs, es)...)
	}
	return ops, unsupported
}

func inlineOf(steps []model.HookStep) []model.HookStep {
	return slices.DeleteFunc(slices.Clone(steps), func(s model.HookStep) bool { return s.Flow != nil })
}

func hookOps(phase string, base, edited []model.HookStep) []model.OverlayOp {
	var ops []model.OverlayOp
	for _, s := range base {
		if s.Flow != nil && !slices.ContainsFunc(edited, func(x model.HookStep) bool { return x.Flow != nil && sameFlow(x, *s.Flow) }) {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayRemove, Path: "hooks." + phase, Value: hookValue(*s.Flow)})
		}
	}
	for _, s := range edited {
		if s.Flow != nil && !slices.ContainsFunc(base, func(x model.HookStep) bool { return x.Flow != nil && sameFlow(x, *s.Flow) }) {
			ops = append(ops, model.OverlayOp{Verb: model.OverlayAdd, Path: "hooks." + phase, Value: hookValue(*s.Flow)})
		}
	}
	return ops
}
