// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
)

func TestSession_should_keep_the_open_tabs_between_runs(t *testing.T) {
	s := vm.NewSessionStore(t.TempDir())

	saveErr := s.SaveTabs([]string{"r:C1:R1", "f:C1:F1"})
	got, loadErr := s.LoadTabs()

	if saveErr != nil || loadErr != nil || !slices.Equal(got, []string{"r:C1:R1", "f:C1:F1"}) {
		t.Fatalf("save %v load %v tabs %v", saveErr, loadErr, got)
	}
}

func TestSession_Given_nothing_saved_Then_loads_empty_without_error(t *testing.T) {
	s := vm.NewSessionStore(filepath.Join(t.TempDir(), "not-yet"))

	tabs, tabErr := s.LoadTabs()
	lines, logErr := s.LoadLog()
	items, histErr := s.LoadHistory()

	if tabErr != nil || logErr != nil || histErr != nil || len(tabs)+len(lines)+len(items) != 0 {
		t.Fatalf("%v %v %v", tabErr, logErr, histErr)
	}
}

func TestSession_Given_a_damaged_file_Then_reports_it_and_loads_empty(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "tabs.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	tabs, err := vm.NewSessionStore(dir).LoadTabs()

	if err == nil || len(tabs) != 0 {
		t.Fatalf("tabs %v err %v", tabs, err)
	}
}

func TestSession_should_write_privately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "session")
	s := vm.NewSessionStore(dir)

	if err := s.SaveLog([]string{"line"}); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(dir, "log.json"))
	if err != nil {
		t.Fatal(err)
	}
	// Windows has no POSIX modes: it reports 0666 whatever was asked for.
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode().Perm())
	}
}

func TestHistory_Export_and_Restore_should_round_trip_the_exchanges(t *testing.T) {
	h := vm.NewHistory()
	rec := &engine.Record{RequestPath: "auth/login", Status: 200}
	h.Add(flowi.RequestTarget{Collection: "C1", Request: "R1"}, "1", engine.SendResult{Status: engine.StatusPassed, Record: rec})
	s := vm.NewSessionStore(t.TempDir())
	if err := s.SaveHistory(h.Export()); err != nil {
		t.Fatal(err)
	}
	stored, err := s.LoadHistory()
	if err != nil {
		t.Fatal(err)
	}

	again := vm.NewHistory()
	again.Restore(stored)

	items := again.Items()
	if len(items) != 1 || items[0].Title != h.Items()[0].Title || items[0].Result.Record.Status != 200 || items[0].Target.Request != "R1" {
		t.Fatalf("items %+v", items)
	}
	next := again.Add(flowi.RequestTarget{}, "1", engine.SendResult{})
	if next.ID == items[0].ID {
		t.Fatal("a new exchange must not reuse a restored id")
	}
}

func TestRunLog_Restore_should_put_the_old_lines_first(t *testing.T) {
	l := vm.NewRunLog()
	l.Add("new")

	l.Restore([]string{"old1", "old2"})

	if got := l.Lines(); !slices.Equal(got, []string{"old1", "old2", "new"}) {
		t.Fatalf("lines %v", got)
	}
}

func TestSession_DropHistory_should_remove_log_and_history_but_keep_the_tabs(t *testing.T) {
	s := vm.NewSessionStore(t.TempDir())
	_ = s.SaveTabs([]string{"a"})
	_ = s.SaveLog([]string{"l"})
	_ = s.SaveHistory([]vm.StoredExchange{{Title: "h"}})

	dropErr := s.DropHistory()
	lines, _ := s.LoadLog()
	items, _ := s.LoadHistory()
	tabs, _ := s.LoadTabs()

	if dropErr != nil || len(lines) != 0 || len(items) != 0 || len(tabs) != 1 {
		t.Fatalf("drop %v lines %v items %v tabs %v", dropErr, lines, items, tabs)
	}
	if s.DropHistory() != nil {
		t.Fatal("dropping nothing is not an error")
	}
}

func TestSessionWriter_should_write_only_the_latest_of_queued_writes_and_never_in_parallel(t *testing.T) {
	var running, maxRunning atomic.Int32
	var wrote []string
	var mu sync.Mutex
	gate, started := make(chan struct{}), make(chan struct{})
	w := vm.NewSessionWriter(func(error) {})
	// Holds the writer busy. Before: the three writes were queued without waiting for this one to start, so on a
	// loaded machine the writer could pick up "a" before "b" and "c" were queued and the test failed now and then.
	w.Queue("tabs", func() error { close(started); <-gate; return nil })
	<-started
	for _, v := range []string{"a", "b", "c"} {
		w.Queue("tabs", func() error {
			n := running.Add(1)
			maxRunning.Store(max(maxRunning.Load(), n))
			mu.Lock()
			wrote = append(wrote, v)
			mu.Unlock()
			running.Add(-1)
			return nil
		})
	}

	close(gate)
	w.Wait()

	if !slices.Equal(wrote, []string{"c"}) || maxRunning.Load() > 1 {
		t.Fatalf("wrote %v, max parallel %d", wrote, maxRunning.Load())
	}
}

func TestSessionWriter_should_report_a_failed_write(t *testing.T) {
	var got error
	w := vm.NewSessionWriter(func(err error) { got = err })

	w.Queue("log", func() error { return errors.New("disk full") })
	w.Wait()

	if got == nil {
		t.Fatal("the failure must be reported")
	}
}

func TestHistory_should_keep_at_most_a_fixed_number_of_exchanges_when_exporting_and_restoring(t *testing.T) {
	h := vm.NewHistory()
	var stored []vm.StoredExchange
	for i := range vm.MaxStoredHistory + 20 {
		stored = append(stored, vm.StoredExchange{ID: i + 1, Title: "x"})
	}

	h.Restore(stored)

	if len(h.Items()) != vm.MaxStoredHistory || len(h.Export()) != vm.MaxStoredHistory {
		t.Fatalf("items %d exported %d", len(h.Items()), len(h.Export()))
	}
}

// Before: restored ids 1..N could equal the id of an exchange sent while the
// session was still being read, and Open picked the wrong one.
func TestHistory_Restore_Given_a_send_finished_first_Then_ids_stay_unique(t *testing.T) {
	h := vm.NewHistory()
	live := h.Add(flowi.RequestTarget{}, "1", engine.SendResult{})

	h.Restore([]vm.StoredExchange{{ID: 2, Title: "newer"}, {ID: 1, Title: "older"}})

	seen := map[int]bool{}
	for _, ex := range h.Items() {
		if seen[ex.ID] {
			t.Fatalf("duplicate id %d in %+v", ex.ID, h.Items())
		}
		seen[ex.ID] = true
	}
	if len(h.Items()) != 3 || h.Items()[0].ID != live.ID {
		t.Fatalf("items %+v", h.Items())
	}
}

func TestHistory_OnAdd_should_fire_for_a_new_exchange_but_not_for_opening_one(t *testing.T) {
	h := vm.NewHistory()
	added := 0
	h.OnAdd(func() { added++ })
	ex := h.Add(flowi.RequestTarget{}, "1", engine.SendResult{})

	h.Open(ex.ID)

	if added != 1 {
		t.Fatalf("added %d", added)
	}
}
