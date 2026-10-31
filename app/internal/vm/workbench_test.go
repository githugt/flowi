// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestRequestVM_Configured_should_not_count_the_authorization_header_as_a_header(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.SetAuth(vm.AuthBearer, "x")

	if r.Configured(vm.SectionHeaders) || !r.Configured(vm.SectionAuth) {
		t.Fatalf("headers %v auth %v", r.Configured(vm.SectionHeaders), r.Configured(vm.SectionAuth))
	}
}

func TestRunLog_Sections_should_group_lines_under_their_send(t *testing.T) {
	log := vm.NewRunLog()
	log.Add("stray")
	log.Add("── 12:00:01 send 1")
	log.Add("→ GET http://x")
	log.Add("← 200 (3ms)")
	log.Add("── 12:00:05 send 1")
	log.Add("→ GET http://y")

	got := log.Sections()

	if len(got) != 3 || got[0].Title != "" || len(got[0].Lines) != 1 || got[1].Title != "── 12:00:01 send 1" || len(got[1].Lines) != 2 || len(got[2].Lines) != 1 {
		t.Fatalf("sections %+v", got)
	}
}

func TestLineTone_should_colour_log_lines_by_what_they_say(t *testing.T) {
	tests := map[string]vm.Tone{
		"→ GET http://x":        vm.TonePending,
		"← 200 (3ms)":           vm.ToneSuccess,
		"  ← 404 (3ms)":         vm.ToneClientError,
		"← 503 (3ms)":           vm.ToneServerError,
		"← ✗ timeout (1s)":      vm.ToneServerError,
		"✗ vault is locked":     vm.ToneServerError,
		"⚠ something":           vm.ToneClientError,
		"cancelled":             vm.ToneCancelled,
		"log: hello":            vm.ToneNone,
		"passed (12ms)":         vm.ToneSuccess,
		"· node (request) done": vm.ToneNone,
	}
	for line, want := range tests {
		if got := vm.LineTone(line); got != want {
			t.Errorf("%q: got %v want %v", line, got, want)
		}
	}
}

func TestWorkspaceVM_Display_should_prefer_a_label_set_for_the_node(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	changes := 0
	ws.OnLabelChange(func() { changes++ })

	before := ws.Display("r:S1:SR1")
	ws.SetLabel("r:S1:SR1", "pokeapi.co/api/v2/x")
	ws.SetLabel("r:S1:SR1", "pokeapi.co/api/v2/x") // unchanged: no redraw
	after := ws.Display("r:S1:SR1")
	ws.SetLabel("r:S1:SR1", "")

	if before != "untitled" || after != "pokeapi.co/api/v2/x" || ws.Display("r:S1:SR1") != "untitled" || changes != 2 {
		t.Fatalf("before %q after %q reset %q changes %d", before, after, ws.Display("r:S1:SR1"), changes)
	}
}

func TestHistory_Add_should_name_an_untitled_request_by_what_was_sent(t *testing.T) {
	h := vm.NewHistory()
	rec := &engine.Record{RequestPath: "untitled-6", Status: 403, Request: engine.SentRequest{Method: "POST", URL: "https://pokeapi.co/api/v2/pokemon/ditto"}}

	ex := h.Add(flowi.RequestTarget{}, "1", engine.SendResult{Status: engine.StatusFailed, Record: rec})
	named := h.Add(flowi.RequestTarget{}, "1", engine.SendResult{Status: engine.StatusPassed, Record: &engine.Record{RequestPath: "auth/login", Status: 200}})

	if ex.Title != "POST pokeapi.co/api/v2/pokemon/ditto v1 · 403 · failed" || named.Title != "auth/login v1 · 200 · passed" {
		t.Fatalf("titles %q / %q", ex.Title, named.Title)
	}
}

func TestRunLog_Entries_should_split_a_line_into_time_level_step_and_message(t *testing.T) {
	log := vm.NewRunLog()
	log.Add("── 12:00:01 send 1")
	log.Add("→ GET http://x")
	log.Add("← 503 (3ms)")
	log.Add("⚠ slow")
	log.Add("log: hello")
	before := time.Now()

	entries := log.Sections()[0].Entries

	want := []struct{ level, step, message string }{
		{"INFO", "Request", "→ GET http://x"}, {"ERROR", "Response", "← 503 (3ms)"}, {"WARN", "-", "⚠ slow"}, {"INFO", "-", "log: hello"},
	}
	if len(entries) != len(want) {
		t.Fatalf("entries %+v", entries)
	}
	for i, w := range want {
		e := entries[i]
		if e.Level != w.level || e.Step != w.step || e.Message != w.message || e.At.IsZero() || e.At.After(before) {
			t.Errorf("entry %d = %+v, want %+v with a time", i, e, w)
		}
	}
}

func TestRunLog_Entries_should_leave_the_time_blank_for_lines_restored_from_an_earlier_session(t *testing.T) {
	log := vm.NewRunLog()
	log.Restore([]string{"→ GET http://old"})
	log.Add("→ GET http://new")

	entries := log.Sections()[0].Entries

	if !entries[0].At.IsZero() || entries[1].At.IsZero() {
		t.Fatalf("times %v %v, want blank then set", entries[0].At, entries[1].At)
	}
}
