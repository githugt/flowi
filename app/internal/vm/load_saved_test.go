// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// Before: a restarted app showed "not sent" for every request, although the
// last response was saved on disk.
func TestSend_LoadSaved_should_show_the_response_saved_on_disk(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, SavedRecord: &engine.Record{Status: 200, Version: "1", RequestPath: "auth/login"}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	req := vm.NewRequest(b, w, b.Cols[0], b.Cols[0].Requests[0])
	_, before := send.LastFor(req.Target())

	send.LoadSaved(req)
	w.Wait()

	ex, after := send.LastFor(req.Target())
	if before || !after || ex.Result.Record == nil || ex.Result.Record.Status != 200 {
		t.Fatalf("before %v after %v exchange %+v", before, after, ex)
	}
}

func TestSend_LoadSaved_Given_nothing_saved_Then_stays_not_sent(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	req := vm.NewRequest(b, w, b.Cols[0], b.Cols[0].Requests[0])

	send.LoadSaved(req)
	w.Wait()

	if _, ok := send.LastFor(req.Target()); ok {
		t.Fatal("an exchange appeared from nothing")
	}
}
