// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/lib/engine"
)

func TestResultLine(t *testing.T) {
	// Before: every card said only "passed · 120ms".
	rec := func(status int, size int64) *engine.Record {
		return &engine.Record{Status: status, Body: engine.Body{Size: size}}
	}
	cases := []struct {
		name string
		d    vm.NodeDetail
		want string
	}{
		{"a request with a status and a body", vm.NodeDetail{Status: "passed", Duration: 120 * time.Millisecond, Record: rec(200, 2300)}, "200 · 2.3 KB · 120ms"},
		{"a failed request keeps the word", vm.NodeDetail{Status: "failed", Duration: time.Second, Record: rec(500, 0)}, "failed · 500 · 1s"},
		{"a step that ran several times", vm.NodeDetail{Status: "passed", Instances: 5, Duration: 2100 * time.Millisecond}, "passed · 5 runs · 2.1s"},
		{"a step with no record keeps the old line", vm.NodeDetail{Status: "passed", Instances: 1, Duration: 40 * time.Millisecond}, "passed · 40ms"},
		{"a running step has no time", vm.NodeDetail{Status: "running"}, "running"},
		{"megabytes", vm.NodeDetail{Status: "passed", Record: rec(200, 4_100_000)}, "200 · 4.1 MB"},
	}
	for _, c := range cases {
		if got := resultLine(c.d); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
