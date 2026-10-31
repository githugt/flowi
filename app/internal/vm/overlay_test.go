// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

var errPending = errors.New("not called")

func backendWithOverlays(overlays ...model.Overlay) *vmtest.Backend {
	b := sampleBackend()
	b.OverlayList = overlays
	return b
}

func vipOverlay() model.Overlay {
	return model.Overlay{Name: "vip", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}}}
}

func TestRequestVM_should_show_the_effective_request_when_an_overlay_is_chosen(t *testing.T) {
	// Arrange
	r, w := newRequestVM(t, backendWithOverlays(vipOverlay()))
	r.LoadOverlays()
	w.Wait()

	// Act
	r.SelectOverlay("vip")
	w.Wait()

	// Assert
	cur := r.Current()
	found := false
	for _, h := range cur.Headers {
		found = found || (h.Key == "X-Tier" && h.Value == "vip")
	}
	if !found || r.Dirty() || r.OverlayName() != "vip" {
		t.Errorf("headers %v dirty %v overlay %q", cur.Headers, r.Dirty(), r.OverlayName())
	}
}

func TestRequestVM_should_take_the_overlay_off_again(t *testing.T) {
	// Arrange
	r, w := newRequestVM(t, backendWithOverlays(vipOverlay()))
	r.LoadOverlays()
	r.SelectOverlay("vip")
	w.Wait()

	// Act
	r.SelectOverlay("")
	w.Wait()

	// Assert
	for _, h := range r.Current().Headers {
		if h.Key == "X-Tier" {
			t.Errorf("overlay header still shown: %v", r.Current().Headers)
		}
	}
}

func TestRequestVM_should_refuse_to_choose_an_overlay_over_unsaved_edits(t *testing.T) {
	// Arrange
	r, w := newRequestVM(t, backendWithOverlays(vipOverlay()))
	r.LoadOverlays()
	w.Wait()
	r.Edit(func(v *model.RequestVersion) { v.URL = "x" })

	// Act
	r.SelectOverlay("vip")
	w.Wait()

	// Assert
	if r.OverlayName() != "" || !strings.Contains(r.Err(), "Save or discard") {
		t.Errorf("overlay %q err %q", r.OverlayName(), r.Err())
	}
}

func TestRequestVM_should_note_what_the_overlay_cannot_do(t *testing.T) {
	// Arrange
	stale := model.Overlay{Name: "stale", Shared: []model.OverlayOp{{Verb: model.OverlayRemove, Path: "headers.X-Gone"}}}
	r, w := newRequestVM(t, backendWithOverlays(stale))
	r.LoadOverlays()
	w.Wait()

	// Act
	r.SelectOverlay("stale")
	w.Wait()

	// Assert
	if notes := r.OverlayNotes(); len(notes) != 1 || !strings.Contains(notes[0], "X-Gone") {
		t.Errorf("notes = %v", notes)
	}
}

func TestRequestVM_should_save_the_working_copy_as_an_overlay(t *testing.T) {
	// Arrange
	b := backendWithOverlays()
	r, w := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) {
		v.Headers = append(v.Headers, model.KeyValue{Key: "X-Slow", Value: "1"})
	})
	got := errPending

	// Act
	r.SaveAsOverlay(flowi.SaveOverlayInput{Name: "slow"}, false, func(err error) { got = err })
	w.Wait()

	// Assert
	if got != nil || len(b.SavedOverlays) != 1 || b.SavedOverlays[0].Name != "slow" || b.SavedOverlays[0].Edited.Headers[len(b.SavedOverlays[0].Edited.Headers)-1].Key != "X-Slow" {
		t.Fatalf("err %v saved %+v", got, b.SavedOverlays)
	}
	if r.Dirty() {
		t.Error("the edit now lives in the overlay, so the request is no longer dirty")
	}
}

func TestRequestVM_should_keep_the_overlay_with_the_stored_edit(t *testing.T) {
	// Arrange
	b := backendWithOverlays(vipOverlay())
	r, w := newRequestVM(t, b)
	r.LoadOverlays()
	r.SelectOverlay("vip")
	w.Wait()
	r.Edit(func(v *model.RequestVersion) { v.URL += "?slow=1" })

	// Act
	r.FlushResult(func(bool) {})
	w.Wait()

	// Assert
	if len(b.Edits) != 1 || b.Edits[0].Overlay != "vip" {
		t.Fatalf("stored edits = %+v", b.Edits)
	}
}

func TestRequestVM_should_choose_the_overlay_again_with_a_restored_edit(t *testing.T) {
	// Arrange
	b := backendWithOverlays(vipOverlay())
	req := b.Cols[0].Requests[0]
	edit := req.Versions[0]
	edit.URL = "http://elsewhere"
	req.Edit, req.EditBase, req.EditOverlay = &edit, req.Versions[0].Label, "vip"
	b.Cols[0].Requests[0] = req

	// Act
	r, _ := newRequestVM(t, b)

	// Assert
	if r.OverlayName() != "vip" || !r.Dirty() || r.Current().URL != "http://elsewhere" {
		t.Errorf("overlay %q dirty %v url %q", r.OverlayName(), r.Dirty(), r.Current().URL)
	}
}

func TestRequestVM_should_stack_overlays_and_drop_a_deleted_one_from_the_stack(t *testing.T) {
	// Arrange
	slow := model.Overlay{Name: "slow", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "slow"}}}
	b := backendWithOverlays(vipOverlay(), slow)
	r, w := newRequestVM(t, b)
	r.LoadOverlays()
	w.Wait()

	// Act
	r.SelectOverlay("vip+slow")
	w.Wait()
	stacked := r.OverlayStack()
	tier := ""
	for _, h := range r.Current().Headers {
		if h.Key == "X-Tier" {
			tier = h.Value
		}
	}
	b.OverlayList = []model.Overlay{vipOverlay()}
	r.LoadOverlays()
	w.Wait()

	// Assert
	if len(stacked) != 2 || tier != "slow" {
		t.Errorf("stack %v, X-Tier %q: the later overlay should win", stacked, tier)
	}
	if r.OverlayName() != "vip" {
		t.Errorf("after slow was deleted the stack is %q", r.OverlayName())
	}
}

func TestRequestVM_should_mark_the_rows_a_stack_of_overlays_sets_with_the_later_overlay_winning(t *testing.T) {
	// Arrange
	slow := model.Overlay{Name: "slow", Shared: []model.OverlayOp{
		{Verb: model.OverlaySet, Path: "headers.x-tier", Value: "slow"},
		{Verb: model.OverlaySet, Path: "query.page", Value: "9"},
		{Verb: model.OverlayAdd, Path: "assert", Value: "status == 200"},
	}}
	r, w := newRequestVM(t, backendWithOverlays(vipOverlay(), slow))
	r.LoadOverlays()
	w.Wait()

	// Act
	r.SelectOverlay("vip+slow")
	w.Wait()
	marks := r.OverlayMarks()

	// Assert
	if len(marks) != 2 {
		t.Fatalf("marks %+v: one for the header (set by both, the later wins) and one for the query row", marks)
	}
	if marks[0].Section != "query" && marks[1].Section != "query" {
		t.Errorf("marks %+v", marks)
	}
	for _, m := range marks {
		if m.Overlay != "slow" {
			t.Errorf("%+v should name the overlay that wins", m)
		}
	}
}
