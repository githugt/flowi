// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func suggestingBackend() *vmtest.Backend {
	return &vmtest.Backend{
		Cols:        []model.Collection{vmtest.SampleCollection()},
		Suggestions: map[model.ID][]flowi.FlowSuggestion{"C1": {{Name: "pets", Path: "suggested/pets"}}},
	}
}

func TestWorkspaceVM_AcceptSuggestion_should_save_the_suggestion_and_open_the_flow(t *testing.T) {
	b := suggestingBackend()
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.AcceptSuggestion("C1", "suggested/pets", func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("AcceptSuggestion suggested/pets") || opened != "f:C1:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_AcceptSuggestion_Given_a_suggestion_that_is_gone_Then_fails_and_saves_nothing(t *testing.T) {
	b := suggestingBackend()
	ws, w := loadedWorkspace(t, b)
	var failed error

	ws.AcceptSuggestion("C1", "suggested/cats", func(string) { t.Error("opened") }, func(err error) { failed = err })
	w.Wait()

	if failed == nil || b.Called("AcceptSuggestion suggested/cats") {
		t.Fatalf("failed %v, calls %v", failed, b.Calls)
	}
}
