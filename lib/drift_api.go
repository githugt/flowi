// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
)

// DriftOptions configure CheckDrift.
type DriftOptions struct {
	Env    string
	Inputs map[string]string
	Seed   uint64
}

// RequestDrift is what one request version's response, with one status, did against its stored schema.
type RequestDrift struct {
	Request     string // the request's path
	Version     string
	Status      int
	NoSchema    bool                     // no schema is stored for this response yet, so nothing could be compared
	Differences []schemainfer.Difference // what moved, across every call of the run
}

// DriftReport is the result of CheckDrift.
type DriftReport struct {
	Flow      string
	RunStatus engine.Status // how the run itself ended; a failed run still reports what it did get
	Requests  []RequestDrift
}

// Has says whether any request drifted in one of the kinds.
func (r DriftReport) Has(kinds ...schemainfer.DriftKind) bool {
	return slices.ContainsFunc(r.Requests, func(q RequestDrift) bool {
		return slices.ContainsFunc(q.Differences, func(d schemainfer.Difference) bool { return slices.Contains(kinds, d.Kind) })
	})
}

// CheckDrift runs the flow without learning and compares every response with the schema stored for it, then
// lists what moved: new fields, required fields that are gone, fields of another type. The stored schemas are not
// touched, so the next check measures against the same baseline (a normal run widens them as the API changes,
// which is how a baseline is lost). A response with no stored schema is reported as such and does not create one.
func (w *Workspace) CheckDrift(ctx context.Context, t FlowTarget, o DriftOptions) (DriftReport, error) {
	c, f, err := w.flowFor(t)
	if err != nil {
		return DriftReport{}, err
	}
	res, err := w.runQuietly(ctx, t, RunOptions{Env: o.Env, Inputs: o.Inputs, Seed: o.Seed, SkipSchemas: true})
	if err != nil {
		return DriftReport{}, err
	}
	rep := DriftReport{Flow: f.Path, RunStatus: res.Status}
	for _, rec := range res.Captured {
		w.addDrift(&rep, c, rec)
	}
	slices.SortFunc(rep.Requests, func(a, b RequestDrift) int {
		return strings.Compare(a.Request+"\x00"+a.Version, b.Request+"\x00"+b.Version)
	})
	return rep, nil
}

// addDrift compares one call's response with its schema and merges the result into the report.
func (w *Workspace) addDrift(rep *DriftReport, c model.Collection, rec engine.Record) {
	if rec.HasMessages() || rec.Status == 0 || rec.Body.JSON == nil {
		return // only a JSON response has a schema to compare with
	}
	_, r, err := w.requestFor(RequestTarget{Collection: c.Manifest.ID, Request: rec.RequestID})
	if err != nil {
		return
	}
	entry := RequestDrift{Request: r.Path, Version: rec.Version, Status: rec.Status}
	file, err := schemaFile(r.File, rec.Version, SchemaPart{Kind: "response", Status: rec.Status})
	if err != nil {
		return
	}
	schema, _, found := readSchema(file)
	if !found {
		entry.NoSchema = true
	} else {
		entry.Differences = schemainfer.Drift(schema, rec.Body.JSON)
	}
	rep.merge(entry)
}

// merge adds entry, joining it with an earlier entry for the same request, version and status.
func (r *DriftReport) merge(entry RequestDrift) {
	for i := range r.Requests {
		q := &r.Requests[i]
		if q.Request != entry.Request || q.Version != entry.Version || q.Status != entry.Status {
			continue
		}
		for _, d := range entry.Differences {
			if !slices.Contains(q.Differences, d) {
				q.Differences = append(q.Differences, d)
			}
		}
		return
	}
	r.Requests = append(r.Requests, entry)
}
