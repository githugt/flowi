// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func TestProblemsVM_messages_and_targets(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Diags: diag.List{
		diag.New(diag.CodeRefUnknownRequest, diag.Location{File: "/c/requests/auth/login.req.yaml", Pos: diag.Position{Line: 3, Column: 5}}, map[string]string{"id": "X"}),
		diag.New(diag.CodeLintWhitespace, diag.Location{File: "/c/environments/staging.yaml"}, map[string]string{"field": "base_url"}),
		diag.New(diag.CodeYAMLSyntax, diag.Location{File: "/c/flowi.yaml"}, map[string]string{"detail": "bad"}),
	}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	p := vm.NewProblems(b, w, ws)

	p.Refresh()
	w.Wait()

	items := p.Items()
	if len(items) != 3 || items[0].Severity != "error" || !strings.Contains(items[0].Message, "unknown request id X") || items[0].Where != "login.req.yaml:3:5" {
		t.Fatalf("items %+v", items)
	}
	if items[0].Node != "r:C1:R1" || items[1].Node != "e:/c/environments/staging.yaml" || items[2].Node != "" {
		t.Fatalf("targets %q %q %q", items[0].Node, items[1].Node, items[2].Node)
	}
	if p.Summary() != "2 errors, 1 warning" {
		t.Fatalf("summary %q", p.Summary())
	}
}

func TestProblems_should_list_what_the_last_send_of_a_request_reported_and_clear_it_when_it_passes(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	p := vm.NewProblems(b, w, ws)
	p.Refresh()
	w.Wait()
	target := flowi.RequestTarget{Collection: "C1", Request: "R1"}
	failed := diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "status == 200", "message": ""})}

	p.SetRun(target, failed)
	listed := p.Items()
	p.Refresh() // a validation refresh keeps the run problems
	w.Wait()
	kept := len(p.Items())
	p.SetRun(target, nil)

	if len(listed) != 1 || listed[0].Node != "r:C1:R1" || !strings.Contains(listed[0].Message, "status == 200") || kept != 1 || len(p.Items()) != 0 {
		t.Fatalf("listed %+v kept %d after pass %d", listed, kept, len(p.Items()))
	}
}

func TestSendVM_should_report_the_diagnostics_of_each_finished_send(t *testing.T) {
	f := newSendFixture(t)
	f.b.NextSend = &vmtest.Sending{Result: engine.SendResult{Status: engine.StatusFailed,
		Diags: diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "status == 200", "message": ""})}}}
	var got diag.List
	var from flowi.RequestTarget
	f.send.OnResult(func(t flowi.RequestTarget, d diag.List) { from, got = t, d })

	f.send.Send(f.req, vm.SendOptions{})
	f.w.Wait()

	if len(got) != 1 || from != f.req.Target() {
		t.Fatalf("from %+v got %v", from, got)
	}
}

func TestProblems_Summary_should_count_run_problems_too(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	p := vm.NewProblems(b, w, ws)

	p.SetRun(flowi.RequestTarget{Collection: "C1", Request: "R1"}, diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "x", "message": ""})})

	if p.Summary() != "1 error, 0 warnings" {
		t.Fatalf("summary %q", p.Summary())
	}
}

func TestProblems_ClearRun_should_forget_what_sends_reported_and_keep_the_validation_problems(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	p := vm.NewProblems(b, w, ws)
	p.Refresh()
	w.Wait()
	validation := len(p.Items())
	failed := diag.List{diag.New(diag.CodeRunAssertFailed, diag.Location{}, map[string]string{"check": "status == 200", "message": ""})}
	p.SetRun(flowi.RequestTarget{Collection: "C1", Request: "R1"}, failed)

	p.ClearRun()

	if len(p.Items()) != validation {
		t.Fatalf("items %d, want the %d validation problems", len(p.Items()), validation)
	}
}
