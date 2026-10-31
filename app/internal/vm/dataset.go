// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/model"
)

// The sections only a dataset request has.
const (
	SectionQuery  Section = "query"  // the statement
	SectionValues Section = "values" // the params the statement is sent with
)

// datasetVerb is what a dataset request shows where an HTTP request shows its method.
const datasetVerb = "SQL"

// IsDataset reports whether the selected version is a dataset query.
func (r *Request) IsDataset() bool { return r.Current().Dataset != nil }

// Dataset is a copy of the selected version's dataset block; the zero value for any other version.
func (r *Request) Dataset() model.Dataset {
	if d := r.Current().Dataset; d != nil {
		return *cloneDataset(d)
	}
	return model.Dataset{}
}

// EditDataset changes the dataset block of the selected version; it does nothing for another kind.
func (r *Request) EditDataset(f func(d *model.Dataset)) {
	if !r.IsDataset() {
		return
	}
	r.Edit(func(v *model.RequestVersion) { f(v.Dataset) })
}

// cloneDataset deep-copies a dataset block, so an edit never reaches the copy it came from.
func cloneDataset(d *model.Dataset) *model.Dataset {
	if d == nil {
		return nil
	}
	out := *d
	out.Params = slices.Clone(d.Params)
	return &out
}

// datasetConfigured is whether section s of a dataset version has content.
func datasetConfigured(d model.Dataset, s Section) bool {
	switch s {
	case SectionQuery:
		return strings.TrimSpace(d.Query) != ""
	case SectionValues:
		return len(d.Params) > 0
	}
	return false
}

// datasetTip is the target of a dataset version as a tooltip: its server, without the password.
func datasetTip(d model.Dataset) string { return model.RedactedTarget(d.Server) }
