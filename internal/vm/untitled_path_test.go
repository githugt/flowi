// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm/vmtest"
)

// Before: an empty path made nothing (the backend refused it silently).
func TestWorkspaceVM_CreateRequest_Given_no_name_Then_creates_an_untitled_request(t *testing.T) {
	tests := map[string]string{"": "untitled", "  ": "untitled", "auth/": "auth/untitled", "Health": "Health"}
	for given, want := range tests {
		b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
		ws, w := loadedWorkspace(t, b)

		ws.CreateRequest("C1", given, nil, nil)
		w.Wait()

		if !b.Called("NewRequest " + want) {
			t.Errorf("path %q: calls %v, want NewRequest %s", given, b.Calls, want)
		}
	}
}

func TestWorkspaceVM_CreateRequest_Given_untitled_exists_Then_numbers_the_new_one(t *testing.T) {
	col := vmtest.SampleCollection()
	col.Requests = append(col.Requests, model.Request{Meta: model.Meta{ID: "U1", Name: "untitled"}, Path: "untitled"})
	b := &vmtest.Backend{Cols: []model.Collection{col}}
	ws, w := loadedWorkspace(t, b)

	ws.CreateRequest("C1", "", nil, nil)
	w.Wait()

	if !b.Called("NewRequest untitled-2") {
		t.Fatalf("calls %v", b.Calls)
	}
}
