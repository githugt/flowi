// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func startScheduleRunner(t *testing.T, b *vmtest.Backend) (*vm.ScheduleRunner, *uiQueue) {
	t.Helper()
	q, w := newUIQueue()
	r := vm.NewScheduleRunner(b, w)
	t.Cleanup(func() {
		closed := make(chan struct{})
		go func() { r.Close(); close(closed) }()
		for {
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

func TestScheduleRunner_Start_should_serve_schedules_and_triggers_without_the_replayer(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startScheduleRunner(t, b)

	r.Start()
	if r.State() != vm.ScheduleStarting {
		t.Fatalf("state %v", r.State())
	}
	q.run(1)

	o := b.ServedWith[0]
	if r.State() != vm.ScheduleRunning || !o.NoReplay || o.NoTriggers || o.NoSchedules {
		t.Fatalf("state %v, served with %+v", r.State(), o)
	}
	if up := r.Upcoming(); len(up) != 1 || up[0].Name != "nightly" || r.TriggerAddr() == "" {
		t.Fatalf("upcoming %+v, triggers on %q", up, r.TriggerAddr())
	}
}

func TestScheduleRunner_Stop_should_end_the_run_and_forget_what_was_upcoming(t *testing.T) {
	b := &vmtest.Backend{}
	r, q := startScheduleRunner(t, b)
	r.Start()
	q.run(1)

	r.Stop()
	stopping := r.State()
	q.run(1) // the waiter reports the stop

	if stopping != vm.ScheduleStopping || r.State() != vm.ScheduleStopped || len(r.Upcoming()) != 0 || r.Err() != "" {
		t.Fatalf("stopping %v, then %v, upcoming %+v, err %q", stopping, r.State(), r.Upcoming(), r.Err())
	}
}

func TestScheduleRunner_Start_Given_the_server_cannot_start_Then_shows_why(t *testing.T) {
	b := &vmtest.Backend{ServeErr: errors.New("address already in use")}
	r, q := startScheduleRunner(t, b)

	r.Start()
	q.run(1)

	if r.State() != vm.ScheduleStopped || r.Err() == "" {
		t.Fatalf("state %v err %q", r.State(), r.Err())
	}
}
