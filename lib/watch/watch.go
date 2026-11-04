// SPDX-License-Identifier: Apache-2.0

// Package watch tells whether an API changed between two runs of a read-only flow: a request that now answers
// with another status, a response whose shape moved (a field gone or new, or of another type), or a call that
// became much slower. It compares snapshots; the caller runs the flow and keeps the last snapshot.
package watch

import (
	"slices"
	"strconv"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/schemainfer"
)

// slowFactor is how many times slower than before a call must be to count as a change: a call that varies by a
// factor of two between runs is noise, not news.
const slowFactor = 3

// Change kinds.
const (
	KindStatus  = "status"
	KindShape   = "shape"
	KindLatency = "latency"
)

// Step is what one request looked like in a run.
type Step struct {
	Request   string             `json:"request"`
	Status    int                `json:"status"`
	LatencyMS int64              `json:"latency_ms"`
	Schema    schemainfer.Schema `json:"schema,omitempty"` // inferred from the JSON body; empty when there was none

	body any // the body itself, kept to compare against the next snapshot's schema
}

// Snapshot is what a run's calls looked like, one step per request (its first call).
type Snapshot struct {
	At    time.Time `json:"at"`
	Steps []Step    `json:"steps"`
}

// Change is one thing that moved between two snapshots.
type Change struct {
	Request string `json:"request"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

// Take makes a snapshot of the calls of a run: HTTP calls only, the first call of each request.
func Take(records []engine.Record) Snapshot {
	s := Snapshot{At: time.Now().UTC(), Steps: []Step{}}
	seen := map[string]bool{}
	for _, r := range records {
		if r.Kind != "" || seen[r.RequestPath] {
			continue
		}
		seen[r.RequestPath] = true
		step := Step{Request: r.RequestPath, Status: r.Status, LatencyMS: r.Timings.Total.Milliseconds(), body: r.Body.JSON}
		if r.Body.JSON != nil {
			step.Schema = schemainfer.Infer(r.Body.JSON)
		}
		s.Steps = append(s.Steps, step)
	}
	return s
}

// Compare lists what changed in cur since prev. A first snapshot (prev nil) changed nothing, and a request prev
// did not call is new, not changed.
func Compare(prev *Snapshot, cur Snapshot) []Change {
	changes := []Change{}
	if prev == nil {
		return changes
	}
	before := map[string]Step{}
	for _, s := range prev.Steps {
		before[s.Request] = s
	}
	for _, now := range cur.Steps {
		was, known := before[now.Request]
		if !known {
			continue
		}
		changes = append(changes, stepChanges(was, now)...)
	}
	return changes
}

func stepChanges(was, now Step) []Change {
	var out []Change
	if was.Status != now.Status {
		out = append(out, Change{Request: now.Request, Kind: KindStatus, Detail: statusDetail(was.Status, now.Status)})
	}
	if len(was.Schema) > 0 && now.body != nil {
		for _, d := range schemainfer.Drift(was.Schema, now.body) {
			out = append(out, Change{Request: now.Request, Kind: KindShape, Detail: d.Path + ": " + d.Detail})
		}
	}
	if was.LatencyMS > 0 && now.LatencyMS > was.LatencyMS*slowFactor {
		out = append(out, Change{Request: now.Request, Kind: KindLatency, Detail: latencyDetail(was.LatencyMS, now.LatencyMS)})
	}
	return out
}

func statusDetail(was, now int) string {
	return strconv.Itoa(was) + " -> " + strconv.Itoa(now)
}

func latencyDetail(was, now int64) string {
	return strconv.Itoa(int(was)) + "ms -> " + strconv.Itoa(int(now)) + "ms"
}

// HistoryLimit is how many passes a flow's history keeps: enough for a sparkline, small enough to read in a blink.
const HistoryLimit = 60

// Pass is one watch pass in a flow's history.
type Pass struct {
	At      time.Time `json:"at"`
	Status  string    `json:"status"`   // how the run ended: passed, failed, …
	Changes int       `json:"changes"`  // how many things moved since the pass before
	TotalMS int64     `json:"total_ms"` // the calls' latencies added up
}

// PassOf summarises a pass for the history.
func PassOf(s Snapshot, status string, changes []Change) Pass {
	var total int64
	for _, step := range s.Steps {
		total += step.LatencyMS
	}
	return Pass{At: s.At, Status: status, Changes: len(changes), TotalMS: total}
}

// Append adds p to the history and keeps the latest limit passes, oldest first. It does not change history.
func Append(history []Pass, p Pass, limit int) []Pass {
	out := append(slices.Clone(history), p)
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out
}
