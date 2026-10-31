// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/githugt/flowi-lib/model"
)

func TestSend_should_pass_every_event_to_Observe_in_order(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("ping", get("/echo/x"))
	var mu sync.Mutex
	var observed []string
	deps := f.deps()
	deps.Observe = func(ev Event) { mu.Lock(); observed = append(observed, ev.EventName()); mu.Unlock() }

	s := StartSend(context.Background(), deps, f.input(r))
	var streamed []string
	for ev := range s.Events() {
		streamed = append(streamed, ev.EventName())
	}
	s.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(streamed) == 0 || strings.Join(observed, ",") != strings.Join(streamed, ",") {
		t.Fatalf("observed %v streamed %v", observed, streamed)
	}
}

func TestSend_Given_Traceparent_Then_header_is_sent_unless_the_user_set_one(t *testing.T) {
	tests := []struct {
		name    string
		headers []model.KeyValue
		want    string
	}{
		{name: "added", want: "00-aaaa-bbbb-01"},
		{name: "user header wins", headers: []model.KeyValue{{Key: "traceparent", Value: "00-mine-mine-01"}}, want: "00-mine-mine-01"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			v := get("/echo/x")
			v.Headers = tt.headers
			r := f.addRequest("ping", v)
			deps := f.deps()
			deps.Traceparent = func(_, _ string) string { return "00-aaaa-bbbb-01" }

			s := StartSend(context.Background(), deps, f.input(r))
			for range s.Events() {
			}
			s.Wait()

			if got := f.lastEcho()["Traceparent"]; got != tt.want {
				t.Fatalf("traceparent %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSend_Given_no_Traceparent_Then_no_header(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("ping", get("/echo/x"))

	f.send(f.input(r))

	if got := f.lastEcho()["Traceparent"]; got != "" {
		t.Fatalf("traceparent %q", got)
	}
}

func TestObservers_should_call_every_non_nil_observer_in_order(t *testing.T) {
	var got []string
	record := func(tag string) func(Event) { return func(Event) { got = append(got, tag) } }

	Observers(record("a"), nil, record("b"))(RunCancelled{})

	if strings.Join(got, ",") != "a,b" {
		t.Fatalf("got %v", got)
	}
}

func TestObservers_Given_none_Then_nil(t *testing.T) {
	if Observers(nil, nil) != nil {
		t.Fatal("a no-op observer would still cost a call per event")
	}
}
