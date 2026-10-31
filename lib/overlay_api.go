// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/model"
)

// overlayFor finds the flowi overlay name in collection c and returns it as
// the operations that apply to request r. "" is no overlay. An unknown name
// is FLW-OVL-002.
func overlayFor(c model.Collection, r model.Request, name string) (*engine.Overlay, error) {
	if name == "" {
		return nil, nil
	}
	ops, missing := flowioverlay.Resolve(c.Overlays, name, r.ID)
	if missing != "" {
		return nil, diagErr(diag.CodeOverlayNotFound, map[string]string{"name": missing})
	}
	return &engine.Overlay{Name: name, Ops: ops}, nil
}

// Overlays lists the flowi overlays of a collection, by name.
func (w *Workspace) Overlays(collection model.ID) ([]model.Overlay, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	out := slices.Clone(c.Overlays)
	slices.SortFunc(out, func(a, b model.Overlay) int { return compareStrings(a.Name, b.Name) })
	return out, nil
}

func compareStrings(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// Overlay returns one flowi overlay of a collection (FLW-OVL-002 if absent).
func (w *Workspace) Overlay(collection model.ID, name string) (model.Overlay, error) {
	all, err := w.Overlays(collection)
	if err != nil {
		return model.Overlay{}, err
	}
	i := slices.IndexFunc(all, func(o model.Overlay) bool { return o.Name == name })
	if i < 0 {
		return model.Overlay{}, diagErr(diag.CodeOverlayNotFound, map[string]string{"name": name})
	}
	return all[i], nil
}

// OverlayDiff is what saving an edited version as a flowi overlay would
// store: the operations (computed against the base version) and the parts of
// the edit an overlay cannot hold.
type OverlayDiff struct {
	Ops         []model.OverlayOp
	Unsupported []string
}

// DiffForOverlay works out the operations that turn the saved version base of
// request t into edited. Nothing is written.
func (w *Workspace) DiffForOverlay(t RequestTarget, base string, edited model.RequestVersion) (OverlayDiff, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return OverlayDiff{}, err
	}
	b, err := versionOf(r, base)
	if err != nil {
		return OverlayDiff{}, err
	}
	ops, unsupported := flowioverlay.Diff(b, edited)
	return OverlayDiff{Ops: ops, Unsupported: unsupported}, nil
}

func versionOf(r model.Request, label string) (model.RequestVersion, error) {
	i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == label })
	if i < 0 {
		return model.RequestVersion{}, diagErr(diag.CodeVersionPinnedAbsent, map[string]string{"label": label, "request": r.Path})
	}
	return r.Versions[i], nil
}

// SaveOverlayInput describes saving an edit as a flowi overlay.
type SaveOverlayInput struct {
	Name        string
	Description string // kept when empty and the overlay exists
	Base        string // the saved version the edit started from
	Edited      model.RequestVersion
	// Shared lists the indexes (into the DiffForOverlay operations) that go to
	// the overlay's shared part; the rest stay with this request.
	Shared []int
}

// SaveAsOverlay stores the edit of request t as the flowi overlay in.Name:
// the operations marked shared join the overlay's shared part, the others
// replace this request's own part. An overlay that exists keeps the parts of
// its other requests. Problems are *diag.ListError (FLW-OVL-005 for a bad
// name, FLW-WRITE-001 when the file has comments and o does not allow
// dropping them).
func (w *Workspace) SaveAsOverlay(t RequestTarget, in SaveOverlayInput, o SaveOptions) error {
	if err := flowioverlay.CheckName(in.Name); err != nil {
		return diagErr(diag.CodeOverlayName, map[string]string{"name": in.Name, "detail": err.Error()})
	}
	c, r, err := w.requestFor(t)
	if err != nil {
		return err
	}
	diff, err := w.DiffForOverlay(t, in.Base, in.Edited)
	if err != nil {
		return err
	}
	ov := overlayNamed(c, in.Name)
	if in.Description != "" {
		ov.Description = in.Description
	}
	shared, own := splitOps(diff.Ops, in.Shared)
	// Editing with an overlay open shows its shared changes too; they are
	// already in the shared part, so they are not repeated in this request's.
	own = slices.DeleteFunc(own, func(op model.OverlayOp) bool { return alreadyShared(ov.Shared, op) })
	ov.Shared = joinOps(ov.Shared, shared)
	ov.Requests = withRequestPart(ov.Requests, model.RequestOverlay{Request: r.ID, Path: r.Path, Ops: own})
	return w.WriteOverlay(t.Collection, ov, o)
}

func overlayNamed(c model.Collection, name string) model.Overlay {
	if i := slices.IndexFunc(c.Overlays, func(o model.Overlay) bool { return o.Name == name }); i >= 0 {
		return c.Overlays[i]
	}
	return model.Overlay{Name: name}
}

func alreadyShared(shared []model.OverlayOp, op model.OverlayOp) bool {
	return slices.ContainsFunc(shared, func(s model.OverlayOp) bool {
		return s.Verb == op.Verb && s.Path == op.Path && reflect.DeepEqual(s.Value, op.Value)
	})
}

func splitOps(ops []model.OverlayOp, shared []int) (toShared, own []model.OverlayOp) {
	for i, op := range ops {
		if slices.Contains(shared, i) {
			toShared = append(toShared, op)
		} else {
			own = append(own, op)
		}
	}
	return toShared, own
}

// joinOps adds extra to base; an operation on a path base already has
// replaces it, so saving the same change twice does not stack.
func joinOps(base, extra []model.OverlayOp) []model.OverlayOp {
	out := slices.Clone(base)
	for _, op := range extra {
		out = slices.DeleteFunc(out, func(x model.OverlayOp) bool {
			return x.Path == op.Path && x.Verb == op.Verb && op.Verb != model.OverlayAdd
		})
		out = append(out, op)
	}
	return out
}

// withRequestPart replaces the part for part.Request (dropping it when it
// holds nothing).
func withRequestPart(parts []model.RequestOverlay, part model.RequestOverlay) []model.RequestOverlay {
	out := slices.DeleteFunc(slices.Clone(parts), func(p model.RequestOverlay) bool { return p.Request == part.Request })
	if len(part.Ops) > 0 {
		out = append(out, part)
	}
	return out
}

// WriteOverlay validates ov and writes it as overlays/<name>.flowi-overlay.yaml,
// replacing the overlay of that name if there is one. An operation no version
// could take is FLW-OVL-001.
func (w *Workspace) WriteOverlay(collection model.ID, ov model.Overlay, o SaveOptions) error {
	if err := flowioverlay.CheckName(ov.Name); err != nil {
		return diagErr(diag.CodeOverlayName, map[string]string{"name": ov.Name, "detail": err.Error()})
	}
	if err := checkOverlayOps(ov); err != nil {
		return diagErr(diag.CodeOverlayInvalid, map[string]string{"detail": err.Error()})
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	data, err := w.store.EncodeOverlay(ov)
	if err != nil {
		return err
	}
	file := filepath.Join(c.Root, "overlays", ov.Name+flowioverlay.Suffix)
	return w.writeAndReload(collection, file, data, o)
}

func checkOverlayOps(ov model.Overlay) error {
	all := slices.Clone(ov.Shared)
	for _, r := range ov.Requests {
		all = append(all, r.Ops...)
	}
	for _, op := range all {
		if err := flowioverlay.CheckOp(op); err != nil {
			return fmt.Errorf("%s %s: %w", op.Verb, op.Path, err)
		}
	}
	return nil
}

// DeleteOverlay removes the overlay's file. Responses kept for it stay (they
// are in the data folder, not the collection). An unknown name is FLW-OVL-002.
func (w *Workspace) DeleteOverlay(collection model.ID, name string) error {
	ov, err := w.Overlay(collection, name)
	if err != nil {
		return err
	}
	return w.removeFile(collection, ov.File)
}

// ApplyOverlay lays overlay name over version v of request t, as a send would,
// and returns the result with what did not fit or drifted. It is what an
// editor shows when an overlay is chosen. Nothing is written.
func (w *Workspace) ApplyOverlay(t RequestTarget, name string, v model.RequestVersion) (model.RequestVersion, []flowioverlay.Problem, error) {
	c, r, err := w.requestFor(t)
	if err != nil {
		return v, nil, err
	}
	use, err := overlayFor(c, r, name)
	if err != nil || use == nil {
		return v, nil, err
	}
	out, problems := flowioverlay.Apply(v, use.Ops)
	return out, problems, nil
}

// OverlayUses lists the requests (by path) an overlay has a part of its own
// for; the shared part applies to every request.
func OverlayUses(ov model.Overlay) []string {
	out := make([]string, 0, len(ov.Requests))
	for _, r := range ov.Requests {
		out = append(out, r.Path)
	}
	return out
}

// OverlayText returns the file of the flowi overlay name, as written.
func (w *Workspace) OverlayText(collection model.ID, name string) (string, error) {
	ov, err := w.Overlay(collection, name)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(ov.File)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", ov.File, err)
	}
	return string(data), nil
}

// WriteOverlayText saves text, written by hand or brought from elsewhere, as
// the flowi overlay name, after checking that it reads as one and that every
// operation is one a version could take. The text is written as given, so
// its comments stay. Problems are FLW-OVL-001 (with the file's own
// diagnostics) and FLW-OVL-005 (the name).
func (w *Workspace) WriteOverlayText(collection model.ID, name, text string) error {
	if err := flowioverlay.CheckName(name); err != nil {
		return diagErr(diag.CodeOverlayName, map[string]string{"name": name, "detail": err.Error()})
	}
	ov, diags := flowioverlay.Decode("", name, []byte(text))
	if diags.HasErrors() {
		return diags.Err()
	}
	if err := checkOverlayOps(ov); err != nil {
		return diagErr(diag.CodeOverlayInvalid, map[string]string{"detail": err.Error()})
	}
	w.mu.RLock()
	c, ok := w.collection(collection)
	w.mu.RUnlock()
	if !ok {
		return diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	file := filepath.Join(c.Root, "overlays", name+flowioverlay.Suffix)
	return w.writeAndReload(collection, file, []byte(text), SaveOptions{DropComments: true})
}

// NewOverlayText is the text the overlay editor starts a new overlay with.
func NewOverlayText() string {
	return "flowi_overlay: 1\ndescription: \nshared:\n  - set: headers.X-Example\n    value: example\n"
}
