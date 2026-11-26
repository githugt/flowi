// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
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
	// ServerExample is what the server box shows as its placeholder for the driver.
	ServerExample string `json:"serverExample"`
}

// driverLabels are the names a person knows each driver by.
func driverLabels() map[string]string {
	return map[string]string{
		model.DriverPostgres: "PostgreSQL", model.DriverMySQL: "MySQL", model.DriverMariaDB: "MariaDB", model.DriverSQLite: "SQLite",
		model.FormatCSV: "CSV files", model.FormatXLSX: "Excel files", model.FormatParquet: "Parquet files",
	}
}

// driverChoices lists the drivers in the order a picker shows them.
func driverChoices() []DriverChoice {
	labels := driverLabels()
	out := []DriverChoice{}
	for _, id := range append(model.DatasetDrivers(), model.DataFileFormats()...) {
		out = append(out, DriverChoice{ID: id, Label: labels[id]})
	}
	return out
}

// databasePromptChoices are the databases as the choices of a prompt that asks which one to query.
func databasePromptChoices() []Choice { return promptChoices(model.DatasetDrivers()) }

// dataFilePromptChoices are the file formats as the choices of a prompt that asks what kind of file to query.
func dataFilePromptChoices() []Choice { return promptChoices(model.DataFileFormats()) }

func promptChoices(ids []string) []Choice {
	labels := driverLabels()
	out := []Choice{}
	for _, id := range ids {
		out = append(out, Choice{ID: id, Label: labels[id]})
	}
	return out
}

// datasetInfoOf is nil for a request that is not a dataset query.
func datasetInfoOf(r *vm.Request) *DatasetInfo {
	if !r.IsDataset() {
		return nil
	}
	b := r.Dataset()
	return &DatasetInfo{Block: b, Drivers: driverChoices(), ServerExample: model.DatasetServerExample(b.Driver)}
}

// SetDataset replaces the dataset block of the selected version.
func (s *RequestService) SetDataset(node string, d model.Dataset) {
	s.with(node, func(o *openRequest) { o.req.EditDataset(func(cur *model.Dataset) { *cur = d }) })
}

// PickDataPath asks for the file (or, with folder, the folder) a file or SQLite request reads, and returns it as the
// request should keep it: relative to its collection when it lies inside, so the request works for whoever clones
// the collection, else the full path. "" means the user cancelled.
func (s *RequestService) PickDataPath(node string, folder bool) (string, error) {
	root := ""
	s.app.actor.Do(func() {
		if o, ok := s.app.requests.reqs[node]; ok {
			for _, c := range s.app.workspace.Collections() {
				if c.Manifest.ID == o.req.Target().Collection {
					root = c.Root
				}
			}
		}
	})
	var picked string
	var err error
	if folder {
		picked, err = s.app.pickers.pickFolder("Choose the folder of data files")
	} else {
		picked, err = s.app.pickers.pickFile("Choose the data file")
	}
	if err != nil || picked == "" {
		return "", err
	}
	return relativeInside(root, picked), nil
}

// relativeInside is path relative to root when it lies inside root, else path as given.
func relativeInside(root, path string) string {
	if root == "" {
		return path
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(rel)
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

// DataColumnView is one column of a table of a data file.
type DataColumnView struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// DataTableView is one table a data file request can read, with the statement that looks at it.
type DataTableView struct {
	Name    string           `json:"name"`
	File    string           `json:"file"`
	Sheet   string           `json:"sheet,omitempty"`
	Rows    int              `json:"rows"`
	Columns []DataColumnView `json:"columns"`
	Select  string           `json:"select"` // SELECT * FROM name LIMIT 100
}

// DataTablesView is the tables of a data file request, or why they cannot be shown.
type DataTablesView struct {
	Tables []DataTableView `json:"tables"`
	Note   string          `json:"note,omitempty"`
}

// dataTablesLimit is how many rows the statement a table chip inserts asks for.
const dataTablesLimit = 100

// DataTables lists the tables, columns and row counts of the files the selected data file request reads, for the chip
// row above its statement. The files are read off the actor. A server with a template, a missing path or a database
// request gives no tables and a note, never an error: the chips are help, not a requirement.
func (s *RequestService) DataTables(node string) DataTablesView {
	var collection model.ID
	var d model.Dataset
	var file bool
	s.with(node, func(o *openRequest) {
		collection, d, file = o.req.Target().Collection, o.req.Dataset(), o.req.IsDataFile()
	})
	out := DataTablesView{Tables: []DataTableView{}}
	if !file {
		return out
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataset.DefaultTimeout)
	defer cancel()
	tables, err := s.app.backend.DataFileTables(ctx, collection, d.Driver, d.Server)
	if err != nil {
		out.Note = vm.ErrorText(err)
		return out
	}
	for _, t := range tables {
		v := DataTableView{Name: t.Name, File: t.File, Sheet: t.Sheet, Rows: t.Rows, Columns: []DataColumnView{}, Select: fmt.Sprintf("SELECT * FROM %s LIMIT %d", t.Name, dataTablesLimit)}
		for _, c := range t.Columns {
			v.Columns = append(v.Columns, DataColumnView{Name: c.Name, Type: c.Type})
		}
		out.Tables = append(out.Tables, v)
	}
	return out
}
