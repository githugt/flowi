// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/openapi"
	"github.com/githugt/flowi-lib/specoverlay"
)

// SpecImpact is what a changed API description does to the things that use it.
type SpecImpact struct {
	Spec string // the kept description it was compared with
	// Breakages are the consumers the change breaks, and only those: a consumer that was already broken
	// before (an input nothing fills, say) is not the change's doing and is left out.
	Breakages []Breakage
}

// Breakage is one consumer a changed description breaks.
type Breakage struct {
	Request  string            `json:"request"` // path of the request it uses
	Version  string            `json:"version"`
	Consumer analysis.Consumer `json:"consumer"` // its reads are only the ones the change broke
}

// Broken says whether the change breaks anything.
func (s SpecImpact) Broken() bool { return len(s.Breakages) > 0 }

// ImpactAgainst tells which flows, hooks and checks would break if the kept OpenAPI description called name
// (empty when the collection keeps one) were replaced by spec, a newer version of it. It imports spec in
// memory, with the overlays kept for the description, and writes nothing. Each request version in it is
// judged by the response schemas it now has; a request version the kept copy had and spec lacks is broken
// for everyone who uses it. Without a kept copy only what spec has can be judged, never what it dropped.
func (w *Workspace) ImpactAgainst(collection model.ID, name string, spec []byte) (SpecImpact, diag.List, error) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	c, ok := w.collection(collection)
	if !ok {
		return SpecImpact{}, nil, diagErr(diag.CodeWSUnknownCollection, map[string]string{"name": string(collection)})
	}
	kept, err := keptForImpact(c.Root, name)
	if err != nil {
		return SpecImpact{}, nil, err
	}
	fresh, err := importAgainst(kept, spec)
	if err != nil {
		return SpecImpact{}, nil, err
	}
	replaced, touched := replacedContracts(w.contracts(), kept, fresh)
	before, _ := analysis.Analyze(w.collections, w.contracts())
	after, diags := analysis.Analyze(w.collections, replaced)
	return SpecImpact{Spec: kept.Name, Breakages: newBreakages(before, after, touched)}, w.positioned(diags), nil
}

// newBreakages keeps what is broken after and was not broken before, for the requests the description
// mentions. A consumer of a removed request is always new: before, it had a request to call.
func newBreakages(before, after []analysis.Report, touched map[string]bool) []Breakage {
	was := brokenReads(before)
	var out []Breakage
	for _, rep := range after {
		if !touched[rep.Request] {
			continue
		}
		for _, v := range rep.Versions {
			for _, cons := range v.Consumers {
				if cons.Status != analysis.Broken {
					continue
				}
				if cons.Reason == "" {
					cons.Reads = slices.DeleteFunc(slices.Clone(cons.Reads), func(r analysis.Read) bool {
						return r.Status != analysis.Broken || was[readKey(rep.Request, v.Version, cons, r)]
					})
				}
				if cons.Reason != "" || len(cons.Reads) > 0 {
					out = append(out, Breakage{Request: rep.Request, Version: v.Version, Consumer: cons})
				}
			}
		}
	}
	return out
}

func brokenReads(reports []analysis.Report) map[string]bool {
	out := map[string]bool{}
	for _, rep := range reports {
		for _, v := range rep.Versions {
			for _, cons := range v.Consumers {
				for _, r := range cons.Reads {
					if r.Status == analysis.Broken {
						out[readKey(rep.Request, v.Version, cons, r)] = true
					}
				}
			}
		}
	}
	return out
}

// readKey names a read by where it is written and which request version it reads.
func readKey(request, version string, c analysis.Consumer, r analysis.Read) string {
	return strings.Join([]string{request, version, c.Kind, c.File, c.Flow, c.Node, c.Hook, r.Expr, r.Path, r.Reader}, "\x00")
}

// keptForImpact picks the kept description: the one called name, or the only one.
func keptForImpact(root, name string) (KeptSpec, error) {
	if name != "" {
		kept, ok := keptSpecNamed(root, name)
		if !ok {
			return KeptSpec{}, fmt.Errorf("%w: %q", ErrNoKeptSpec, name)
		}
		return kept, nil
	}
	specs, err := keptSpecs(root)
	if err != nil {
		return KeptSpec{}, err
	}
	if len(specs) != 1 {
		return KeptSpec{}, fmt.Errorf("%w: the collection keeps %d, so name the one to compare with", ErrNoKeptSpec, len(specs))
	}
	return specs[0], nil
}

// importAgainst imports spec as the kept description would be re-imported: with its overlays.
func importAgainst(kept KeptSpec, spec []byte) (openapi.Result, error) {
	overlays, edits, err := kept.readOverlays()
	if err != nil {
		return openapi.Result{}, err
	}
	if edits != nil {
		overlays = append(overlays, edits)
	}
	data, diags := specoverlay.Apply(spec, overlays)
	if diags.HasErrors() {
		return openapi.Result{}, diags.Err()
	}
	res, diags := openapi.Import(data, openapi.Options{NewID: func() model.ID { return "" }, Files: kept.refFiles()})
	if diags.HasErrors() {
		return openapi.Result{}, diags.Err()
	}
	return res, nil
}

// replacedContracts builds what the new description says for each request version it has, and marks the
// versions only the kept copy had as removed. touched is every request path either one mentions.
func replacedContracts(base analysis.Contracts, kept KeptSpec, fresh openapi.Result) (analysis.Contracts, map[string]bool) {
	by := map[string]analysis.Contract{}
	touched := map[string]bool{}
	for _, req := range fresh.Requests {
		touched[req.Path] = true
		for _, v := range req.Versions {
			by[analysis.ContractKey(req.Path, v.Label)] = contractOfSchemas(fresh.Schemas[req.Path][v.Label])
		}
	}
	for path, old := range previousImport(kept, true) {
		touched[path] = true
		for _, v := range old.Versions {
			key := analysis.ContractKey(path, v.Label)
			if _, still := by[key]; !still {
				by[key] = analysis.Contract{Removed: true}
			}
		}
	}
	return analysis.Replaced{Base: base, By: by}, touched
}

// contractOfSchemas is a contract made of the response schemas of one request version, by status.
func contractOfSchemas(byStatus map[string]any) analysis.Contract {
	c := analysis.Contract{}
	for _, status := range slices.Sorted(maps.Keys(byStatus)) {
		n, err := strconv.Atoi(status)
		if err != nil {
			continue // "default" and ranges such as 2XX say nothing about one status
		}
		c.Samples = append(c.Samples, analysis.Sample{Source: analysis.SourceSchema, Status: n, Schema: byStatus[status]})
	}
	if len(c.Samples) > 0 {
		c.Source = analysis.SourceSchema
	}
	return c
}
