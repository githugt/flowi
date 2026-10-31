// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"os"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// A flowi overlay is a named set of changes laid over a request version (the
// same request as a VIP customer, a delayed request). It is not the spec
// overlay of spec_overlay.go, which fixes an OpenAPI description.

// FlowiOverlayView is a flowi overlay as the picker lists it.
type FlowiOverlayView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Shared      int    `json:"shared"`
	Own         int    `json:"own"`  // changes only this request gets
	Used        bool   `json:"used"` // this request has a part of its own
}

// FlowiOverlayOpView is one change the working copy would add to an overlay.
type FlowiOverlayOpView struct {
	Index   int    `json:"index"`
	Summary string `json:"summary"`
}

// FlowiOverlayDiffView is what saving the working copy as an overlay stores.
type FlowiOverlayDiffView struct {
	Ops         []FlowiOverlayOpView `json:"ops"`
	Unsupported []string             `json:"unsupported"` // parts of the edit an overlay cannot hold
	Names       []string             `json:"names"`       // overlays of the collection
	Open        string               `json:"open"`        // the overlay laid over the request now
}

// FlowiOverlayMark tags a header or query row an overlay sets.
type FlowiOverlayMark struct {
	Section string `json:"section"` // "header" or "query"
	Key     string `json:"key"`
	Overlay string `json:"overlay"`
}

func flowiOverlayMarksOf(r *vm.Request) []FlowiOverlayMark {
	out := []FlowiOverlayMark{}
	for _, m := range r.OverlayMarks() {
		out = append(out, FlowiOverlayMark{Section: m.Section, Key: m.Key, Overlay: m.Overlay})
	}
	return out
}

func flowiOverlaysOf(r *vm.Request) []FlowiOverlayView {
	out := []FlowiOverlayView{}
	for _, o := range r.Overlays() {
		out = append(out, FlowiOverlayView{Name: o.Name, Description: o.Description, Shared: o.Shared, Own: o.Own, Used: o.Used})
	}
	return out
}

// SelectFlowiOverlay lays the flowi overlay name over the open version; ""
// takes it off. The window shows the effective request.
func (s *RequestService) SelectFlowiOverlay(node, name string) {
	s.with(node, func(o *openRequest) {
		o.req.SelectOverlay(name)
		s.app.send.LoadSaved(o.req)
	})
}

// FlowiOverlayDiff lists what saving the working copy as a flowi overlay
// would store, for the save dialog.
func (s *RequestService) FlowiOverlayDiff(node string) (FlowiOverlayDiffView, error) {
	done := make(chan struct {
		v   FlowiOverlayDiffView
		err error
	}, 1)
	s.with(node, func(o *openRequest) {
		open := o.req.OverlayName()
		if len(o.req.OverlayStack()) > 1 {
			open = "" // a stack is not one overlay to save into: the user names the new one
		}
		o.req.OverlayDiff(func(d flowi.OverlayDiff, err error) {
			v := FlowiOverlayDiffView{Ops: []FlowiOverlayOpView{}, Unsupported: nonNil(d.Unsupported), Names: nonNil(o.req.OverlayNames()), Open: open}
			for i, op := range d.Ops {
				if o.req.OverlayHolds(op) {
					continue // the overlay makes this change already; it is in the working copy because the overlay is laid over it
				}
				v.Ops = append(v.Ops, FlowiOverlayOpView{Index: i, Summary: flowioverlay.Describe(op)})
			}
			done <- struct {
				v   FlowiOverlayDiffView
				err error
			}{v, err}
		})
	})
	r := <-done
	return r.v, r.err
}

// SaveAsFlowiOverlay stores the working copy as the flowi overlay name: the
// changes at the indexes in shared go to the part every request gets, the
// rest to this request's own. It reports true, saving nothing, when the
// overlay's file has comments the rewrite would lose: ask, then call it again
// with dropComments.
func (s *RequestService) SaveAsFlowiOverlay(node, name, description string, shared []int, dropComments bool) (needsConfirm bool, err error) {
	done := make(chan error, 1)
	s.with(node, func(o *openRequest) {
		in := flowi.SaveOverlayInput{Name: name, Description: description, Shared: shared}
		o.req.SaveAsOverlay(in, dropComments, func(err error) { done <- err })
	})
	err = <-done
	if vm.IsDropsComments(err) {
		return true, nil
	}
	if err == nil {
		s.refreshFlowiOverlays()
	}
	return false, err
}

// refreshFlowiOverlays makes every open request read its collection's
// flowi overlays again, after one was saved, edited or deleted.
func (s *RequestService) refreshFlowiOverlays() {
	s.app.actor.Do(func() {
		for _, o := range s.reqs {
			o.req.LoadOverlays()
		}
	})
}

func (s *RequestService) collectionOf(node string) (model.ID, error) {
	var id model.ID
	var err error
	s.with(node, func(o *openRequest) { id = o.req.Target().Collection })
	if id == "" {
		err = fmt.Errorf("%q is not open", node)
	}
	return id, err
}

// FlowiOverlayText is the file of the flowi overlay name in the collection of
// the request node, for the overlay editor. An empty name gives the text a
// new overlay starts with.
func (s *RequestService) FlowiOverlayText(node, name string) (string, error) {
	if name == "" {
		return flowi.NewOverlayText(), nil
	}
	collection, err := s.collectionOf(node)
	if err != nil {
		return "", err
	}
	var text string
	s.app.actor.Do(func() { text, err = s.app.backend.OverlayText(collection, name) })
	return text, err
}

// SaveFlowiOverlayText saves text as the flowi overlay name. It returns the
// problem to show (the file's own diagnostics), "" when saved.
func (s *RequestService) SaveFlowiOverlayText(node, name, text string) string {
	collection, err := s.collectionOf(node)
	if err == nil {
		s.app.actor.Do(func() { err = s.app.backend.WriteOverlayText(collection, name, text) })
	}
	if err != nil {
		return vm.ErrorText(err)
	}
	s.refreshFlowiOverlays()
	return ""
}

// DeleteFlowiOverlay removes the flowi overlay name. Responses kept for it stay.
func (s *RequestService) DeleteFlowiOverlay(node, name string) error {
	collection, err := s.collectionOf(node)
	if err != nil {
		return err
	}
	s.app.actor.Do(func() { err = s.app.backend.DeleteOverlay(collection, name) })
	if err != nil {
		return err
	}
	s.refreshFlowiOverlays()
	return nil
}

// SpecOverlayDocumentView is an OpenAPI Overlay made from a flowi overlay.
type SpecOverlayDocumentView struct {
	Spec     string   `json:"spec"` // the kept spec's name
	File     string   `json:"file"` // the name to save it under
	YAML     string   `json:"yaml"`
	Requests []string `json:"requests"`
}

// SpecExportView is the OpenAPI Overlays a flowi overlay makes, and what they leave out.
type SpecExportView struct {
	Documents  []SpecOverlayDocumentView `json:"documents"`
	NotCarried []string                  `json:"notCarried"` // one sentence each
}

// ExportFlowiOverlay writes the flowi overlay name as OpenAPI Overlays for the
// specs the collection keeps (nothing is saved; see SaveSpecExport).
func (s *RequestService) ExportFlowiOverlay(node, name string) (SpecExportView, error) {
	collection, err := s.collectionOf(node)
	if err != nil {
		return SpecExportView{}, err
	}
	var res flowi.OverlayExport
	s.app.actor.Do(func() { res, err = s.app.backend.ExportOverlayAsSpec(collection, name) })
	if err != nil {
		return SpecExportView{}, err
	}
	return specExportView(name, res), nil
}

func specExportView(name string, res flowi.OverlayExport) SpecExportView {
	out := SpecExportView{Documents: []SpecOverlayDocumentView{}, NotCarried: []string{}}
	for _, d := range res.Documents {
		out.Documents = append(out.Documents, SpecOverlayDocumentView{
			Spec: d.Spec.Name, File: name + "." + d.Spec.Name + ".overlay.yaml", YAML: d.YAML, Requests: nonNil(d.Requests)})
	}
	for _, n := range res.NotCarried {
		where := ""
		if n.Request != "" {
			where = n.Request + ": "
		}
		out.NotCarried = append(out.NotCarried, where+n.What+" ("+n.Why+")")
	}
	return out
}

// SaveSpecExport asks where to save an exported OpenAPI Overlay and writes it
// there. It returns the file, "" when the user cancelled.
func (s *RequestService) SaveSpecExport(file, text string) (string, error) {
	if s.app.pickers.Save == nil {
		return "", errNoDialogs
	}
	path, err := s.app.pickers.Save("Save the OpenAPI Overlay", file)
	if err != nil || path == "" {
		return "", err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return "", fmt.Errorf("saving the overlay: %w", err)
	}
	return path, nil
}
