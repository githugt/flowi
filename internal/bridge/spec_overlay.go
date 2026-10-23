// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// A request imported from a spec kept in its collection can save its edits
// to the spec, as an overlay beside it; the window marks the fields
// overlays set and updates the collection from the spec again. The
// view-model's backend knows none of this, so the workspace is handed in
// apart (WithSpecOverlays), as for the workspace's own file.

// SpecOverlays is what the window needs of the workspace's kept specs.
type SpecOverlays interface {
	SpecOrigin(t flowi.RequestTarget, label string) (flowi.SpecOrigin, error)
	PreviewSpecOverlay(t flowi.RequestTarget, v model.RequestVersion) (flowi.OverlayPlan, error)
	ApplySpecOverlay(p flowi.OverlayPlan, chosen []int) error
	KeptSpecs(collection model.ID) ([]flowi.KeptSpec, error)
	PreviewSpecUpdate(collection model.ID, name string, data []byte) (flowi.ImportPlan, error)
	ApplyImport(p flowi.ImportPlan, o flowi.ApplyOptions) error
}

// WithSpecOverlays lets the window save edits to spec overlays and update
// collections from their specs.
func WithSpecOverlays(s SpecOverlays) Option {
	return func(o *appOptions) { o.specs = s }
}

var errNoSpecs = errors.New("specs are not available here")

// fetchTimeout bounds downloading a newer spec for Update from spec.
const fetchTimeout = 30 * time.Second

// OverlayFieldView is a field of the request an overlay sets.
type OverlayFieldView struct {
	Section string `json:"section"` // docs, body, query or header
	Key     string `json:"key"`     // the parameter's name; "" for docs and body
	File    string `json:"file"`    // the overlay, relative to the collection
}

// SpecOriginView is the kept spec a request comes from; Spec is "" when it
// comes from none.
type SpecOriginView struct {
	Spec       string             `json:"spec"`
	Name       string             `json:"name"`
	Collection string             `json:"collection"`
	Fields     []OverlayFieldView `json:"fields"`
}

// OverlayChangeView is one change saving to the overlay would make.
type OverlayChangeView struct {
	What   string `json:"what"`
	Target string `json:"target"`
	Saved  bool   `json:"saved"` // the overlay already has it
}

// OverlayNoteView is an edit the overlay does not carry, and why.
type OverlayNoteView struct {
	What string `json:"what"`
	Why  string `json:"why"`
}

// OverlayPreviewView is what Save to spec overlay would write.
type OverlayPreviewView struct {
	Spec       string              `json:"spec"`
	Edits      string              `json:"edits"` // the overlay it writes, relative to the collection
	Changes    []OverlayChangeView `json:"changes"`
	NotCarried []OverlayNoteView   `json:"notCarried"`
}

// SpecOrigin says which kept spec the request's selected version comes
// from, and which of its fields overlays set.
func (s *RequestService) SpecOrigin(node string) SpecOriginView {
	v := SpecOriginView{Fields: []OverlayFieldView{}}
	if s.app.specs == nil {
		return v
	}
	s.with(node, func(o *openRequest) {
		t := o.req.Target()
		origin, err := s.app.specs.SpecOrigin(t, o.req.SelectedLabel())
		if err != nil {
			return // from no spec (ErrNotFromSpec), or one that no longer reads: nothing to mark
		}
		v.Spec, v.Name, v.Collection = origin.Spec, origin.Name, string(t.Collection)
		for _, f := range origin.Fields {
			v.Fields = append(v.Fields, OverlayFieldView{Section: f.Section, Key: f.Key, File: f.File})
		}
	})
	return v
}

// PreviewOverlay says what saving the selected version, as edited, to its
// spec's overlay would change.
func (s *RequestService) PreviewOverlay(node string) (OverlayPreviewView, error) {
	var view OverlayPreviewView
	plan, err := s.overlayPlan(node)
	if err != nil {
		return view, err
	}
	view = OverlayPreviewView{Spec: plan.Spec.Rel, Edits: "descriptions/" + filepath.Base(plan.Spec.Edits),
		Changes: []OverlayChangeView{}, NotCarried: []OverlayNoteView{}}
	for _, ch := range plan.Changes {
		view.Changes = append(view.Changes, OverlayChangeView{What: ch.What, Target: ch.Target, Saved: ch.Saved})
	}
	for _, n := range plan.NotCarried {
		view.NotCarried = append(view.NotCarried, OverlayNoteView{What: n.What, Why: n.Why})
	}
	return view, nil
}

// OverlayYAML is the overlay as it would be written with the chosen
// changes (indexes into PreviewOverlay's).
func (s *RequestService) OverlayYAML(node string, chosen []int) (string, error) {
	plan, err := s.overlayPlan(node)
	if err != nil {
		return "", err
	}
	return plan.YAML(chosen)
}

// SaveToOverlay saves the request, then writes the chosen changes to its
// spec's overlay. The outcome is a notice; "spec-origin" says the request's
// markers changed.
func (s *RequestService) SaveToOverlay(node string, chosen []int) error {
	plan, err := s.overlayPlan(node)
	if err != nil {
		return err
	}
	s.with(node, func(o *openRequest) {
		o.req.SaveThen(false, func() {
			if err := s.app.specs.ApplySpecOverlay(plan, chosen); err != nil {
				s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
				return
			}
			s.app.emit("notice", Notice{Level: "info", Text: "Saved, and written to " + "descriptions/" + filepath.Base(plan.Spec.Edits)})
			s.app.emit("spec-origin", node)
		})
	})
	return nil
}

// overlayPlan previews the selected version, as edited, against its spec.
func (s *RequestService) overlayPlan(node string) (flowi.OverlayPlan, error) {
	if s.app.specs == nil {
		return flowi.OverlayPlan{}, errNoSpecs
	}
	var plan flowi.OverlayPlan
	err := fmt.Errorf("request %s is not open", node)
	s.with(node, func(o *openRequest) {
		plan, err = s.app.specs.PreviewSpecOverlay(o.req.Target(), o.req.Current())
	})
	if err != nil {
		return flowi.OverlayPlan{}, fmt.Errorf("previewing the spec overlay: %w", err)
	}
	return plan, nil
}

// KeptSpecView is a spec kept in a collection.
type KeptSpecView struct {
	Name     string   `json:"name"`
	Spec     string   `json:"spec"`     // relative to the collection
	Overlays []string `json:"overlays"` // relative to the collection, in the order they apply, the edits last
}

// SpecUpdateView is the preview of Update from spec.
type SpecUpdateView struct {
	Spec    string   `json:"spec"`
	Added   []string `json:"added"`   // files the update adds
	Changed []string `json:"changed"` // files it rewrites
	Removed []string `json:"removed"` // files it removes
	Notes   []string `json:"notes"`   // what did not carry over, and edits kept where the spec changed too
}

// pendingSpecUpdate is the last previewed update, applied by ApplySpecUpdate.
type pendingSpecUpdate struct {
	collection, name string
	plan             flowi.ImportPlan
}

// KeptSpecs lists the specs kept in a collection.
func (s *ManagerService) KeptSpecs(collection string) ([]KeptSpecView, error) {
	out := []KeptSpecView{}
	if s.app.specs == nil {
		return out, nil
	}
	var specs []flowi.KeptSpec
	var err error
	s.app.actor.Do(func() { specs, err = s.app.specs.KeptSpecs(model.ID(collection)) })
	if err != nil {
		return nil, fmt.Errorf("listing the kept specs: %w", err)
	}
	for _, k := range specs {
		v := KeptSpecView{Name: k.Name, Spec: k.Rel, Overlays: []string{}}
		for _, o := range k.Overlays {
			v.Overlays = append(v.Overlays, "descriptions/"+filepath.Base(o))
		}
		if _, err := os.Stat(k.Edits); err == nil {
			v.Overlays = append(v.Overlays, "descriptions/"+filepath.Base(k.Edits))
		}
		out = append(out, v)
	}
	return out, nil
}

// PreviewSpecUpdate previews importing the kept spec name again: from the
// kept copy when source is "", else from a newer document at source (a
// file or an http(s) URL), which then becomes the kept copy.
func (s *ManagerService) PreviewSpecUpdate(collection, name, source string) (SpecUpdateView, error) {
	if s.app.specs == nil {
		return SpecUpdateView{}, errNoSpecs
	}
	data, err := readSource(strings.TrimSpace(source))
	if err != nil {
		return SpecUpdateView{}, err
	}
	var plan flowi.ImportPlan
	s.app.actor.Do(func() {
		plan, err = s.app.specs.PreviewSpecUpdate(model.ID(collection), name, data)
		if err == nil {
			s.pending = &pendingSpecUpdate{collection: collection, name: name, plan: plan}
		}
	})
	if err != nil {
		return SpecUpdateView{}, fmt.Errorf("previewing the update: %w", err)
	}
	return specUpdateView(name, plan), nil
}

// ApplySpecUpdate applies the update last previewed for the spec, replacing
// the files it rewrites (requests are merged with their edits).
func (s *ManagerService) ApplySpecUpdate(collection, name string) error {
	var err error
	s.app.actor.Do(func() {
		p := s.pending
		if p == nil || p.collection != collection || p.name != name {
			err = errors.New("preview the update first")
			return
		}
		s.pending = nil
		err = s.app.specs.ApplyImport(p.plan, flowi.ApplyOptions{Overwrite: true})
	})
	if err != nil {
		return fmt.Errorf("updating from the spec: %w", err)
	}
	return nil
}

// readSource reads a newer spec: nil for "" (the kept copy).
func readSource(source string) ([]byte, error) {
	switch {
	case source == "":
		return nil, nil
	case strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://"):
		ctx, cancel := context.WithTimeoutCause(context.Background(), fetchTimeout, errors.New("downloading the spec took too long"))
		defer cancel()
		data, _, err := flowi.FetchImportSource(ctx, source)
		if err != nil {
			return nil, fmt.Errorf("downloading %s: %w", source, err)
		}
		return data, nil
	}
	data, err := os.ReadFile(filepath.Clean(source))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", source, err)
	}
	return data, nil
}

func specUpdateView(name string, plan flowi.ImportPlan) SpecUpdateView {
	v := SpecUpdateView{Spec: name, Added: []string{}, Changed: []string{}, Removed: []string{}, Notes: []string{}}
	for _, f := range plan.Files {
		switch {
		case f.Same:
			continue // written back as it is
		case f.Remove:
			v.Removed = append(v.Removed, f.Rel)
		case f.Exists:
			v.Changed = append(v.Changed, f.Rel)
		default:
			v.Added = append(v.Added, f.Rel)
		}
	}
	for _, d := range plan.Diags {
		msg, _ := diag.English().Message(d) // a code without English text shows as its code
		if msg == "" {
			msg = string(d.Code)
		}
		v.Notes = append(v.Notes, msg)
	}
	return v
}
