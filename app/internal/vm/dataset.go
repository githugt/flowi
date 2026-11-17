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

// datasetVerbOf is the verb of one dataset request: its database (POSTGRES, SQLITE …), as the editor's own pill
// says, so the tab and the tree do not call the same request SQL. A request with no driver yet is SQL.
func datasetVerbOf(d model.Dataset) string {
	if d.Driver == "" {
		return datasetVerb
	}
	return strings.ToUpper(d.Driver)
}

// IsDataset reports whether the selected version is a dataset query.
// IsDataset reports whether the selected version is a SQL query: of a database or of data files. The two are kinds of
// their own in the library (model.Dataset and model.DataFile); the screen that edits a query is the same for both.
func (r *Request) IsDataset() bool { return r.Current().SQL() != nil }

// IsDataFile reports whether the selected version is a query over data files (CSV, Excel, Parquet).
func (r *Request) IsDataFile() bool { return r.Current().File != nil }

// Dataset is a copy of the selected version's query block (a data file query as its SQL equivalent: the format is the
// driver and the path is the server); the zero value for any other version.
func (r *Request) Dataset() model.Dataset {
	if d := r.Current().SQL(); d != nil {
		return *cloneDataset(d)
	}
	return model.Dataset{}
}

// EditDataset changes the query block of the selected version; it does nothing for another kind. A data file query
// stays one: the format and the path are written back to its file block.
func (r *Request) EditDataset(f func(d *model.Dataset)) {
	if !r.IsDataset() {
		return
	}
	r.Edit(func(v *model.RequestVersion) {
		if v.File != nil {
			d := v.File.AsDataset()
			f(&d)
			*v.File = model.DataFileOf(d)
			return
		}
		f(v.Dataset)
	})
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

// cloneDataFile deep-copies a file block, so an edit never reaches the copy it came from.
func cloneDataFile(f *model.DataFile) *model.DataFile {
	if f == nil {
		return nil
	}
	out := *f
	out.Params = slices.Clone(f.Params)
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
