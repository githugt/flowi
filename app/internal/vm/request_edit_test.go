// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// timerQueue stands in for the UI goroutine: timer callbacks are queued and
// the test runs them on its own goroutine (as the app's poster does), so
// there is no data race between the timer goroutine and the test.
type timerQueue struct{ ch chan func() }

func newTimerQueue() timerQueue { return timerQueue{ch: make(chan func(), 16)} }

func (q timerQueue) post(f func()) { q.ch <- f }

// drain runs what the timers have posted so far.
func (q timerQueue) drain() {
	for {
		select {
		case f := <-q.ch:
			f()
		default:
			return
		}
	}
}

// newAutosavingRequest is a request VM whose autosave timer runs on the
// (simulated) clock of the test's synctest bubble.
func newAutosavingRequest(t *testing.T, b *vmtest.Backend) (*vm.Request, timerQueue) {
	t.Helper()
	r, _ := newRequestVM(t, b)
	q := newTimerQueue()
	r.SetScheduler(vm.NewTimerScheduler(q.post))
	return r, q
}

func storedEdits(b *vmtest.Backend) []vmtest.StoredEdit {
	b.Mu.Lock()
	defer b.Mu.Unlock()
	return slices.Clone(b.Edits)
}

func TestRequestVM_Given_edit_Then_autosaves_to_the_slot_10_seconds_after_the_last_change(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := sampleBackend()
		r, ui := newAutosavingRequest(t, b)

		r.Edit(func(v *model.RequestVersion) { v.URL = "http://one" })
		time.Sleep(9 * time.Second)
		ui.drain()
		before := len(storedEdits(b))
		r.Edit(func(v *model.RequestVersion) { v.URL = "http://two" })
		time.Sleep(9 * time.Second)
		ui.drain()
		stillDebounced := len(storedEdits(b))
		time.Sleep(2 * time.Second)
		ui.drain()

		got := storedEdits(b)
		if before != 0 || stillDebounced != 0 || len(got) != 1 || got[0].Base != "1" || got[0].Version.URL != "http://two" || !r.Dirty() {
			t.Fatalf("before %d debounced %d stored %+v", before, stillDebounced, got)
		}
	})
}

func TestRequestVM_Flush_should_store_the_edit_at_once_and_cancel_the_timer(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := sampleBackend()
		r, ui := newAutosavingRequest(t, b)
		r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

		r.Flush(nil)
		time.Sleep(30 * time.Second)
		ui.drain()

		if got := storedEdits(b); len(got) != 1 || got[0].Version.URL != "http://x" {
			t.Fatalf("stored %+v", got)
		}
	})
}

func TestRequestVM_Given_a_structural_change_Then_autosave_is_skipped(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := sampleBackend()
		r, ui := newAutosavingRequest(t, b)
		_ = r.NewVersionFromCurrent("2") // a new version cannot live in the one-version slot

		r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
		time.Sleep(30 * time.Second)
		ui.drain()

		if got := storedEdits(b); len(got) != 0 {
			t.Fatalf("stored %+v", got)
		}
	})
}

func TestRequestVM_Given_a_slot_on_disk_Then_opens_dirty_showing_the_edit(t *testing.T) {
	b := sampleBackend()
	edit := model.RequestVersion{Method: "PUT", URL: "http://from-slot"}
	b.Cols[0].Requests[0].Edit, b.Cols[0].Requests[0].EditBase = &edit, "1"

	r, _ := newRequestVM(t, b)

	if !r.Dirty() || r.Current().URL != "http://from-slot" || r.Current().Method != "PUT" || r.SelectedLabel() != "1" || r.Draft().Edit != nil {
		t.Fatalf("dirty %v current %+v draft %+v", r.Dirty(), r.Current(), r.Draft())
	}
}

func TestRequestVM_DiscardEdit_should_clear_the_slot_and_show_the_saved_version(t *testing.T) {
	b := sampleBackend()
	r, _ := newAutosavingRequest(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	r.Flush(nil)

	r.DiscardEdit()

	if r.Dirty() || r.Current().URL != "{{env.base_url}}/login" || !b.Called("DiscardEdit") || b.Cols[0].Requests[0].Edit != nil {
		t.Fatalf("dirty %v current %+v", r.Dirty(), r.Current())
	}
}

func TestRequestVM_Save_should_write_every_version_and_leave_no_slot(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://saved" })

	r.Save(false)
	w.Wait()

	if r.Dirty() || len(b.Saved) != 1 || b.Saved[0].Edit != nil || b.Saved[0].Versions[0].URL != "http://saved" {
		t.Fatalf("saved %+v", b.Saved)
	}
}

func TestRequestVM_SelectVersion_Given_an_edit_Then_refuses_and_says_why(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Requests[0].Versions = append(b.Cols[0].Requests[0].Versions, model.RequestVersion{Label: "2", Method: "GET", URL: "u2"})
	b.Cols[0].Requests[0].Default = "1"
	r, _ := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

	r.SelectVersion("2")

	if r.SelectedLabel() != "1" || !strings.Contains(r.Err(), "Save or discard") {
		t.Fatalf("selected %q err %q", r.SelectedLabel(), r.Err())
	}
}

func TestRequestVM_NewVersionFromCurrent_Given_an_edit_Then_the_base_keeps_its_saved_content(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://edited" })

	err := r.NewVersionFromCurrent("2")
	r.Save(false)
	w.Wait()

	saved := b.Saved[0].Versions
	if err != nil || len(saved) != 2 || saved[0].URL != "{{env.base_url}}/login" || saved[1].Label != "2" || saved[1].URL != "http://edited" {
		t.Fatalf("err %v saved %+v", err, saved)
	}
}

func TestRequestVM_Reloaded_Given_only_the_slot_changed_Then_no_changed_on_disk_banner(t *testing.T) {
	b := sampleBackend()
	r, _ := newAutosavingRequest(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	r.Flush(nil)
	c := b.Cols[0]

	r.Reloaded(c, c.Requests[0], true) // what the file watcher reports after our own autosave

	if r.ChangedOnDisk() || !r.Dirty() || r.Current().URL != "http://x" {
		t.Fatalf("banner %v dirty %v current %+v", r.ChangedOnDisk(), r.Dirty(), r.Current())
	}
}

func TestRequestVM_Given_autosave_refused_for_comments_Then_says_so_without_blocking_edits(t *testing.T) {
	b := sampleBackend()
	b.EditErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "login.req.yaml"})}.Err()
	r, _ := newAutosavingRequest(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

	r.Flush(nil)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://y" })

	if !strings.Contains(r.AutosaveProblem(), "comments") || r.Current().URL != "http://y" {
		t.Fatalf("problem %q current %+v", r.AutosaveProblem(), r.Current())
	}
}

func TestSend_Given_an_edit_Then_stores_it_and_sends_the_edit(t *testing.T) {
	b := sampleBackend()
	b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusPassed}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	c := b.Cols[0]
	r := vm.NewRequest(b, w, c, c.Requests[0])
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://edited" })

	send.Send(r, vm.SendOptions{Hooks: vm.HooksAll})
	w.Wait()

	if got := storedEdits(b); len(got) != 1 || got[0].Version.URL != "http://edited" {
		t.Fatalf("stored %+v", got)
	}
	if len(b.SendOpts) != 1 || !b.SendOpts[0].UseEdit || b.SendOpts[0].Version != "1" || len(b.Saved) != 0 {
		t.Fatalf("send opts %+v saved %d", b.SendOpts, len(b.Saved))
	}
}

// Review finding 1: a "stored" report for changes that were never written
// made quitting (and "Keep edit") lose a new version silently.
func TestRequestVM_FlushResult_Given_changes_that_do_not_fit_the_slot_Then_reports_not_stored(t *testing.T) {
	b := sampleBackend()
	r, _ := newRequestVM(t, b)
	_ = r.NewVersionFromCurrent("2")
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	stored := true

	r.FlushResult(func(ok bool) { stored = ok })

	if stored || r.AutosaveProblem() == "" || len(storedEdits(b)) != 0 {
		t.Fatalf("stored %v problem %q", stored, r.AutosaveProblem())
	}
}

// Review finding 2: Send while a Save was still writing used the old state.
func TestRequestVM_PrepareSend_Given_a_save_in_flight_Then_waits_for_it(t *testing.T) {
	b := sampleBackend()
	q := newTimerQueue()
	w := vm.NewWorker(q.post)
	c := b.Cols[0]
	r := vm.NewRequest(b, w, c, c.Requests[0])
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	r.Save(false) // queued on the worker, not finished
	sent, useEdit := false, false

	r.PrepareSend(func(u bool) { sent, useEdit = true, u })
	immediately := sent
	q.drainAll(w)

	if immediately || !sent || useEdit {
		t.Fatalf("immediately %v sent %v useEdit %v (the save covered the edit, so no slot is used)", immediately, sent, useEdit)
	}
}

// Review finding 3: a Discard while an autosave was in flight left the edit
// in the file.
func TestRequestVM_DiscardEdit_Given_a_store_in_flight_Then_still_clears_the_slot(t *testing.T) {
	b := sampleBackend()
	q := newTimerQueue()
	w := vm.NewWorker(q.post)
	c := b.Cols[0]
	r := vm.NewRequest(b, w, c, c.Requests[0])
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	r.Flush(nil) // StoreEdit queued
	r.DiscardEdit()
	q.drainAll(w)

	b.Mu.Lock()
	slot := b.Cols[0].Requests[0].Edit
	b.Mu.Unlock()
	if slot != nil || r.Dirty() {
		t.Fatalf("slot %+v dirty %v", slot, r.Dirty())
	}
}

// Review finding 4: Send did nothing for an edited request in a commented
// file; it must fall back to saving (which asks to drop the comments).
func TestSendVM_Given_a_commented_file_Then_falls_back_to_saving_and_asks(t *testing.T) {
	f := newSendFixture(t)
	f.b.EditErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "x"})}.Err()
	f.b.SaveErr = f.b.EditErr
	asked := 0
	f.req.OnDropConfirm(func() { asked++ })
	f.req.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	if asked != 1 || f.b.Called("Send") {
		t.Fatalf("asked %d sent %v", asked, f.b.Called("Send"))
	}
}

// Review finding 9: an edit whose base version is gone must never be
// overwritten without the user knowing.
func TestRequestVM_Given_an_orphan_slot_Then_autosave_does_not_overwrite_it_and_says_so(t *testing.T) {
	b := sampleBackend()
	edit := model.RequestVersion{Method: "PUT", URL: "http://old"}
	b.Cols[0].Requests[0].Edit, b.Cols[0].Requests[0].EditBase = &edit, "7"
	r, _ := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) { v.URL = "http://new" })

	r.Flush(nil)

	if len(storedEdits(b)) != 0 || !strings.Contains(r.AutosaveProblem(), "7") {
		t.Fatalf("stored %+v problem %q", storedEdits(b), r.AutosaveProblem())
	}
}

// Review finding 11: "1.0" equals "1" for the file format, so it is taken.
func TestRequestVM_NewVersionFromCurrent_should_refuse_labels_equal_to_an_existing_one(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	err := r.NewVersionFromCurrent("1.0")

	if err == nil {
		t.Fatal("1.0 equals 1")
	}
}

// drainAll runs every function the worker posts, until it is quiet: each
// result can start the next queued write.
func (q timerQueue) drainAll(w *vm.Worker) {
	for {
		w.Wait()
		if len(q.ch) == 0 {
			return
		}
		q.drain()
	}
}

func blankRequestBackend() *vmtest.Backend {
	c := vmtest.SampleCollection()
	c.Requests = []model.Request{{Meta: model.Meta{ID: "RB", Name: "untitled"}, Path: "untitled", Default: "1", Versions: []model.RequestVersion{{Label: "1", Method: "GET"}}}}
	return &vmtest.Backend{Cols: []model.Collection{c}}
}

// There is nothing to go back to on a request that was never filled in, so
// the discard button would only confuse.
func TestRequestVM_CanDiscardEdit_should_be_false_for_an_edit_of_an_empty_request(t *testing.T) {
	blank, _ := newRequestVM(t, blankRequestBackend())
	filled, _ := newRequestVM(t, sampleBackend())

	blank.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })
	filled.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

	if blank.CanDiscardEdit() || !filled.CanDiscardEdit() {
		t.Fatalf("blank %v filled %v", blank.CanDiscardEdit(), filled.CanDiscardEdit())
	}
}

func TestRequestVM_CanDiscardEdit_should_be_false_without_an_edit(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	if r.CanDiscardEdit() {
		t.Fatal("no edit, nothing to discard")
	}
}

// Before: closing a tab left its autosave timer running, so a request that
// was deleted meanwhile was written back to disk.
func TestRequestVM_Abandon_should_stop_the_autosave(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := sampleBackend()
		r, ui := newAutosavingRequest(t, b)
		r.Edit(func(v *model.RequestVersion) { v.URL = "http://x" })

		r.Abandon()
		time.Sleep(30 * time.Second)
		ui.drain()
		stored := false
		r.FlushResult(func(ok bool) { stored = ok })

		if got := storedEdits(b); len(got) != 0 || !stored {
			t.Fatalf("stored %+v, flush result %v", got, stored)
		}
	})
}

func TestRequestVM_AvailableSettings_should_list_what_a_request_may_set_with_defaults(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	got := r.AvailableSettings()

	byName := map[string]vm.AvailableSetting{}
	for _, s := range got {
		byName[s.Key] = s
	}
	timeout, ok := byName["timeout"]
	if !ok || timeout.Default != "30s" || timeout.Description == "" || timeout.Set {
		t.Fatalf("timeout %+v (listed %v)", timeout, ok)
	}
	if _, listed := byName["logging.level"]; listed {
		t.Fatal("a setting that cannot live in a request file must not be offered")
	}
}

func TestRequestVM_AddSetting_should_add_a_row_with_the_default_and_mark_it_set(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.AddSetting("timeout")

	rows := r.SettingRows()
	var set bool
	for _, s := range r.AvailableSettings() {
		if s.Key == "timeout" {
			set = s.Set
		}
	}
	if len(rows) != 1 || rows[0].Key != "timeout" || rows[0].Raw != "30s" || !set {
		t.Fatalf("rows %+v set %v", rows, set)
	}
}

// Before: Browse stored the absolute path, which a send refuses (files must
// be inside the collection).
func TestRequestVM_RelativeToRoot_should_name_files_inside_the_collection_only(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend()) // root /c

	inside, okIn := r.RelativeToRoot("/c/data/up.csv")
	_, okOut := r.RelativeToRoot("/elsewhere/up.csv")
	_, okParent := r.RelativeToRoot("/c/../x/up.csv")

	if inside != "data/up.csv" || !okIn || okOut || okParent {
		t.Fatalf("inside %q %v, outside %v, parent %v", inside, okIn, okOut, okParent)
	}
}
