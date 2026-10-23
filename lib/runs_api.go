// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
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
