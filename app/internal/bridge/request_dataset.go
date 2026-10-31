// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

// DriverChoice is a store a dataset request can query, as a picker lists it.
type DriverChoice struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

// DatasetInfo is what the screen of a dataset request needs besides its block.
type DatasetInfo struct {
	Block   model.Dataset  `json:"block"`
	Drivers []DriverChoice `json:"drivers"`
}

// driverLabels are the names a person knows each driver by.
func driverLabels() map[string]string {
	return map[string]string{
		model.DriverPostgres: "PostgreSQL", model.DriverMySQL: "MySQL", model.DriverMariaDB: "MariaDB", model.DriverSQLite: "SQLite",
	}
}

// driverChoices lists the drivers in the order a picker shows them.
func driverChoices() []DriverChoice {
	labels := driverLabels()
	out := []DriverChoice{}
	for _, id := range model.DatasetDrivers() {
		out = append(out, DriverChoice{ID: id, Label: labels[id]})
	}
	return out
}

// driverPromptChoices are the drivers as the choices of a prompt that asks which database to query.
func driverPromptChoices() []Choice {
	out := []Choice{}
	for _, d := range driverChoices() {
		out = append(out, Choice{ID: d.ID, Label: d.Label})
	}
	return out
}

// datasetInfoOf is nil for a request that is not a dataset query.
func datasetInfoOf(r *vm.Request) *DatasetInfo {
	if !r.IsDataset() {
		return nil
	}
	return &DatasetInfo{Block: r.Dataset(), Drivers: driverChoices()}
}

// SetDataset replaces the dataset block of the selected version.
func (s *RequestService) SetDataset(node string, d model.Dataset) {
	s.with(node, func(o *openRequest) { o.req.EditDataset(func(cur *model.Dataset) { *cur = d }) })
}

// TableView is the rows of a dataset response, for the table: one value per column, in column order, as the
// store returned them (a number, text, true or false, or null).
type TableView struct {
	Columns   []string `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"rowCount"`
	Truncated bool     `json:"truncated"` // the store had more rows than were kept
}

// tableOf is nil for a record that is not a dataset response.
func tableOf(rec *engine.Record) *TableView {
	if rec.Kind != engine.KindDataset {
		return nil
	}
	body, ok := rec.Body.JSON.(map[string]any)
	if !ok {
		return nil // the query failed: there is no result
	}
	columns := stringsOf(body["columns"])
	rows, _ := body["rows"].([]any)
	table := &TableView{Columns: columns, Rows: make([][]any, 0, len(rows))}
	table.RowCount, table.Truncated = rec.Rows()
	for _, r := range rows {
		object, _ := r.(map[string]any)
		cells := make([]any, len(columns))
		for i, name := range columns {
			cells[i] = object[name]
		}
		table.Rows = append(table.Rows, cells)
	}
	return table
}

func stringsOf(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, item := range list {
		text, _ := item.(string)
		out = append(out, text)
	}
	return out
}
