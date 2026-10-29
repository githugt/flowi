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

	"github.com/githugt/flowi-lib/report"
)

// RunSummary is one saved run of a flow, as much as a history list and a
// comparison need: the report's header and each top-level node's outcome.
type RunSummary struct {
	File     string // the report file
	ID       string
	Env      string
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
	s := RunSummary{File: file, ID: raw.ID, Env: raw.Env, Status: raw.Status, Seed: raw.Seed, Started: raw.Started,
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

func isIDRune(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}
