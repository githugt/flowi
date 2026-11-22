// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/report"
)

// RunSummary is one saved run of a flow, as much as a history list and a
// comparison need: the report's header and each top-level node's outcome.
type RunSummary struct {
	File     string // the report file
	ID       string
	Env      string
	Trigger  string // what started the run, "" for a report that does not say
	Status   string
	Seed     uint64
	Started  time.Time
	Duration time.Duration
	Nodes    []RunNodeSummary
}

// RunNodeSummary is a top-level node's outcome in a saved run.
type RunNodeSummary struct {
	ID       string
	Name     string
	Type     string
	Status   string
	Duration time.Duration
}

// maxFlowRuns bounds FlowRuns when the caller gives no limit.
const maxFlowRuns = 50

// FlowRuns lists the saved runs of flow t, newest first, at most limit of
// them (0: 50). Reports are a cache the engine wrote and nobody edits, so a
// file that cannot be read or parsed is left out rather than failing the list.
func (w *Workspace) FlowRuns(t FlowTarget, limit int) []RunSummary {
	if limit <= 0 {
		limit = maxFlowRuns
	}
	dir := filepath.Join(w.DataDir(t.Collection), "runs", string(t.Flow))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	// Run ids are time-ordered, so the names sort oldest first.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	slices.Sort(names)
	slices.Reverse(names)
	var out []RunSummary
	for _, name := range names {
		if len(out) == limit {
			break
		}
		if s, ok := readRunSummary(filepath.Join(dir, name)); ok {
			out = append(out, s)
		}
	}
	return out
}

func readRunSummary(file string) (RunSummary, bool) {
	data, err := os.ReadFile(file)
	if err != nil {
		return RunSummary{}, false
	}
	var raw struct {
		ID         string    `json:"id"`
		Env        string    `json:"env"`
		Trigger    string    `json:"trigger"`
		Status     string    `json:"status"`
		Seed       uint64    `json:"seed"`
		Started    time.Time `json:"started"`
		DurationMS int64     `json:"duration_ms"`
		Nodes      []struct {
			ID         string `json:"id"`
			Name       string `json:"name"`
			Type       string `json:"type"`
			Status     string `json:"status"`
			DurationMS int64  `json:"duration_ms"`
		} `json:"nodes"`
	}
	if json.Unmarshal(data, &raw) != nil || raw.ID == "" {
		return RunSummary{}, false
	}
	s := RunSummary{File: file, ID: raw.ID, Env: raw.Env, Trigger: raw.Trigger, Status: raw.Status, Seed: raw.Seed, Started: raw.Started,
		Duration: time.Duration(raw.DurationMS) * time.Millisecond}
	for _, n := range raw.Nodes {
		s.Nodes = append(s.Nodes, RunNodeSummary{ID: n.ID, Name: n.Name, Type: n.Type, Status: n.Status,
			Duration: time.Duration(n.DurationMS) * time.Millisecond})
	}
	return s, true
}

// RunDetail is what a saved run held at each top-level node, and the
// variables the flow ended with, for comparing two runs in depth.
type RunDetail struct {
	ID      string
	Outputs map[string]map[string]any // by node id
	Vars    map[string]any
}

// FlowRunDetail reads the saved run id of flow t. The id is a report's file
// name without ".json": letters and digits only, so it cannot name a file
// outside the flow's runs folder.
func (w *Workspace) FlowRunDetail(t FlowTarget, id string) (RunDetail, error) {
	if id == "" || strings.IndexFunc(id, func(r rune) bool { return !isIDRune(r) }) >= 0 {
		return RunDetail{}, fmt.Errorf("%q is not a run id", id)
	}
	file := filepath.Join(w.DataDir(t.Collection), "runs", string(t.Flow), id+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		return RunDetail{}, fmt.Errorf("reading run %s: %w", id, err)
	}
	var raw report.Run
	if err := json.Unmarshal(data, &raw); err != nil {
		return RunDetail{}, fmt.Errorf("reading run %s: %w", id, err)
	}
	out := RunDetail{ID: raw.ID, Outputs: map[string]map[string]any{}, Vars: raw.Vars}
	for _, n := range raw.Nodes {
		out.Outputs[n.ID] = n.Outputs
	}
	return out, nil
}

// RunStep is one step of a saved run, with every instance of it folded in (a step inside a batch ran once for each
// round): the worst status, how many times it ran, and when it ran on the run's clock.
type RunStep struct {
	ID        string
	Status    string
	Instances int
	// Start and End bracket every instance: the first start and the last finish, counted from the start of the run.
	Start, End time.Duration
	// Code is the HTTP status of the last attempt of the last instance, 0 for a step that sent nothing.
	Code int
}

// RunSteps is what a saved run did at each step, in the order the steps first ran.
type RunSteps struct {
	ID     string
	Status string
	Steps  []RunStep
}

// FlowRunSteps reads the saved run id of flow t step by step, steps inside batches, branches and conditions included,
// for showing the run as a list or a timeline. The id is checked as FlowRunDetail checks it.
func (w *Workspace) FlowRunSteps(t FlowTarget, id string) (RunSteps, error) {
	if id == "" || strings.IndexFunc(id, func(r rune) bool { return !isIDRune(r) }) >= 0 {
		return RunSteps{}, fmt.Errorf("%q is not a run id", id)
	}
	file := filepath.Join(w.DataDir(t.Collection), "runs", string(t.Flow), id+".json")
	data, err := os.ReadFile(file)
	if err != nil {
		return RunSteps{}, fmt.Errorf("reading run %s: %w", id, err)
	}
	var raw savedRun
	if err := json.Unmarshal(data, &raw); err != nil {
		return RunSteps{}, fmt.Errorf("reading run %s: %w", id, err)
	}
	out := RunSteps{ID: raw.ID, Status: raw.Status}
	at := map[string]int{}
	var walk func(nodes []savedNode)
	walk = func(nodes []savedNode) {
		for _, n := range nodes {
			i, seen := at[n.ID]
			if !seen {
				i = len(out.Steps)
				at[n.ID] = i
				out.Steps = append(out.Steps, RunStep{ID: n.ID})
			}
			out.Steps[i].fold(n, raw.Started)
			for _, lane := range n.Lanes {
				walk(lane.Nodes)
			}
		}
	}
	walk(raw.Nodes)
	return out, nil
}

// savedRun and savedNode are the parts of a report FlowRunSteps reads; report.Run has no decoder of its own, since
// a report is written by the engine and read only here and by the tools that compare runs.
type savedRun struct {
	ID      string      `json:"id"`
	Status  string      `json:"status"`
	Started time.Time   `json:"started"`
	Nodes   []savedNode `json:"nodes"`
}

type savedNode struct {
	ID         string    `json:"id"`
	Status     string    `json:"status"`
	Started    time.Time `json:"started"`
	DurationMS int64     `json:"duration_ms"`
	Attempts   []struct {
		Status int `json:"status"`
	} `json:"attempts"`
	Lanes []struct {
		Nodes []savedNode `json:"nodes"`
	} `json:"lanes"`
}

// fold adds one instance of the step to s.
func (s *RunStep) fold(n savedNode, runStart time.Time) {
	s.Instances++
	if rank(n.Status) > rank(s.Status) {
		s.Status = n.Status
	}
	if k := len(n.Attempts); k > 0 {
		s.Code = n.Attempts[k-1].Status
	}
	if n.Started.IsZero() || runStart.IsZero() {
		return
	}
	start := max(n.Started.Sub(runStart), 0)
	end := start + time.Duration(n.DurationMS)*time.Millisecond
	if s.Instances == 1 || start < s.Start {
		s.Start = start
	}
	s.End = max(s.End, end)
}

// rank orders statuses by how much a person needs to see them: the worst instance names the step.
func rank(status string) int {
	switch status {
	case "failed", "error":
		return 5
	case "aborted":
		return 4
	case "cancelled":
		return 3
	case "passed":
		return 2
	case "":
		return 0
	}
	return 1
}

// DeleteFlowRun deletes the saved run id of flow t. The id is checked as FlowRunDetail checks it. A run that is
// already gone is not an error: the list the person clicked in may be a moment old.
func (w *Workspace) DeleteFlowRun(t FlowTarget, id string) error {
	if id == "" || strings.IndexFunc(id, func(r rune) bool { return !isIDRune(r) }) >= 0 {
		return fmt.Errorf("%q is not a run id", id)
	}
	file := filepath.Join(w.DataDir(t.Collection), "runs", string(t.Flow), id+".json")
	if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("deleting run %s: %w", id, err)
	}
	return nil
}

// ClearFlowRuns deletes every saved run of flow t, the ones FlowRuns leaves out as unreadable too, and says how many
// files it removed. Other files in the folder are left alone.
func (w *Workspace) ClearFlowRuns(t FlowTarget) (int, error) {
	dir := filepath.Join(w.DataDir(t.Collection), "runs", string(t.Flow))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("reading the runs of %s: %w", t.Flow, err)
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil && !os.IsNotExist(err) {
			return removed, fmt.Errorf("deleting run %s: %w", strings.TrimSuffix(e.Name(), ".json"), err)
		}
		removed++
	}
	return removed, nil
}

func isIDRune(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}
