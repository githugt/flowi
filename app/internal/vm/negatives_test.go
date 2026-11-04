// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func negativeBackend() *vmtest.Backend {
	return &vmtest.Backend{
		Cols: []model.Collection{vmtest.SampleCollection()},
		Negatives: []model.Overlay{
			{Name: "no-email", Description: "email missing"},
			{Name: "stale-token", Description: "an expired token"},
		},
	}
}

func TestWorkspaceVM_SuggestNegatives_should_ask_for_the_request_of_the_row(t *testing.T) {
	b := negativeBackend()
	ws, _ := loadedWorkspace(t, b)

	got, err := ws.SuggestNegatives("r:C1:R1")

	if err != nil || len(got) != 2 || !b.Called("SuggestNegatives R1") {
		t.Fatalf("got %v err %v calls %v", got, err, b.Calls)
	}
}

func TestWorkspaceVM_SuggestNegatives_Given_a_row_that_is_not_a_request_Then_it_says_so(t *testing.T) {
	ws, _ := loadedWorkspace(t, negativeBackend())

	_, err := ws.SuggestNegatives("f:C1:F1")

	if err == nil || !strings.Contains(err.Error(), "request") {
		t.Fatalf("err %v", err)
	}
}

func TestWorkspaceVM_AcceptNegatives_should_save_only_the_ticked_variants(t *testing.T) {
	b := negativeBackend()
	ws, _ := loadedWorkspace(t, b)

	saved, skipped, err := ws.AcceptNegatives("r:C1:R1", []string{"stale-token"})

	if err != nil || len(saved) != 1 || len(skipped) != 0 || !b.Called("AcceptNegatives C1 stale-token") {
		t.Fatalf("saved %v skipped %v err %v calls %v", saved, skipped, err, b.Calls)
	}
}

func TestWorkspaceVM_AcceptNegatives_Given_a_variant_that_is_gone_Then_it_fails_and_saves_nothing(t *testing.T) {
	b := negativeBackend()
	ws, _ := loadedWorkspace(t, b)

	_, _, err := ws.AcceptNegatives("r:C1:R1", []string{"no-such"})

	if err == nil || len(b.CallsStarting("AcceptNegatives")) != 0 {
		t.Fatalf("err %v calls %v", err, b.Calls)
	}
}
