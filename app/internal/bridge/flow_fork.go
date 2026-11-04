// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/report"

	"github.com/githugt/flowi/app/internal/vm"
)

// forkValueWidth bounds a variable shown for editing: long enough that nothing a person would edit is cut.
const forkValueWidth = 4000

// ForkStepView is a step of a fork and how it ended.
type ForkStepView struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ForkChangeView is one way the fork went differently from the recording.
type ForkChangeView struct {
	Node string `json:"node"`
	What string `json:"what"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ForkView is a fork of a recorded run.
type ForkView struct {
	Status  string           `json:"status"`
	Steps   []ForkStepView   `json:"steps"`
	Changes []ForkChangeView `json:"changes"`
	Why     string           `json:"why,omitempty"` // what went wrong, when the fork did not pass
}

// ForkVars are the variables the recording held before step from, for the person to change. Nothing is sent.
func (s *FlowService) ForkVars(node, from string) ([]VarValue, error) {
	t, _, err := s.flowTarget(node)
	if err != nil {
		return nil, err
	}
	vars, err := s.app.backend.ForkVariables(t, from)
	if err != nil {
		return nil, err
	}
	out := []VarValue{}
	for _, name := range slices.Sorted(maps.Keys(vars)) {
		out = append(out, VarValue{Name: name, Value: oneLine(vars[name], forkValueWidth), Stale: flowi.StaleReason(name, vars[name], time.Now())})
	}
	return out, nil
}

// ForkRequest says where a fork starts and what is different there.
type ForkRequest struct {
	From string `json:"from"` // the top-level step that runs again
	// Set changes variables before From runs: a value that is JSON keeps its type, anything else is text.
	Set  map[string]string `json:"set"`
	Only bool              `json:"only"` // run just From, not the steps after it
	// AnswerStatus and AnswerBody are the response From's request gets in place of the recorded one: an HTTP status
	// (100 to 599) and a body, JSON unless AnswerText says it is text (an XML or HTML page, a message), in which case
	// AnswerContentType says what kind ("" is plain text). "" keeps what the recording has.
	AnswerStatus      string `json:"answerStatus"`
	AnswerBody        string `json:"answerBody"`
	AnswerText        bool   `json:"answerText"`
	AnswerContentType string `json:"answerContentType"`
}

// Fork runs the flow of a bundle again as req says. Calls are answered from the recording: nothing is sent.
func (s *FlowService) Fork(node string, req ForkRequest) (ForkView, error) {
	t, _, err := s.flowTarget(node)
	if err != nil {
		return ForkView{}, err
	}
	respond, err := answerOf(req)
	if err != nil {
		return ForkView{}, err
	}
	edits := map[string]any{}
	for name, raw := range req.Set {
		var v any
		if json.Unmarshal([]byte(raw), &v) != nil {
			v = raw
		}
		edits[name] = v
	}
	res, err := s.app.backend.ForkReplay(context.Background(), t, flowi.ForkOptions{From: req.From, Set: edits, Only: req.Only, Respond: respond})
	if err != nil {
		return ForkView{}, err
	}
	return forkViewOf(res), nil
}

// answerOf reads the answer a fork gives its first step: nil when none is asked for, an error that says what to fix
// when the status or the body cannot be used.
func answerOf(req ForkRequest) (map[string]flowi.ForkResponse, error) {
	status, body := strings.TrimSpace(req.AnswerStatus), strings.TrimSpace(req.AnswerBody)
	if status == "" && body == "" {
		return nil, nil
	}
	var r flowi.ForkResponse
	if status != "" {
		code, err := strconv.Atoi(status)
		if err != nil || code < 100 || code > 599 {
			return nil, fmt.Errorf("the status %q is not an HTTP status (100 to 599)", status)
		}
		r.Status = code
	}
	switch {
	case body != "" && req.AnswerText:
		// The text is kept as typed, spaces and all: it is not trimmed like the JSON, which is read by a parser.
		r.Text, r.HasText, r.ContentType = req.AnswerBody, true, strings.TrimSpace(req.AnswerContentType)
	case body != "":
		if err := json.Unmarshal([]byte(body), &r.Body); err != nil {
			return nil, fmt.Errorf("the body is not JSON (choose Text if it is not): %w", err)
		}
		r.HasBody = true
	}
	return map[string]flowi.ForkResponse{req.From: r}, nil
}

func forkViewOf(res flowi.ForkResult) ForkView {
	out := ForkView{Status: string(res.Run.Status), Steps: []ForkStepView{}, Changes: []ForkChangeView{}}
	for _, n := range res.Run.Report.Nodes {
		out.Steps = append(out.Steps, ForkStepView{ID: n.ID, Status: n.Status})
	}
	for _, c := range res.Comparison.Changes {
		out.Changes = append(out.Changes, ForkChangeView{Node: c.Node, What: c.What, From: c.From, To: c.To})
	}
	if res.Run.Status != engine.StatusPassed {
		out.Why = whyForkFailed(res.Run)
	}
	return out
}

// whyForkFailed is the first message a failed step gave, else the run's own first problem.
func whyForkFailed(res engine.RunResult) string {
	var walk func(list []report.Node) string
	walk = func(list []report.Node) string {
		for _, n := range list {
			for _, d := range n.Diagnostics {
				if d.Message != "" {
					return fmt.Sprintf("%s: %s", n.ID, d.Message)
				}
			}
			for _, l := range n.Lanes {
				if m := walk(l.Nodes); m != "" {
					return m
				}
			}
		}
		return ""
	}
	if m := walk(res.Report.Nodes); m != "" {
		return m
	}
	return vm.FirstProblem(res)
}
