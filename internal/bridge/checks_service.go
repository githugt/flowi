// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"fmt"
	"os"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/model"
)

// ChecksService backs the Checks screen: how much of the API descriptions the flows use, and what a changed
// description would break. Both read files and may take a moment, so they run on the caller, not the actor.
type ChecksService struct{ app *App }

// StatusView is a documented status and whether a saved run ever got it.
type StatusView struct {
	Status int  `json:"status"`
	Seen   bool `json:"seen"`
}

// OperationView is one operation of an imported description.
type OperationView struct {
	Collection string       `json:"collection"`
	Request    string       `json:"request"`
	Version    string       `json:"version"`
	Used       []string     `json:"used"` // the flows that call it
	Statuses   []StatusView `json:"statuses"`
	Read       []string     `json:"read"`   // response fields a flow reads
	Unread     []string     `json:"unread"` // response fields no flow reads
}

// CoverageView is the coverage of the flows against the descriptions, with its totals.
type CoverageView struct {
	Operations      []OperationView `json:"operations"`
	OperationsUsed  int             `json:"operationsUsed"`
	OperationsTotal int             `json:"operationsTotal"`
	StatusesSeen    int             `json:"statusesSeen"`
	StatusesTotal   int             `json:"statusesTotal"`
	FieldsRead      int             `json:"fieldsRead"`
	FieldsTotal     int             `json:"fieldsTotal"`
}

// ReadView is one read a change breaks, and why.
type ReadView struct {
	Expr string `json:"expr"`
	Why  string `json:"why"`
}

// BreakageView is one consumer a changed description breaks.
type BreakageView struct {
	Request string     `json:"request"`
	Version string     `json:"version"`
	Where   string     `json:"where"`  // "flow checkout › step pay"
	Reason  string     `json:"reason"` // set when the whole request is gone from the new description
	Reads   []ReadView `json:"reads"`
}

// SpecChangeView is what a changed description would break.
type SpecChangeView struct {
	Spec      string         `json:"spec"`
	Broken    bool           `json:"broken"`
	Breakages []BreakageView `json:"breakages"`
}

// Targets are the collections the checks can look at (not the Scratch draft).
func (s *ChecksService) Targets() []Choice {
	out := []Choice{}
	for _, c := range s.app.workspace.Collections() {
		if s.app.workspace.IsDraftCollection(c.Manifest.ID) {
			continue
		}
		out = append(out, Choice{ID: string(c.Manifest.ID), Label: c.Manifest.Name, Path: c.Manifest.Name})
	}
	return out
}

// Specs are the API descriptions a collection kept at import, by name.
func (s *ChecksService) Specs(collection string) []string {
	return append([]string{}, s.app.backend.SpecNames(model.ID(collection))...)
}

// Coverage measures the flows of a collection ("" for all of them) against its descriptions.
func (s *ChecksService) Coverage(collection string) (CoverageView, error) {
	cov, err := s.app.backend.Coverage(model.ID(collection))
	if err != nil {
		return CoverageView{}, err
	}
	v := CoverageView{Operations: []OperationView{}}
	for _, op := range cov.Operations {
		o := OperationView{Collection: op.Collection, Request: op.Request, Version: op.Version, Used: nonNil(op.Used), Read: nonNil(op.Read), Unread: nonNil(op.Unread), Statuses: []StatusView{}}
		for _, st := range op.Statuses {
			o.Statuses = append(o.Statuses, StatusView{Status: st.Status, Seen: st.Seen})
		}
		v.Operations = append(v.Operations, o)
	}
	t := cov.Totals()
	v.OperationsUsed, v.OperationsTotal, v.StatusesSeen, v.StatusesTotal, v.FieldsRead, v.FieldsTotal = t.Used, t.Operations, t.SeenStatuses, t.Statuses, t.ReadFields, t.Fields
	return v, nil
}

// SpecChange says what the OpenAPI description at ref (a file or an http(s) address) would break if it replaced the
// one the collection kept as name ("" when it kept one). Nothing is written.
func (s *ChecksService) SpecChange(collection, name, ref string) (SpecChangeView, error) {
	if collection == "" {
		return SpecChangeView{}, fmt.Errorf("choose the collection whose flows to check")
	}
	data, err := readSpec(ref)
	if err != nil {
		return SpecChangeView{}, err
	}
	res, err := s.app.backend.ImpactAgainst(model.ID(collection), name, data)
	if err != nil {
		return SpecChangeView{}, err
	}
	v := SpecChangeView{Spec: res.Spec, Broken: res.Broken(), Breakages: []BreakageView{}}
	for _, b := range res.Breakages {
		bv := BreakageView{Request: b.Request, Version: b.Version, Where: consumerWhere(b.Consumer), Reason: b.Consumer.Reason, Reads: []ReadView{}}
		for _, r := range b.Consumer.Reads {
			bv.Reads = append(bv.Reads, ReadView{Expr: r.Expr, Why: r.Reason})
		}
		v.Breakages = append(v.Breakages, bv)
	}
	return v, nil
}

// readSpec reads the description from a file, or downloads it.
func readSpec(ref string) ([]byte, error) {
	if flowi.IsImportURL(ref) {
		data, _, err := flowi.FetchImportSource(context.Background(), ref)
		return data, err
	}
	data, err := os.ReadFile(ref) //nolint:gosec // the person named the file in the window
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", ref, err)
	}
	return data, nil
}

// consumerWhere says where a consumer is written, the way the command line does.
func consumerWhere(c analysis.Consumer) string {
	switch {
	case c.Kind == analysis.KindRequest:
		return "the request's own extract and assert"
	case c.Kind == analysis.KindHook:
		return "hook " + c.Hook
	case c.Flow != "":
		return "flow " + c.Flow + " › step " + c.Node
	}
	return "hook " + c.Hook + " › step " + c.Node
}
