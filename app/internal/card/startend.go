// SPDX-License-Identifier: AGPL-3.0-or-later

package card

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// StartCard is the flow's first card: a view of its inputs, in the order the
// file declares them (the person drags to reorder), not a step. It shows every
// input, since each is one row of its own, and never More.
func StartCard(inputs []model.Input) Card {
	c := Card{Badge: "MANUAL", Subline: countOf(len(inputs), "input", "inputs"), Rows: []Row{}, More: []Row{}, Ports: []Port{}}
	if len(inputs) == 0 {
		c.Subline = "no inputs"
	}
	for _, in := range inputs {
		c.Rows = append(c.Rows, Row{Key: "input:" + in.Name, Icon: "variables", Summary: cut(inputSummary(in), cutAt), Editor: "input"})
	}
	return c
}

func inputSummary(in model.Input) string {
	if in.Type == "" {
		return in.Name
	}
	return in.Name + " · " + in.Type
}

// EndCard is the flow's last card: the values the flow hands back. It is not
// drawn (false) for a flow with no outputs. With a finished run, each row
// carries the value the run ended with and the sub-line is the run's short
// report; an output the run could not work out says so.
func EndCard(outputs []model.Output, run *report.Run) (Card, bool) {
	if len(outputs) == 0 {
		return Card{}, false
	}
	c := Card{Badge: "RETURNS", Subline: countOf(len(outputs), "value", "values"), Rows: []Row{}, More: []Row{}, Ports: []Port{}}
	if run != nil {
		c.Subline = resultLine(run)
	}
	for _, o := range outputs {
		c.Rows = append(c.Rows, Row{Key: "output:" + o.Name, Icon: "extract", Summary: cut(outputSummary(o, run), cutAt), Editor: "output"})
	}
	return c, true
}

func outputSummary(o model.Output, run *report.Run) string {
	if run == nil {
		return o.Name + " = " + o.Value
	}
	for _, v := range run.Outputs {
		if v.Name == o.Name {
			return o.Name + " = " + valueText(v.Value)
		}
	}
	return o.Name + " = no value"
}

// resultLine is the run's short report: how it ended, how many steps passed
// and how long it took.
func resultLine(run *report.Run) string {
	passed := 0
	for _, n := range run.Nodes {
		if n.Status == "passed" {
			passed++
		}
	}
	return fmt.Sprintf("%s · %d of %d steps · %s", run.Status, passed, len(run.Nodes), run.Duration.Round(100*time.Millisecond))
}

// valueText is a value on one line: text as it is, anything else as JSON.
func valueText(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprint(v)
	}
	return string(b)
}

func countOf(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}
