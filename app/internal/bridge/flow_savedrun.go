// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"slices"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
)

// SavedRunView is a saved run laid out the way the live run is, so the Steps list and the Timeline show either one:
// each step's status and result line, and the bars on the run's clock.
type SavedRunView struct {
	Statuses map[string]string `json:"statuses"`
	Results  map[string]string `json:"results"`
	Timeline []TimelineBar     `json:"timeline"`
}

// RunSteps reads the saved run id of the flow step by step.
func (s *FlowService) RunSteps(node, id string) (SavedRunView, error) {
	type result struct {
		steps flowi.RunSteps
		err   error
	}
	done := make(chan result, 1)
	if err := s.edit(node, func(o *openFlow) error {
		o.history.Steps(id, func(r flowi.RunSteps, err error) { done <- result{r, err} })
		return nil
	}); err != nil {
		return SavedRunView{}, err
	}
	r := <-done
	if r.err != nil {
		return SavedRunView{}, r.err
	}
	return savedRunViewOf(r.steps), nil
}

// savedRunViewOf lays the steps of a saved run out for the Steps list and the Timeline. A step with no start time
// (a report from before steps carried one) is listed but has no bar.
func savedRunViewOf(r flowi.RunSteps) SavedRunView {
	v := SavedRunView{Statuses: map[string]string{}, Results: map[string]string{}, Timeline: []TimelineBar{}}
	for _, st := range r.Steps {
		v.Statuses[st.ID] = st.Status
		v.Results[st.ID] = savedResultLine(st)
		if st.End > 0 || st.Start > 0 {
			v.Timeline = append(v.Timeline, TimelineBar{Step: st.ID, Status: st.Status, StartMs: ms(st.Start), EndMs: ms(max(st.End, st.Start))})
		}
	}
	slices.SortStableFunc(v.Timeline, func(a, b TimelineBar) int { return int(a.StartMs - b.StartMs) })
	return v
}

// savedResultLine is a step's line in the list, as resultLine words it for a live run: the status when it is not a
// plain pass, the HTTP code, how many times it ran, and how long.
func savedResultLine(st flowi.RunStep) string {
	var parts []string
	if st.Status != "passed" || st.Code == 0 {
		parts = append(parts, st.Status)
	}
	if st.Code > 0 {
		parts = append(parts, fmt.Sprint(st.Code))
	}
	if st.Instances > 1 {
		parts = append(parts, fmt.Sprintf("%d runs", st.Instances))
	}
	if d := st.End - st.Start; d > 0 {
		parts = append(parts, d.Round(time.Millisecond).String())
	}
	return strings.Join(parts, " · ")
}
