// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"reflect"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/model"
)

// A flowi overlay chosen on the request screen is laid over the selected
// version, and the draft then shows the effective request: what the editor,
// the code snippet and a send all use. Nothing is written until the user
// saves: to the overlay (the default while one is chosen), to the version, or
// as a new version. The overlay is a view-state: it is not remembered when
// the tab is closed.

// OverlayName is the flowi overlay laid over the selected version, or "".
func (r *Request) OverlayName() string { return r.overlay }

// OverlayNotes say, for each part of the chosen overlay that does not fit the
// selected version (or no longer matches what it was saved against), why.
func (r *Request) OverlayNotes() []string { return r.overlayNotes }

// OverlayNames lists the flowi overlays of the request's collection.
func (r *Request) OverlayNames() []string {
	names := make([]string, 0, len(r.overlays))
	for _, o := range r.overlays {
		names = append(names, o.Name)
	}
	return names
}

// OverlayInfo describes an overlay as the picker shows it.
type OverlayInfo struct {
	Name        string
	Description string
	Shared      int  // changes every request gets
	Own         int  // changes this request gets on top
	Used        bool // this request has a part of its own
}

// Overlays describes the flowi overlays available to this request.
func (r *Request) Overlays() []OverlayInfo {
	out := make([]OverlayInfo, 0, len(r.overlays))
	for _, o := range r.overlays {
		info := OverlayInfo{Name: o.Name, Description: o.Description, Shared: len(o.Shared)}
		for _, p := range o.Requests {
			if p.Request == r.target.Request {
				info.Own, info.Used = len(p.Ops), true
			}
		}
		out = append(out, info)
	}
	return out
}

// SetOverlays gives the request the collection's flowi overlays it already
// has in hand, so a view built at once (a tab being opened) lists them.
func (r *Request) SetOverlays(list []model.Overlay) { r.overlays = list }

// LoadOverlays reads the collection's flowi overlays again (after one was
// saved, changed or deleted) and re-applies the chosen one.
func (r *Request) LoadOverlays() {
	c := r.target.Collection
	Run(r.w, func() []model.Overlay {
		list, _ := r.b.Overlays(c) // a collection that is gone has none
		return list
	}, func(list []model.Overlay) {
		r.overlays = list
		r.overlay = withoutDeleted(r.overlay, list)
		r.applyOverlay()
		r.changed()
	})
}

// withoutDeleted drops from a stack the overlays that no longer exist.
func withoutDeleted(stack string, list []model.Overlay) string {
	kept := slices.DeleteFunc(flowioverlay.Stack(stack), func(name string) bool {
		return !slices.ContainsFunc(list, func(o model.Overlay) bool { return o.Name == name })
	})
	return strings.Join(kept, flowioverlay.StackSeparator)
}

// OverlayStack lists the overlays laid over the selected version, in the order they apply.
func (r *Request) OverlayStack() []string { return flowioverlay.Stack(r.overlay) }

// SelectOverlay lays the flowi overlay name over the selected version; ""
// takes it off. A name like "vip+slow" stacks overlays, the later one winning. Unsaved edits would be lost or mixed in, so they must be
// saved or discarded first.
func (r *Request) SelectOverlay(name string) {
	if name == r.overlay {
		return
	}
	if r.dirty {
		r.err = "Save or discard your edit before choosing an overlay."
		r.changed()
		return
	}
	r.overlay, r.err = name, ""
	r.applyOverlay()
	r.changed()
}

// applyOverlay shows the effective version for the chosen overlay: the saved
// version with the overlay's changes. The overlay is applied by the library,
// so the editor and a send agree on the result.
func (r *Request) applyOverlay() {
	if r.selected >= len(r.disk.Versions) || r.dirty {
		return
	}
	r.overlaySeq++
	seq, base, name, t := r.overlaySeq, r.disk.Versions[r.selected], r.overlay, r.target
	if name == "" {
		r.showEffective(base, nil)
		return
	}
	type applied struct {
		v        model.RequestVersion
		problems []flowioverlay.Problem
		err      error
	}
	Run(r.w, func() applied {
		v, problems, err := r.b.ApplyOverlay(t, name, base)
		return applied{v, problems, err}
	}, func(a applied) {
		if seq != r.overlaySeq || r.dirty || r.abandoned {
			return // a newer choice, or an edit, got there first
		}
		if a.err != nil {
			r.overlay, r.err = "", ErrorText(a.err)
			r.showEffective(base, nil)
			r.changed()
			return
		}
		r.showEffective(a.v, a.problems)
		r.changed()
	})
}

func (r *Request) showEffective(v model.RequestVersion, problems []flowioverlay.Problem) {
	if r.selected >= len(r.draft.Versions) {
		return
	}
	v = cloneRequest(model.Request{Versions: []model.RequestVersion{v}}).Versions[0]
	v.Label = r.draft.Versions[r.selected].Label
	r.draft.Versions[r.selected] = v
	r.rawRows, r.problems = nil, nil
	r.overlayNotes = r.overlayNotes[:0:0]
	for _, p := range problems {
		note := p.Describe() + ": " + p.Reason
		if p.Drift {
			note = p.Describe() + " (applied, but " + p.Reason + ")"
		}
		r.overlayNotes = append(r.overlayNotes, note)
	}
	for _, f := range r.onEdit {
		f()
	}
}

// OverlayMark says that the stacked overlay Overlay sets the row Key of
// Section ("header" or "query") of this request.
type OverlayMark struct {
	Section, Key, Overlay string
}

// OverlayMarks lists the header and query rows the chosen overlays set, so the
// editor can tag them. A row two overlays set carries the later one.
func (r *Request) OverlayMarks() []OverlayMark {
	var out []OverlayMark
	sections := map[string]string{"headers": "header", "query": "query"}
	for _, name := range flowioverlay.Stack(r.overlay) {
		ops, _ := flowioverlay.Resolve(r.overlays, name, r.target.Request)
		for _, op := range ops {
			field, key, _ := strings.Cut(op.Path, ".")
			section, ok := sections[field]
			if !ok || op.Verb != model.OverlaySet || key == "" {
				continue
			}
			out = slices.DeleteFunc(out, func(m OverlayMark) bool { return m.Section == section && strings.EqualFold(m.Key, key) })
			out = append(out, OverlayMark{Section: section, Key: key, Overlay: name})
		}
	}
	return out
}

// OverlayHolds reports whether the chosen overlay already makes the change op
// to this request: in its shared part or in this request's own. The save
// dialog leaves those out, since the working copy shows them only because the
// overlay is laid over it.
func (r *Request) OverlayHolds(op model.OverlayOp) bool {
	held, _ := flowioverlay.Resolve(r.overlays, r.overlay, r.target.Request)
	return slices.ContainsFunc(held, func(h model.OverlayOp) bool {
		return h.Verb == op.Verb && h.Path == op.Path && reflect.DeepEqual(h.Value, op.Value)
	})
}

// OverlayDiff works out what saving the working copy as a flowi overlay
// would store, against the saved version it started from.
func (r *Request) OverlayDiff(done func(flowi.OverlayDiff, error)) {
	t, base, edited := r.target, r.SelectedLabel(), r.Current()
	type result struct {
		d   flowi.OverlayDiff
		err error
	}
	Run(r.w, func() result {
		d, err := r.b.DiffForOverlay(t, base, edited)
		return result{d, err}
	}, func(res result) { done(res.d, res.err) })
}

// SaveAsOverlay stores the working copy as the flowi overlay in.Name (the
// operations at in.Shared go to its shared part), then shows the request with
// that overlay laid over its saved version. The working copy's own stored
// edit is cleared: its content now lives in the overlay.
func (r *Request) SaveAsOverlay(in flowi.SaveOverlayInput, dropComments bool, done func(error)) {
	in.Base, in.Edited = r.SelectedLabel(), r.Current()
	t := r.target
	r.cancelAutosave()
	r.saving++
	r.write(func() error {
		if err := r.b.SaveAsOverlay(t, in, dropComments); err != nil {
			return err
		}
		return r.b.DiscardEdit(t)
	}, func(err error) {
		r.saving--
		if err != nil {
			r.err = ErrorText(err)
			r.changed()
			done(err)
			return
		}
		r.dirty, r.slot, r.err, r.overlay = false, false, "", in.Name
		r.LoadOverlays()
		done(nil)
	})
}
