// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/parquet-go/parquet-go"
	"github.com/xuri/excelize/v2"
)

// fileSource answers SQL over data files. The server is one file or a folder (one table for each file of the driver's
// kind in it). The files are loaded into a private in-memory SQLite database for the one query. A read-only query
// changes nothing and keeps nothing. A query that may write runs on that copy, and then the tables it changed are
// written back to their files (see writeBack). A column whose every value is a whole number or a number is stored as
// one, otherwise as text; an empty cell is NULL.
func fileSource(ext string, read func(path string) ([]table, error), write func(path string, changes []tableChange) error) Source {
	return fileSrc{ext: ext, read: read, write: write}
}

func csvSource() Source     { return fileSource(".csv", readCSV, writeCSV) }
func xlsxSource() Source    { return fileSource(".xlsx", readXLSX, writeXLSX) }
func parquetSource() Source { return fileSource(".parquet", readParquet, writeParquet) }

// fileSrc is the Source of one kind of data file.
type fileSrc struct {
	ext   string
	read  func(path string) ([]table, error)
	write func(path string, changes []tableChange) error
}

func (s fileSrc) Query(ctx context.Context, q Query) (Result, error) {
	var files []loadedFile
	inner := sqlSource{
		open:  func(q Query) (*sql.DB, error) { return openFiles(q, s.ext, s.read, &files) },
		after: func(db *sql.DB) error { return writeBack(db, files, s.write) },
	}
	return inner.Query(ctx, q)
}

// table is one table to load: the first row of a CSV file or sheet names its columns.
type table struct {
	name  string
	sheet string // the sheet an Excel table came from
	cols  []string
	rows  [][]any // strings (typed by what they look like) or Go numbers, bools and times from typed formats
}

// maxFileBytes is the most a query loads: a larger file is a job for a database.
const maxFileBytes = 256 << 20

func openFiles(q Query, ext string, read func(string) ([]table, error), loaded *[]loadedFile) (*sql.DB, error) {
	files, err := dataFiles(q, ext)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", "file:filedata?mode=memory&cache=private")
	if err != nil {
		return nil, fmt.Errorf("preparing the tables: %w", err)
	}
	db.SetMaxOpenConns(1) // the private in-memory database lives in this one connection
	for _, file := range files {
		if info, err := os.Stat(file); err == nil && info.Size() > maxFileBytes {
			_ = db.Close()
			return nil, fmt.Errorf("%s is larger than %d MB: use a database for data this big", filepath.Base(file), maxFileBytes>>20)
		}
		tables, err := read(file)
		if err != nil {
			_ = db.Close()
			return nil, err
		}
		lf := loadedFile{path: file}
		for _, t := range tables {
			lt, err := loadTable(db, t, filepath.Base(file))
			if err != nil {
				_ = db.Close()
				return nil, err
			}
			lf.tables = append(lf.tables, lt)
		}
		*loaded = append(*loaded, lf)
	}
	return db, nil
}

// dataFiles are the files a query reads: the server's file, or the files of its folder with the driver's extension.
func dataFiles(q Query, ext string) ([]string, error) {
	server := strings.TrimPrefix(q.Server, ext[1:]+"://")
	if strings.TrimSpace(server) == "" {
		return nil, fmt.Errorf("the server is empty: give the path of a %s file, or of a folder of them", ext)
	}
	if !filepath.IsAbs(server) && q.Root != "" {
		server = filepath.Join(q.Root, server)
	}
	server, err := confine(q, server)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(server)
	if err != nil {
		return nil, fmt.Errorf("the path %q cannot be read: %w", q.Server, errors.Unwrap(err))
	}
	if !info.IsDir() {
		return []string{server}, nil
	}
	matches, err := filepath.Glob(filepath.Join(server, "*"+ext))
	if err != nil || len(matches) == 0 {
		return nil, fmt.Errorf("the folder %q has no %s files", q.Server, ext)
	}
	return matches, nil
}

// tableName is a name as a table: anything but letters, digits and "_" becomes "_".
func tableName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 || (b.String()[0] >= '0' && b.String()[0] <= '9') {
		return "t_" + b.String()
	}
	return b.String()
}

func fileTable(file string) string {
	return tableName(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))
}

func quoteIdent(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }

// stringTable is a table of text cells whose first row is the column names.
func stringTable(name string, rows [][]string, file string) (table, error) {
	if len(rows) == 0 {
		return table{}, fmt.Errorf("%s is empty: its first row must name the columns", file)
	}
	t := table{name: name, cols: rows[0]}
	for _, r := range rows[1:] {
		row := make([]any, len(rows[0]))
		for c := range row {
			if c < len(r) {
				row[c] = r[c]
			}
		}
		t.rows = append(t.rows, row)
	}
	return t, nil
}

func readCSV(file string) ([]table, error) {
	f, err := os.Open(file) //nolint:gosec // the path is the request's own server
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", filepath.Base(file), errors.Unwrap(err))
	}
	defer func() { _ = f.Close() }()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s as CSV: %w", filepath.Base(file), err)
	}
	t, err := stringTable(fileTable(file), rows, filepath.Base(file))
	return []table{t}, err
}

// readXLSX makes a table of each sheet that has cells; cells are read as the sheet shows them.
func readXLSX(file string) ([]table, error) {
	f, err := excelize.OpenFile(file)
	if err != nil {
		return nil, fmt.Errorf("reading %s as an Excel workbook: %w", filepath.Base(file), err)
	}
	defer func() { _ = f.Close() }()
	var out []table
	for _, sheet := range f.GetSheetList() {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("reading sheet %q of %s: %w", sheet, filepath.Base(file), err)
		}
		if len(rows) == 0 {
			continue
		}
		t, err := stringTable(tableName(sheet), rows, filepath.Base(file))
		if err != nil {
			return nil, err
		}
		t.sheet = sheet
		out = append(out, t)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s has no sheet with cells: the first row of a sheet must name the columns", filepath.Base(file))
	}
	return out, nil
}

// readParquet makes one table of the file; nested values become JSON text.
func readParquet(file string) ([]table, error) {
	f, err := os.Open(file) //nolint:gosec // the path is the request's own server
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", filepath.Base(file), errors.Unwrap(err))
	}
	defer func() { _ = f.Close() }()
	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", filepath.Base(file), err)
	}
	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return nil, fmt.Errorf("reading %s as Parquet: %w", filepath.Base(file), err)
	}
	t := table{name: fileTable(file)}
	for _, field := range pf.Schema().Fields() {
		t.cols = append(t.cols, field.Name())
	}
	reader := parquet.NewReader(pf)
	defer func() { _ = reader.Close() }()
	for {
		m := map[string]any{}
		if err := reader.Read(&m); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("reading %s as Parquet: %w", filepath.Base(file), err)
		}
		row := make([]any, len(t.cols))
		for c, name := range t.cols {
			row[c] = scalar(m[name])
		}
		t.rows = append(t.rows, row)
	}
	return []table{t}, nil
}

// scalar keeps numbers, bools and text and turns anything else (times, bytes, lists, groups) into text.
func scalar(v any) any {
	switch x := v.(type) {
	case nil, string, bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return x
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case []byte:
		return string(x)
	}
	if b, err := json.Marshal(v); err == nil {
		return string(b)
	}
	return fmt.Sprint(v)
}

// columnsOf are a table's column names as they are loaded (blank ones numbered, repeats suffixed) and the type each
// column gets.
func columnsOf(t table) (cols, kinds []string) {
	cols = make([]string, len(t.cols))
	for i, n := range t.cols {
		cols[i] = strings.TrimSpace(n)
		if cols[i] == "" {
			cols[i] = "column_" + strconv.Itoa(i+1)
		}
	}
	cols = uniqueNames(cols)
	kinds = make([]string, len(cols))
	for c := range cols {
		kinds[c] = columnKind(t.rows, c)
	}
	return cols, kinds
}

func loadTable(db *sql.DB, t table, file string) (loadedTable, error) {
	ctx := context.Background() // an in-memory database: nothing here waits on the network
	cols, kinds := columnsOf(t)
	var ddl strings.Builder
	ddl.WriteString("CREATE TABLE " + quoteIdent(t.name) + " (")
	for c, n := range cols {
		if c > 0 {
			ddl.WriteString(", ")
		}
		ddl.WriteString(quoteIdent(n) + " " + kinds[c])
	}
	ddl.WriteString(")")
	if _, err := db.ExecContext(ctx, ddl.String()); err != nil {
		return loadedTable{}, fmt.Errorf("making the table %q of %s: %w", t.name, file, err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return loadedTable{}, fmt.Errorf("loading %s: %w", file, err)
	}
	marks := strings.TrimSuffix(strings.Repeat("?,", len(cols)), ",")
	stmt, err := tx.PrepareContext(ctx, "INSERT INTO "+quoteIdent(t.name)+" VALUES ("+marks+")") //nolint:gosec // the name is a quoted identifier, the values are bound
	if err != nil {
		_ = tx.Rollback()
		return loadedTable{}, fmt.Errorf("loading %s: %w", file, err)
	}
	defer func() { _ = stmt.Close() }()
	for _, row := range t.rows {
		args := make([]any, len(cols))
		for c := range cols {
			args[c] = cellValue(row[c], kinds[c])
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			_ = tx.Rollback()
			return loadedTable{}, fmt.Errorf("loading %s: %w", file, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return loadedTable{}, fmt.Errorf("loading %s: %w", file, err)
	}
	return loadedTable{table: t, cols: cols, kinds: kinds}, nil
}

// isEmpty is a cell that holds nothing: nil or blank text.
func isEmpty(v any) bool {
	s, ok := v.(string)
	return v == nil || (ok && strings.TrimSpace(s) == "")
}

// numberKind says INTEGER or REAL for a number, bool or text that reads as one, "" for anything else.
func numberKind(v any) string {
	switch x := v.(type) {
	case bool, int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return "INTEGER"
	case float32, float64:
		return "REAL"
	case string:
		s := strings.TrimSpace(x)
		if _, err := strconv.ParseInt(s, 10, 64); err == nil {
			return "INTEGER"
		}
		if _, err := strconv.ParseFloat(s, 64); err == nil {
			return "REAL"
		}
	}
	return ""
}

// columnKind is INTEGER when every value of the column is a whole number, REAL when every one is a number, else TEXT.
func columnKind(rows [][]any, c int) string {
	kind, seen := "INTEGER", false
	for _, row := range rows {
		if isEmpty(row[c]) {
			continue
		}
		seen = true
		switch numberKind(row[c]) {
		case "INTEGER":
		case "REAL":
			kind = "REAL"
		default:
			return "TEXT"
		}
	}
	if !seen {
		return "TEXT"
	}
	return kind
}

func cellValue(v any, kind string) any {
	if isEmpty(v) {
		return nil
	}
	switch kind {
	case "INTEGER":
		switch x := v.(type) {
		case bool:
			if x {
				return int64(1)
			}
			return int64(0)
		case string:
			n, _ := strconv.ParseInt(strings.TrimSpace(x), 10, 64)
			return n
		}
		return v
	case "REAL":
		if s, ok := v.(string); ok {
			f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
			return f
		}
		return v
	}
	if s, ok := v.(string); ok {
		return s // text keeps the spaces it was written with
	}
	return fmt.Sprint(v)
}
