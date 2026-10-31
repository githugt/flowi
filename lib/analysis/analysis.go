// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"errors"
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

// Errors for Impact's target.
var (
	ErrUnknownRequest = errors.New("unknown request")
	ErrUnknownVersion = errors.New("unknown version")
)

// ContractInfo says what a version's consumers were checked against.
type ContractInfo struct {
	Source string   `json:"source"` // SourceExample, SourceResponse, or empty when nothing is known
	Files  []string `json:"files,omitempty"`
}

// VersionUsage is every consumer of one request version.
type VersionUsage struct {
	Version   string       `json:"version"`
	Contract  ContractInfo `json:"contract"`
	Consumers []Consumer   `json:"consumers"`
	Status    Status       `json:"status"` // the worst consumer's
}

// Report is the impact of one request: its versions and their consumers.
type Report struct {
	Collection   string         `json:"collection"`
	CollectionID model.ID       `json:"collection_id"`
	Request      string         `json:"request"` // path, e.g. auth/login
	RequestID    model.ID       `json:"request_id"`
	Versions     []VersionUsage `json:"versions"`
	Status       Status         `json:"status"` // the worst version's
}

// Impact reports who uses request t, across every collection in cs, and
// whether they still fit its contract. version limits the report to one
// version; empty means all of them. diags are contract-source problems.
func Impact(cs []model.Collection, t Target, version string, src Contracts) (Report, diag.List, error) {
	ix := newIndex(cs)
	c, r, ok := ix.request(t)
	if !ok {
		return Report{}, nil, fmt.Errorf("%w %s", ErrUnknownRequest, t.Request)
	}
	versions, err := selectVersions(r, version)
	if err != nil {
		return Report{}, nil, err
	}
	rep, diags := reportOf(ix, c, r, versions, src)
	return rep, unique(diags), nil
}

func selectVersions(r model.Request, version string) ([]model.RequestVersion, error) {
	if version == "" {
		return r.Versions, nil
	}
	for _, v := range r.Versions {
		if v.Label == version {
			return []model.RequestVersion{v}, nil
		}
	}
	return nil, fmt.Errorf("%w %q of %s", ErrUnknownVersion, version, r.Path)
}

// Analyze reports every request version that has consumers, for
// validate's contract warnings.
func Analyze(cs []model.Collection, src Contracts) ([]Report, diag.List) {
	ix := newIndex(cs)
	var reports []Report
	var diags diag.List
	for _, c := range cs {
		for _, r := range c.Requests {
			rep, d := reportOf(ix, c, r, r.Versions, src)
			diags = append(diags, d...)
			if hasConsumers(rep) {
				reports = append(reports, rep)
			}
		}
	}
	return reports, unique(diags)
}

func hasConsumers(r Report) bool {
	for _, v := range r.Versions {
		if len(v.Consumers) > 0 {
			return true
		}
	}
	return false
}

func reportOf(ix *index, c model.Collection, r model.Request, versions []model.RequestVersion, src Contracts) (Report, diag.List) {
	rep := Report{Collection: c.Manifest.Name, CollectionID: c.Manifest.ID, Request: r.Path, RequestID: r.ID, Status: Compatible}
	var diags diag.List
	for _, v := range versions {
		u, d := usageOf(ix, c, r, v, src)
		diags = append(diags, d...)
		rep.Versions = append(rep.Versions, u)
		rep.Status = worst(rep.Status, u.Status)
	}
	return rep, diags
}

func usageOf(ix *index, c model.Collection, r model.Request, v model.RequestVersion, src Contracts) (VersionUsage, diag.List) {
	contract, diags := src.Contract(c, r, v.Label)
	u := VersionUsage{Version: v.Label, Contract: ContractInfo{Source: contract.Source, Files: contract.Files}, Consumers: []Consumer{}, Status: Compatible}
	for _, cons := range consumers(ix, c, r, v) {
		cons = checked(cons, contract)
		u.Consumers = append(u.Consumers, cons)
		u.Status = worst(u.Status, cons.Status)
	}
	sortConsumers(u.Consumers)
	return u, diags
}

// Warnings turns broken reads into validate warnings (FLW-CONTRACT-001,
// FLW-CONTRACT-002), each at the reading template. Unknown extract names
// are left to validate's FLW-NODE-010.
func Warnings(reports []Report) diag.List {
	var out diag.List
	for _, rep := range reports {
		for _, u := range rep.Versions {
			for _, c := range u.Consumers {
				out = append(out, consumerWarnings(rep, u, c)...)
			}
		}
	}
	return unique(out)
}

func consumerWarnings(rep Report, u VersionUsage, c Consumer) diag.List {
	var out diag.List
	for _, r := range c.Reads {
		if r.Status != Broken || r.Kind == ReadExtract {
			continue
		}
		at := diag.Location{File: r.File, Path: r.Path}
		if r.Kind == ReadInput {
			out = append(out, diag.New(diag.CodeContractInput, at, map[string]string{"node": r.Reader, "name": r.Target, "request": rep.Request, "version": u.Version}))
			continue
		}
		out = append(out, diag.New(diag.CodeContractBroken, at, map[string]string{
			"reader": readerName(c, r), "read": r.Expr + " (" + r.Target + ")", "request": rep.Request, "version": u.Version,
			"source": strings.TrimPrefix(r.Reason, "not in "),
		}))
	}
	return out
}

// readerName names who reads, for messages.
func readerName(c Consumer, r Read) string {
	switch {
	case r.Reader != "":
		return `node "` + r.Reader + `"`
	case c.Kind == KindRequest:
		return "the request"
	case c.Flow != "":
		return "flow " + c.Flow
	}
	return "hook " + c.Hook
}

// unique drops repeated diagnostics: an unreadable example is read once
// per version of its request.
func unique(l diag.List) diag.List {
	seen := map[string]bool{}
	var out diag.List
	for _, d := range l {
		key := fmt.Sprint(d.Code, d.File, d.Path, d.Args) // fmt prints maps in key order
		if !seen[key] {
			seen[key] = true
			out = append(out, d)
		}
	}
	return out
}
