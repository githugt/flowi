// SPDX-License-Identifier: Apache-2.0

package report

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

// A report read back keeps how long each part took: a timeline of a saved run needs it.
func TestReport_should_read_back_the_durations_it_wrote(t *testing.T) {
	ms := time.Millisecond
	in := Run{ID: "r", Duration: 90 * ms, Nodes: []Node{
		{ID: "a", Duration: 20 * ms, Attempts: []Attempt{{N: 1, Duration: 15 * ms}}, Hooks: []Hook{{Phase: "pre", Duration: 3 * ms}},
			Lanes: []Lane{{Label: "x", Nodes: []Node{{ID: "b", Duration: 7 * ms}}}}, Wrote: []VarWrite{{Name: "v", Value: "1"}}},
	}}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, in); err != nil {
		t.Fatal(err)
	}

	var out Run
	err := json.Unmarshal(buf.Bytes(), &out)

	if err != nil {
		t.Fatal(err)
	}
	a := out.Nodes[0]
	if out.Duration != 90*ms || a.Duration != 20*ms || a.Attempts[0].Duration != 15*ms || a.Hooks[0].Duration != 3*ms || a.Lanes[0].Nodes[0].Duration != 7*ms {
		t.Fatalf("durations lost: %+v", out)
	}
	if len(a.Wrote) != 1 || a.Wrote[0].Name != "v" {
		t.Fatalf("writes lost: %+v", a.Wrote)
	}
}
