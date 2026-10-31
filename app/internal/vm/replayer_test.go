// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/serve/replay"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

// uiQueue stands in for the UI goroutine: posted functions wait until the
// test runs them, so the test never races the vm's own goroutines.
type uiQueue struct{ posted chan func() }

func newUIQueue() (*uiQueue, *vm.Worker) {
	q := &uiQueue{posted: make(chan func(), 64)}
	return q, vm.NewWorker(func(f func()) { q.posted <- f })
}

// run executes n posted functions (blocking until they arrive).
func (q *uiQueue) run(n int) {
	for range n {
		(<-q.posted)()
	}
}

func startReplayer(t *testing.T, b *vmtest.Backend) (*vm.Replayer, *uiQueue) {
	t.Helper()
	q, w := newUIQueue()
	r := vm.NewReplayer(b, w)
	t.Cleanup(func() {
		closed := make(chan struct{})
		go func() { r.Close(); close(closed) }()
		for { // keep acting as the UI goroutine until the replayer has finished posting
			select {
			case f := <-q.posted:
				f()
			case <-closed:
				return
			}
		}
	})
	return r, q
}

func TestReplayer_Start_should_serve_only_the_replayer_with_the_form_values(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	r.Start(vm.ReplayerForm{Addr: "127.0.0.1:9000", Latency: "250ms", Sequence: true, CORS: true})
	if r.State() != vm.ReplayerStarting {
		t.Fatalf("state %v", r.State())
	}
	q.run(1)

	if r.State() != vm.ReplayerRunning || r.Addr() != "127.0.0.1:9000" || len(r.Notes()) != 1 || r.Err() != "" {
		t.Fatalf("state %v addr %q notes %v err %q", r.State(), r.Addr(), r.Notes(), r.Err())
	}
	o := b.ServedWith[0]
	if o.ReplayAddr != "127.0.0.1:9000" || o.Latency.String() != "250ms" || !o.Sequence || !o.CORS || !o.NoTriggers || !o.NoSchedules || o.NoReplay {
		t.Fatalf("served with %+v", o)
	}
}

func TestReplayer_Start_Given_a_bad_latency_Then_says_so_and_does_not_start(t *testing.T) {
	b := &vmtest.Backend{}
	r, _ := startReplayer(t, b)

	r.Start(vm.ReplayerForm{Latency: "soon"})

	if r.State() != vm.ReplayerStopped || r.Err() == "" || len(b.ServedWith) != 0 {
		t.Fatalf("state %v err %q served %d", r.State(), r.Err(), len(b.ServedWith))
	}
}

func TestReplayer_Start_Given_the_server_cannot_start_Then_shows_why(t *testing.T) {
	b := &vmtest.Backend{ServeErr: errors.New("replayer: listen tcp 127.0.0.1:8525: address already in use")}
	r, q := startReplayer(t, b)

	r.Start(vm.ReplayerForm{})
	q.run(1)

	if r.State() != vm.ReplayerStopped || r.Err() == "" {
		t.Fatalf("state %v err %q", r.State(), r.Err())
	}
}

func TestReplayer_should_collect_traffic_newest_last_and_clear_it(t *testing.T) {
	log := replay.NewLog(10)
	b := &vmtest.Backend{ServeLog: log}
	r, q := startReplayer(t, b)
	r.Start(vm.ReplayerForm{})
	q.run(1)

	log.Add(replay.Entry{Method: "GET", Path: "/a", Status: 200})
	log.Add(replay.Entry{Method: "POST", Path: "/b", Status: 404})
	q.run(2)
	got := r.Entries()
	r.Clear()

	if len(got) != 2 || got[0].Path != "/a" || got[1].Status != 404 {
		t.Fatalf("entries %+v", got)
	}
	if len(r.Entries()) != 0 {
		t.Fatal("Clear left entries")
	}
}

func TestReplayer_should_keep_only_the_newest_entries(t *testing.T) {
	log := replay.NewLog(10000)
	b := &vmtest.Backend{ServeLog: log}
	r, q := startReplayer(t, b)
	r.Start(vm.ReplayerForm{})
	q.run(1)

	// A subscriber's buffer is small and Add never blocks, so feed it in
	// batches the UI queue can take.
	const total, batch = vm.MaxReplayerEntries + 5, 50
	for sent := 0; sent < total; {
		n := min(batch, total-sent)
		for i := range n {
			log.Add(replay.Entry{Seq: sent + i, Path: "/x"})
		}
		q.run(n)
		sent += n
	}

	got := r.Entries()
	if len(got) != vm.MaxReplayerEntries || got[len(got)-1].Seq != vm.MaxReplayerEntries+5 {
		t.Fatalf("%d entries, last seq %d", len(got), got[len(got)-1].Seq)
	}
}

func TestReplayer_Stop_should_drain_then_report_stopped(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)
	r.Start(vm.ReplayerForm{})
	q.run(1)

	r.Stop()
	stopping := r.State()
	q.run(1)

	if stopping != vm.ReplayerStopping || r.State() != vm.ReplayerStopped || r.Err() != "" {
		t.Fatalf("stopping %v then %v err %q", stopping, r.State(), r.Err())
	}
}

func TestReplayer_should_report_a_forced_stop_only_as_a_note_not_an_error(t *testing.T) {
	b := &vmtest.Backend{ServeWait: flowi.ErrShutdownTimeout}
	r, q := startReplayer(t, b)
	r.Start(vm.ReplayerForm{})
	q.run(1)

	r.Stop()
	q.run(1)

	if r.State() != vm.ReplayerStopped || r.Err() != "" {
		t.Fatalf("state %v err %q", r.State(), r.Err())
	}
}

func TestReplayer_should_show_a_server_that_died_on_its_own(t *testing.T) {
	b := &vmtest.Backend{ServeWait: errors.New("replayer: accept failed")}
	r, q := startReplayer(t, b)
	r.Start(vm.ReplayerForm{})
	q.run(1)

	r.Stop()
	q.run(1)

	if r.Err() == "" {
		t.Fatal("a failure while serving was hidden")
	}
}

func TestReplayer_LoadDefaults_should_fill_the_form_from_settings(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	r.LoadDefaults()
	q.run(1)

	if f := r.Form(); f.Addr != "127.0.0.1:8525" || f.Latency != "" {
		t.Fatalf("form %+v", f)
	}
}

func TestReplayer_Start_twice_should_start_once(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	r.Start(vm.ReplayerForm{})
	r.Start(vm.ReplayerForm{})
	q.run(1)

	if len(b.ServedWith) != 1 {
		t.Fatalf("served %d times", len(b.ServedWith))
	}
}

func TestReplayer_Start_should_record_from_the_upstream_in_the_form(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	r.Start(vm.ReplayerForm{Addr: "127.0.0.1:9000", Record: "  https://api.example.com/v1 "})
	q.run(1)

	if r.State() != vm.ReplayerRunning || b.ServedWith[0].Record != "https://api.example.com/v1" {
		t.Fatalf("state %v, served with %+v", r.State(), b.ServedWith)
	}
}

func TestReplayer_Start_Given_an_upstream_that_is_not_a_web_address_Then_says_so_and_does_not_start(t *testing.T) {
	for _, bad := range []string{"api.example.com", "ftp://api.example.com", "https://"} {
		b := &vmtest.Backend{}
		r, _ := startReplayer(t, b)

		r.Start(vm.ReplayerForm{Record: bad})

		if r.State() != vm.ReplayerStopped || r.Err() == "" || len(b.ServedWith) != 0 {
			t.Fatalf("%q: state %v err %q served %d", bad, r.State(), r.Err(), len(b.ServedWith))
		}
	}
}

func TestReplayer_Start_Given_CORS_and_an_upstream_Then_refuses_before_starting(t *testing.T) {
	b := &vmtest.Backend{}
	r, _ := startReplayer(t, b)

	r.Start(vm.ReplayerForm{CORS: true, Record: "https://api.example.com"})

	if r.State() != vm.ReplayerStopped || r.Err() == "" || len(b.ServedWith) != 0 {
		t.Fatalf("state %v err %q", r.State(), r.Err())
	}
}

func TestReplayer_Start_Given_fault_fields_Then_the_percentages_become_rates_and_the_jitter_a_duration(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	// Act
	r.Start(vm.ReplayerForm{FaultErrorPercent: "20", FaultStatus: "502", FaultDropPercent: "5", FaultJitter: "300ms"})
	q.run(1)

	// Assert
	f := b.ServedWith[0].Faults
	if f.ErrorRate != 0.2 || f.ErrorStatus != 502 || f.DropRate != 0.05 || f.Jitter.String() != "300ms" {
		t.Fatalf("faults %+v", f)
	}
}

func TestReplayer_Start_Given_a_bad_fault_value_Then_it_says_which_field_and_does_not_start(t *testing.T) {
	for name, form := range map[string]vm.ReplayerForm{
		"error percent text":      {FaultErrorPercent: "lots"},
		"error percent over 100":  {FaultErrorPercent: "120"},
		"drop percent negative":   {FaultDropPercent: "-1"},
		"status not a number":     {FaultErrorPercent: "10", FaultStatus: "oops"},
		"jitter not a duration":   {FaultJitter: "soon"},
		"rates over 100 together": {FaultErrorPercent: "70", FaultDropPercent: "40"},
	} {
		t.Run(name, func(t *testing.T) {
			b := &vmtest.Backend{}
			r, _ := startReplayer(t, b)

			r.Start(form)

			if r.State() != vm.ReplayerStopped || r.Err() == "" || len(b.ServedWith) != 0 {
				t.Fatalf("state %v err %q served %d", r.State(), r.Err(), len(b.ServedWith))
			}
		})
	}
}

func TestReplayer_Start_Given_a_har_file_Then_it_goes_with_the_recording_and_is_refused_without_one(t *testing.T) {
	// Arrange
	b := &vmtest.Backend{}
	r, q := startReplayer(t, b)

	// Act
	r.Start(vm.ReplayerForm{RecordHAR: "/tmp/s.har"})
	refused := r.Err()
	r.Start(vm.ReplayerForm{Record: "https://api.example.test", RecordHAR: "/tmp/s.har"})
	q.run(1)

	// Assert
	if refused == "" || len(b.ServedWith) != 1 || b.ServedWith[0].RecordHAR != "/tmp/s.har" {
		t.Fatalf("refused %q served %+v", refused, b.ServedWith)
	}
}
