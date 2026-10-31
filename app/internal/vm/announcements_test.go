// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/announce"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestAnnouncements_Load_should_list_the_items_the_backend_returns(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "a", Title: "Hello"}}}}
	a := vm.NewAnnouncements(b, w)
	changed := 0
	a.OnChange(func() { changed++ })

	a.Load()
	q.run(1)

	if len(a.Items()) != 1 || a.Items()[0].Title != "Hello" || changed == 0 {
		t.Fatalf("items %+v changed %d", a.Items(), changed)
	}
}

func TestAnnouncements_Load_Given_nothing_to_fetch_or_a_failure_Then_no_items_and_no_noise(t *testing.T) {
	for name, err := range map[string]error{
		"not configured": announce.ErrNotConfigured, "turned off": announce.ErrTurnedOff, "offline": errors.New("no route to host"),
	} {
		t.Run(name, func(t *testing.T) {
			q, w := newUIQueue()
			a := vm.NewAnnouncements(&vmtest.Backend{NewsErr: err}, w)

			a.Load()
			q.run(1)

			if len(a.Items()) != 0 {
				t.Fatalf("items %+v", a.Items())
			}
		})
	}
}

func TestAnnouncements_Load_Given_a_failure_with_cached_items_Then_the_cached_items_are_shown(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "a", Title: "Cached"}}, Stale: true}, NewsErr: errors.New("offline")}
	a := vm.NewAnnouncements(b, w)

	a.Load()
	q.run(1)

	if len(a.Items()) != 1 {
		t.Fatalf("items %+v", a.Items())
	}
}

func TestAnnouncements_Load_should_leave_out_the_ones_already_dismissed(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{
		News:   announce.Result{Items: []announce.Item{{ID: "a", Title: "Old"}, {ID: "b", Title: "New"}}},
		Values: map[string]any{"announcements_dismissed": []any{"a"}},
	}
	a := vm.NewAnnouncements(b, w)

	a.Load()
	q.run(1)

	if len(a.Items()) != 1 || a.Items()[0].ID != "b" {
		t.Fatalf("items %+v", a.Items())
	}
}

func TestAnnouncements_Dismiss_should_remember_it_and_drop_it_from_the_list(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{
		News:   announce.Result{Items: []announce.Item{{ID: "a", Title: "A"}, {ID: "b", Title: "B"}}},
		Values: map[string]any{"announcements_dismissed": []any{"old"}},
	}
	a := vm.NewAnnouncements(b, w)
	a.Load()
	q.run(1)

	a.Dismiss("a")
	q.run(1)

	if len(a.Items()) != 1 || a.Items()[0].ID != "b" || !b.Called("SetUserSetting announcements_dismissed=old,a") {
		t.Fatalf("items %+v calls %v", a.Items(), b.Calls)
	}
}

func TestAnnouncements_TurnOff_should_switch_the_setting_off_and_clear_the_list(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "a", Title: "A"}}}}
	a := vm.NewAnnouncements(b, w)
	a.Load()
	q.run(1)

	a.TurnOff()
	q.run(1)

	if len(a.Items()) != 0 || !b.Called("SetUserSetting announcements=false") {
		t.Fatalf("items %+v calls %v", a.Items(), b.Calls)
	}
}

func TestAnnouncements_Dismiss_Given_two_quick_calls_Then_both_are_remembered(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "a"}, {ID: "b"}}}}
	a := vm.NewAnnouncements(b, w)
	a.Load()
	q.run(1)

	a.Dismiss("a")
	a.Dismiss("b")
	a.Dismiss("b")
	q.run(3)

	got := dismissedOf(b)
	slices.Sort(got) // the two calls run concurrently
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("dismissed %v", got)
	}
}

func dismissedOf(b *vmtest.Backend) []string {
	b.Mu.Lock()
	defer b.Mu.Unlock()
	switch list := b.Values["announcements_dismissed"].(type) {
	case []string:
		return list
	case []any:
		var out []string
		for _, e := range list {
			out = append(out, e.(string))
		}
		return out
	}
	return nil
}

func TestAnnouncements_Dismiss_Given_a_write_failure_Then_the_error_is_reported_and_the_item_stays(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "a"}}}, SetErr: errors.New("read-only")}
	a := vm.NewAnnouncements(b, w)
	var failed error
	a.OnError(func(err error) { failed = err })
	a.Load()
	q.run(1)

	a.Dismiss("a")
	q.run(1)

	if failed == nil || len(a.Items()) != 1 {
		t.Fatalf("failed %v items %+v", failed, a.Items())
	}
}

func TestAnnouncements_Load_should_leave_out_ids_that_could_never_be_remembered(t *testing.T) {
	q, w := newUIQueue()
	b := &vmtest.Backend{News: announce.Result{Items: []announce.Item{{ID: "x,y"}, {ID: "ok"}}}}
	a := vm.NewAnnouncements(b, w)

	a.Load()
	q.run(1)

	if len(a.Items()) != 1 || a.Items()[0].ID != "ok" {
		t.Fatalf("items %+v", a.Items())
	}
}
