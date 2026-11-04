// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/watch"
)

func TestWatch_should_list_what_changed_since_the_last_pass(t *testing.T) {
	flows, v, b := openChecksFlow(t)
	b.Check.Watch = flowi.WatchResult{Status: engine.StatusPassed, Steps: 2, Change: []watch.Change{
		{Request: "auth/login", Kind: "status", Detail: "200 → 500"},
	}}

	got, err := flows.Watch(v.Node, "local")

	if err != nil || got.Steps != 2 || len(got.Changes) != 1 || got.Changes[0].Kind != "status" || len(got.Refused) != 0 {
		t.Fatalf("watch %+v err %v", got, err)
	}
}

func TestWatch_Given_a_flow_that_changes_things_Then_it_is_refused_with_the_reasons_and_not_run(t *testing.T) {
	flows, v, b := openChecksFlow(t)
	b.Check.WatchProblems = []string{"step a sends POST, which may change something on the server"}

	got, err := flows.Watch(v.Node, "local")

	if err != nil || len(got.Refused) != 1 || got.Changes == nil {
		t.Fatalf("watch %+v err %v", got, err)
	}
	if b.Called("WatchFlow local") {
		t.Fatal("a refused flow must not run")
	}
}
