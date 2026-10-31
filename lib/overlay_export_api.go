// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/model"
)

// A flowi overlay can be written out as an OpenAPI Overlay for the specs the
// collection keeps, so a colleague who does not use Flowi, or another tool,
// gets what a spec can hold: an operation's docs, its parameters and its JSON
// body example. Everything else (checks, extractions, hooks, settings,
// variables, a changed URL) stays in the flowi overlay, and is listed as not
// carried. The document has no "extends": the kept copy's name is not the name
// of the file a colleague imports, and a wrong "extends" only draws a warning.
// It is a one-way export: the OpenAPI Overlay is what the spec
// holds, the flowi overlay is a request as sent.

// ExportNote is a change of the flowi overlay the OpenAPI Overlay does not
// carry, and why.
type ExportNote struct {
	Request string // the request's path; "" when it is about the overlay as a whole
	What    string
	Why     string
}

// SpecOverlayDocument is one OpenAPI Overlay, written for one kept spec.
type SpecOverlayDocument struct {
	Spec     KeptSpec
	YAML     string
	Requests []string // the paths of the requests it has actions for
}

// OverlayExport is the OpenAPI Overlays a flowi overlay makes, one per kept
// spec it changes, and what they leave out.
type OverlayExport struct {
	Documents  []SpecOverlayDocument
	NotCarried []ExportNote
}

// flowiOnlyFields lists the overlay fields a spec has no place for, with the
// phrase PreviewSpecOverlay uses for them.
func flowiOnlyFields() map[string]string {
	return map[string]string{
		"name": "the version title", "assert": "the checks", "extract": "the extractions", "hooks": "the hooks",
		"settings": "the settings", "variables": "the variables",
	}
}

// ExportOverlayAsSpec writes the flowi overlay name as OpenAPI Overlays for
// the specs kept in its collection. A request is covered when it comes from a
// kept spec and the overlay (its shared part, or a part of its own) changes
// something a spec holds. Nothing is written to disk.
func (w *Workspace) ExportOverlayAsSpec(collection model.ID, name string) (OverlayExport, error) {
	ov, err := w.Overlay(collection, name)
	if err != nil {
		return OverlayExport{}, err
	}
	c, ok := w.collection(collection)
	if !ok {
		return OverlayExport{}, fmt.Errorf("collection %s not found", collection)
	}
	var out OverlayExport
	groups := map[string]*SpecOverlayDocument{}
	actions := map[string][]overlayAction{}
	for _, r := range byPath(c.Requests) {
		ops := flowioverlay.ForRequest(ov, r.ID)
		if len(ops) == 0 {
			continue
		}
		plan, found, err := w.planFor(collection, r, ops)
		if err != nil {
			return OverlayExport{}, err
		}
		if !found {
			if listed(ov, r.ID) {
				out.NotCarried = append(out.NotCarried, ExportNote{Request: r.Path, What: "the whole request", Why: "it was not imported from a spec kept in the collection"})
			}
			continue
		}
		out.NotCarried = append(out.NotCarried, notesFor(r.Path, ops, plan)...)
		doc := plan.document(allChanges(plan))
		if len(doc.Actions) == 0 {
			continue
		}
		g := groups[plan.Spec.Rel]
		if g == nil {
			g = &SpecOverlayDocument{Spec: plan.Spec}
			groups[plan.Spec.Rel] = g
		}
		g.Requests = append(g.Requests, r.Path)
		actions[plan.Spec.Rel] = append(actions[plan.Spec.Rel], doc.Actions...)
	}
	if len(groups) == 0 && len(out.NotCarried) == 0 {
		out.NotCarried = append(out.NotCarried, ExportNote{What: "the whole overlay", Why: "no request of this collection was imported from a spec the collection keeps, so there is nothing in a spec to change"})
	}
	for _, rel := range slices.Sorted(maps.Keys(groups)) {
		g := groups[rel]
		data, err := yaml.Marshal(overlayDoc{Overlay: "1.0.0", Info: exportInfo(name), Actions: actions[rel]})
		if err != nil {
			return OverlayExport{}, fmt.Errorf("encoding the overlay for %s: %w", rel, err)
		}
		g.YAML = string(data)
		out.Documents = append(out.Documents, *g)
	}
	return out, nil
}

// planFor works out what version of request r would change in its spec once
// ops are laid over it, trying each version until one belongs to a spec.
func (w *Workspace) planFor(collection model.ID, r model.Request, ops []model.OverlayOp) (OverlayPlan, bool, error) {
	for _, v := range r.Versions {
		laid, _ := flowioverlay.Apply(v, ops)
		plan, err := w.PreviewSpecOverlay(RequestTarget{Collection: collection, Request: r.ID}, laid)
		switch {
		case err == nil:
			plan.others = nil // the export says only what the flowi overlay says
			return plan, true, nil
		case errors.Is(err, ErrNotFromSpec):
			continue
		default:
			return OverlayPlan{}, false, fmt.Errorf("planning %s: %w", r.Path, err)
		}
	}
	return OverlayPlan{}, false, nil
}

func exportInfo(name string) overlayInfo {
	return overlayInfo{Title: "Flowi overlay " + name, Version: "1.0.0"}
}

func allChanges(p OverlayPlan) []int {
	out := make([]int, len(p.Changes))
	for i := range out {
		out[i] = i
	}
	return out
}

func listed(ov model.Overlay, id model.ID) bool {
	return slices.ContainsFunc(ov.Requests, func(r model.RequestOverlay) bool { return r.Request == id })
}

// notesFor lists what ops change that the plan does not carry: the fields a
// spec has no place for, and the plan's own notes about parameters and bodies.
func notesFor(request string, ops []model.OverlayOp, plan OverlayPlan) []ExportNote {
	const why = "the spec has no place for it; the flowi overlay keeps it"
	var out []ExportNote
	only := flowiOnlyFields()
	seen := map[string]bool{}
	for _, op := range ops {
		field, _, _ := strings.Cut(op.Path, ".")
		if what, ok := only[field]; ok && !seen[what] {
			seen[what] = true
			out = append(out, ExportNote{Request: request, What: what, Why: why})
		}
	}
	own := make([]string, 0, len(only))
	for _, what := range only {
		own = append(own, what)
	}
	for _, n := range plan.NotCarried {
		if !slices.Contains(own, n.What) {
			out = append(out, ExportNote{Request: request, What: n.What, Why: n.Why})
		}
	}
	return out
}

func byPath(requests []model.Request) []model.Request {
	return slices.SortedFunc(slices.Values(requests), func(a, b model.Request) int { return strings.Compare(a.Path, b.Path) })
}
