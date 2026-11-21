// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Column is one column of a table.
type Column struct {
	Name string
	Type string // INTEGER, REAL or TEXT: what the column is stored as when a query runs
}

// TableInfo is one table a query over data files can read: what to write after FROM.
type TableInfo struct {
	Name    string
	File    string // the file it comes from (its base name)
	Sheet   string // the sheet of an Excel file
	Columns []Column
	Rows    int
}

// Describer is a Source that can list its tables without running a query.
type Describer interface {
	Tables(ctx context.Context, q Query) ([]TableInfo, error)
}

// Tables lists the tables the driver's source offers for q's server, or an error when the driver cannot say (a
// database source does not list its tables here).
//
// Before: a data file request showed no table, sheet or column names, so the person had to know them to write
// `FROM sales`.
func (r Registry) Tables(ctx context.Context, q Query) ([]TableInfo, error) {
	source, ok := r[q.Driver]
	if !ok {
		return nil, fmt.Errorf("%w %q", ErrUnknownDriver, q.Driver)
	}
	d, ok := source.(Describer)
	if !ok {
		return nil, fmt.Errorf("tables of a %s source cannot be listed here", q.Driver)
	}
	return d.Tables(ctx, q)
}

// Tables reads each file once and describes its tables as a query would load them.
func (s fileSrc) Tables(ctx context.Context, q Query) ([]TableInfo, error) {
	files, err := dataFiles(q, s.ext)
	if err != nil {
		return nil, err
	}
	var out []TableInfo
	for _, file := range files {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if info, err := os.Stat(file); err == nil && info.Size() > maxFileBytes {
			return nil, fmt.Errorf("%s is larger than %d MB: use a database for data this big", filepath.Base(file), maxFileBytes>>20)
		}
		tables, err := s.read(file)
		if err != nil {
			return nil, err
		}
		for _, t := range tables {
			names, kinds := columnsOf(t)
			info := TableInfo{Name: t.name, File: filepath.Base(file), Sheet: t.sheet, Rows: len(t.rows)}
			for i, n := range names {
				info.Columns = append(info.Columns, Column{Name: n, Type: kinds[i]})
			}
			out = append(out, info)
		}
	}
	return out, nil
}
