// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/format"
	"github.com/xuri/excelize/v2"
)

// loadedTable is a table as it was loaded: what the file held (table), and the column names and types it got in the
// in-memory copy.
type loadedTable struct {
	table
	cols  []string
	kinds []string
}

// loadedFile is one data file and the tables it was loaded as: one for CSV and Parquet, one for each sheet of Excel.
type loadedFile struct {
	path   string
	tables []loadedTable
}

// outTable is what a table holds after a query that wrote: every cell is the text the file had when the cell was not
// touched, and otherwise the value the query left (nil, int64, float64 or string).
type outTable struct {
	cols        []string
	colsChanged bool // the columns are not the ones loaded
	rows        [][]any
}

// tableChange is a loaded table and what it holds now.
type tableChange struct {
	loaded loadedTable
	out    outTable
}

// writeBack writes the tables that a committed query changed back to their files. A table nobody changed leaves its
// file alone (not even its modification time), and a table the query dropped or created is not written: only the
// tables loaded from a file go back to it.
func writeBack(db *sql.DB, files []loadedFile, write func(path string, changes []tableChange) error) error {
	for _, f := range files {
		var changes []tableChange
		for _, lt := range f.tables {
			out, changed, err := readBack(db, lt)
			if err != nil {
				return fmt.Errorf("reading %s back to write its changes: %w", filepath.Base(f.path), err)
			}
			if changed {
				changes = append(changes, tableChange{loaded: lt, out: out})
			}
		}
		if len(changes) == 0 {
			continue
		}
		if err := write(f.path, changes); err != nil {
			return fmt.Errorf("writing the changes to %s: %w", filepath.Base(f.path), err)
		}
	}
	return nil
}

// readBack is the table as the query left it, and whether that differs from what was loaded. A table that no longer
// exists is reported as unchanged.
func readBack(db *sql.DB, lt loadedTable) (outTable, bool, error) {
	rows, err := db.QueryContext(context.Background(), "SELECT rowid, * FROM "+quoteIdent(lt.name)+" ORDER BY rowid") //nolint:gosec // the name is a quoted identifier
	if err != nil {
		return outTable{}, false, nil //nolint:nilerr // the query dropped or renamed the table: there is nothing to write back
	}
	defer func() { _ = rows.Close() }()
	cols, err := rows.Columns()
	if err != nil {
		return outTable{}, false, err
	}
	out := outTable{cols: cols[1:]}
	out.colsChanged = !slices.Equal(out.cols, lt.cols)
	changed := out.colsChanged || false
	for rows.Next() {
		values := make([]any, len(cols))
		targets := make([]any, len(cols))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := rows.Scan(targets...); err != nil {
			return outTable{}, false, err
		}
		rowid, _ := values[0].(int64)
		row := make([]any, len(cols)-1)
		for c := range row {
			now := plainCell(values[c+1])
			row[c] = now
			if !out.colsChanged && rowid >= 1 && int(rowid) <= len(lt.rows) && c < len(lt.rows[rowid-1]) {
				if orig := lt.rows[rowid-1][c]; sameCell(orig, now, lt.kinds[c]) {
					row[c] = orig // untouched: keep exactly what the file had
				}
			}
		}
		out.rows = append(out.rows, row)
	}
	if err := rows.Err(); err != nil {
		return outTable{}, false, err
	}
	if len(out.rows) != len(lt.rows) {
		changed = true
	}
	for i := 0; !changed && i < len(out.rows); i++ {
		for c := range out.rows[i] {
			if !equalCells(out.rows[i][c], lt.rows[i][c]) {
				changed = true
				break
			}
		}
	}
	return out, changed, rows.Err()
}

// plainCell is a value the SQLite driver returned as a Go value the writers handle.
func plainCell(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

// sameCell is whether the loaded cell orig and the value now in its place are the same: the cell was not changed.
func sameCell(orig, now any, kind string) bool {
	if isEmpty(orig) {
		return now == nil
	}
	return equalCells(cellValue(orig, kind), now)
}

// equalCells compares two cells; numbers are equal by value whatever their Go type.
func equalCells(a, b any) bool {
	if fa, ok := asFloat(a); ok {
		fb, ok := asFloat(b)
		return ok && fa == fb
	}
	return a == b || (isEmpty(a) && isEmpty(b))
}

func asFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case float32:
		return float64(x), true
	case float64:
		return x, true
	}
	return 0, false
}

// cellText is a cell as a CSV field.
func cellText(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == math.Trunc(x) && math.Abs(x) < 1e15 {
			return strconv.FormatFloat(x, 'f', -1, 64)
		}
		return strconv.FormatFloat(x, 'g', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'g', -1, 32)
	}
	return fmt.Sprint(v)
}

// header is the first row to write: the file's own header cells while the columns are the ones it had, else the new
// column names.
func (t tableChange) header() []string {
	if !t.out.colsChanged {
		return t.loaded.table.cols
	}
	return t.out.cols
}

// replaceFile writes a file through a temporary file of the same folder and renames it over path, so a failure leaves
// the file as it was and never half written.
func replaceFile(path string, fill func(tmp string) error) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".flowi-*"+filepath.Ext(path))
	if err != nil {
		return err
	}
	name := tmp.Name()
	_ = tmp.Close()
	defer func() { _ = os.Remove(name) }() // gone after a successful rename; cleans up after a failure
	if err := fill(name); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil {
		_ = os.Chmod(name, info.Mode().Perm())
	}
	return os.Rename(name, path)
}

func writeCSV(path string, changes []tableChange) error {
	t := changes[0]
	return replaceFile(path, func(tmp string) error {
		f, err := os.Create(tmp) //nolint:gosec // a temporary file next to the request's own file
		if err != nil {
			return err
		}
		w := csv.NewWriter(f)
		err = w.Write(t.header())
		for _, row := range t.out.rows {
			if err != nil {
				break
			}
			fields := make([]string, len(row))
			for i, v := range row {
				fields[i] = cellText(v)
			}
			err = w.Write(fields)
		}
		w.Flush()
		if err == nil {
			err = w.Error()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// writeXLSX updates the changed cells of the workbook in place, so what the query did not touch (formatting, formulas,
// other sheets) stays as it was.
func writeXLSX(path string, changes []tableChange) error {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	for _, t := range changes {
		sheet := t.loaded.sheet
		oldRows, oldCols := len(t.loaded.rows), len(t.loaded.table.cols)
		set := func(col, row int, v any) error {
			cell, err := excelize.CoordinatesToCellName(col+1, row)
			if err != nil {
				return err
			}
			if v == nil {
				v = ""
			}
			return f.SetCellValue(sheet, cell, v)
		}
		if t.out.colsChanged {
			for c := 0; c < max(len(t.out.cols), oldCols); c++ {
				var v any
				if c < len(t.out.cols) {
					v = t.out.cols[c]
				}
				if err := set(c, 1, v); err != nil {
					return err
				}
			}
		}
		for r, row := range t.out.rows {
			for c := 0; c < max(len(row), oldCols); c++ {
				var v, was any
				if c < len(row) {
					v = row[c]
				}
				if r < oldRows && c < len(t.loaded.rows[r]) {
					was = t.loaded.rows[r][c]
				}
				if r < oldRows && equalCells(v, was) {
					continue
				}
				if err := set(c, r+2, v); err != nil {
					return err
				}
			}
		}
		for r := oldRows + 1; r > len(t.out.rows)+1; r-- { // from the bottom, so the rows above keep their numbers
			if err := f.RemoveRow(sheet, r); err != nil {
				return err
			}
		}
	}
	return replaceFile(path, func(tmp string) error { return f.SaveAs(tmp) })
}

// writeParquet rewrites the whole file: Parquet cannot be edited in place. The file's own schema is kept, so a column
// keeps its type; a change a column of that type cannot hold is refused before anything is written.
func writeParquet(path string, changes []tableChange) error {
	t := changes[0]
	schema, err := parquetSchemaOf(path)
	if err != nil {
		return err
	}
	if t.out.colsChanged {
		return errors.New("a Parquet file keeps its columns: adding, dropping or renaming one is not supported")
	}
	fields := schema.Fields()
	rows := make([]map[string]any, len(t.out.rows))
	for r, row := range t.out.rows {
		m := make(map[string]any, len(fields))
		for c, field := range fields {
			v, err := parquetValue(field, row[c])
			if err != nil {
				return fmt.Errorf("column %q: %w", field.Name(), err)
			}
			m[field.Name()] = v
		}
		rows[r] = m
	}
	return replaceFile(path, func(tmp string) error {
		f, err := os.Create(tmp) //nolint:gosec // a temporary file next to the request's own file
		if err != nil {
			return err
		}
		w := parquet.NewWriter(f, schema)
		for _, m := range rows {
			if err = w.Write(m); err != nil {
				break
			}
		}
		if err == nil {
			err = w.Close()
		}
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		return err
	})
}

// parquetSchemaOf reads the schema of a Parquet file and closes the file again: on Windows a file that is open
// cannot be renamed over, and the rewrite replaces it.
func parquetSchemaOf(path string) (*parquet.Schema, error) {
	src, err := os.Open(path) //nolint:gosec // the path is the request's own server
	if err != nil {
		return nil, err
	}
	defer func() { _ = src.Close() }()
	info, err := src.Stat()
	if err != nil {
		return nil, err
	}
	pf, err := parquet.OpenFile(src, info.Size())
	if err != nil {
		return nil, err
	}
	return pf.Schema(), nil
}

// parquetValue converts a cell to what the field holds. Only plain columns are supported: numbers, booleans and text.
func parquetValue(field parquet.Field, v any) (any, error) {
	if !field.Leaf() {
		return nil, errors.New("a nested column cannot be written back")
	}
	if v == nil || (isEmpty(v) && field.Type().Kind() != parquet.ByteArray) {
		if !field.Optional() {
			return nil, errors.New("is required and cannot be empty")
		}
		return nil, nil
	}
	if lt := field.Type().LogicalType(); lt != nil {
		switch lt.Value.(type) {
		case *format.StringType, *format.IntType:
		default:
			return nil, fmt.Errorf("a column of this type (%s) cannot be written back", field.Type())
		}
	}
	switch field.Type().Kind() {
	case parquet.Boolean:
		switch x := v.(type) {
		case bool:
			return x, nil
		case int64:
			return x != 0, nil
		case string:
			b, err := strconv.ParseBool(strings.TrimSpace(x))
			return b, err
		}
	case parquet.Int32:
		if n, ok := wholeNumber(v); ok && n >= math.MinInt32 && n <= math.MaxInt32 {
			return int32(n), nil
		}
	case parquet.Int64:
		if n, ok := wholeNumber(v); ok {
			return n, nil
		}
	case parquet.Float:
		if f, ok := numberOf(v); ok {
			return float32(f), nil
		}
	case parquet.Double:
		if f, ok := numberOf(v); ok {
			return f, nil
		}
	case parquet.ByteArray:
		if s, ok := v.(string); ok {
			return s, nil
		}
		return cellText(v), nil
	}
	return nil, fmt.Errorf("the value %v does not fit a column of type %s", v, field.Type())
}

func wholeNumber(v any) (int64, bool) {
	if x, isText := v.(string); isText {
		n, err := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
		return n, err == nil
	}
	f, ok := asFloat(v)
	return int64(f), ok && f == math.Trunc(f)
}

func numberOf(v any) (float64, bool) {
	if s, ok := v.(string); ok {
		f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		return f, err == nil
	}
	return asFloat(v)
}
