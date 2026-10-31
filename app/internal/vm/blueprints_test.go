// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm/vmtest"
)

func collectionWithBlueprint() model.Collection {
	c := vmtest.SampleCollection()
	c.Blueprints = []model.Blueprint{{
		Meta: model.Meta{ID: "BP1", Name: "Two calls", Description: "Chains two requests"}, Path: "two-calls",
		Params: []model.Param{{Name: "first", Type: model.ParamRequestRef, Default: "auth/login"}},
	}}
	return c
}

func TestWorkspaceVM_Blueprints_should_list_every_blueprint_with_its_collection_and_parameters(t *testing.T) {
	ws, _ := loadedWorkspace(t, &vmtest.Backend{Cols: []model.Collection{collectionWithBlueprint()}})

	got := ws.Blueprints()

	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	bp := got[0]
	if bp.Label() != "Two calls (Shop)" || bp.ID != "BP1" || bp.Collection != "C1" || len(bp.Params) != 1 || bp.Params[0].Default != "auth/login" {
		t.Fatalf("blueprint %+v (label %q)", bp, bp.Label())
	}
}

func TestWorkspaceVM_CreateFlowFromBlueprint_should_create_reload_and_open_the_flow(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{collectionWithBlueprint()}}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateFlowFromBlueprint(ws.Blueprints()[0], "C1", "chain", map[string]string{"first": "auth/verify"}, func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("NewFlowFromBlueprint BP1 chain first=auth/verify") || opened != "f:C1:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateFlowFromBlueprint_Given_no_collection_Then_uses_Scratch(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{collectionWithBlueprint()}}
	ws, w := loadedWorkspace(t, b)
	var opened string

	ws.CreateFlowFromBlueprint(ws.Blueprints()[0], "", "chain", nil, func(id string) { opened = id }, nil)
	w.Wait()

	if !b.Called("Scratch") || opened != "f:SCRATCH:new" {
		t.Fatalf("calls %v, opened %q", b.Calls, opened)
	}
}

func TestWorkspaceVM_CreateFlowFromBlueprint_Given_a_refusal_Then_reports_it_and_opens_nothing(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{collectionWithBlueprint()}, NewErr: errBoom}
	ws, w := loadedWorkspace(t, b)
	var got error

	ws.CreateFlowFromBlueprint(ws.Blueprints()[0], "C1", "chain", nil, func(string) { t.Fatal("opened") }, func(err error) { got = err })
	w.Wait()

	if got == nil {
		t.Fatal("no error")
	}
}

func TestWorkspaceVM_SaveFlowAsBlueprint_should_create_it_and_refresh_the_list(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	ws, w := loadedWorkspace(t, b)
	done := false

	ws.SaveFlowAsBlueprint(flowi.FlowTarget{Collection: "C1", Flow: "F1"}, "tpl", "Template", func() { done = true }, nil)
	w.Wait()

	if !done || !b.Called("CreateBlueprint F1 tpl Template") {
		t.Fatalf("done %v calls %v", done, b.Calls)
	}
}

func TestWorkspaceVM_SaveFlowAsBlueprint_Given_a_refusal_Then_reports_it(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, NewErr: errBoom}
	ws, w := loadedWorkspace(t, b)
	var got error

	ws.SaveFlowAsBlueprint(flowi.FlowTarget{Collection: "C1", Flow: "F1"}, "tpl", "Template", func() { t.Fatal("done") }, func(err error) { got = err })
	w.Wait()

	if got == nil {
		t.Fatal("no error")
	}
}
