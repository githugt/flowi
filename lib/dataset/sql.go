// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"database/sql"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// sqlSource runs a query on a database/sql pool opened by open. The pool is one connection that lives as long as the
// query: a request is one query, and nothing is kept between requests.
type sqlSource struct {
	open func(q Query) (*sql.DB, error)
	// readOnly asks the store for a read-only transaction. SQLite has none and is opened read-only instead.
	readOnly bool
}

func (s sqlSource) Query(ctx context.Context, q Query) (Result, error) {
	db, err := s.open(q)
	if err != nil {
		return Result{}, fmt.Errorf("opening the connection: %w", redactServer(err, q.Server))
	}
	defer func() { _ = db.Close() }() // a close failing after the rows are read changes nothing the caller can use
	limitPool(db, q.Timeout)

	// The first thing a pool does is connect, so this is where a wrong address or login shows.
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: s.readOnly})
	if err != nil {
		return Result{}, fmt.Errorf("connecting and starting a read-only transaction: %w", redactServer(err, q.Server))
	}
	// Never committed: a query reads, so there is nothing to keep, and a rollback ends it whatever happened.
	defer func() { _ = tx.Rollback() }()

	rows, err := tx.QueryContext(ctx, q.Statement, bind(q.Params)...)
	if err != nil {
		return Result{}, fmt.Errorf("running the query: %w", redactServer(err, q.Server))
	}
	defer func() { _ = rows.Close() }()
	return collect(rows, q.Limit)
}

// limitPool sets every pool limit explicitly: one connection, closed when idle or old, so nothing outlives the query.
func limitPool(db *sql.DB, lifetime time.Duration) {
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(lifetime)
	db.SetConnMaxIdleTime(lifetime)
}

// bind turns the params into the arguments of the query.
func bind(params []string) []any {
	args := make([]any, len(params))
	for i, p := range params {
		args[i] = p
	}
	return args
}

// collect reads up to limit rows.
func collect(rows *sql.Rows, limit int) (Result, error) {
	columns, err := rows.Columns()
	if err != nil {
		return Result{}, fmt.Errorf("reading the columns: %w", err)
	}
	types, err := rows.ColumnTypes()
	if err != nil {
		return Result{}, fmt.Errorf("reading the column types: %w", err)
	}
	res := Result{Columns: uniqueNames(columns)}
	for rows.Next() {
		if len(res.Rows) == limit {
			res.Truncated = true
			break
		}
		row, err := scanRow(rows, types)
		if err != nil {
			return res, err
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("reading the rows: %w", err)
	}
	return res, nil
}

func scanRow(rows *sql.Rows, types []*sql.ColumnType) ([]any, error) {
	values := make([]any, len(types))
	targets := make([]any, len(types))
	for i := range values {
		targets[i] = &values[i]
	}
	if err := rows.Scan(targets...); err != nil {
		return nil, fmt.Errorf("reading a row: %w", err)
	}
	for i, v := range values {
		values[i] = plain(v, types[i].DatabaseTypeName())
	}
	return values, nil
}

// uniqueNames numbers the repeats of a column name: a, a_2, a_3.
func uniqueNames(names []string) []string {
	seen := map[string]int{}
	out := make([]string, len(names))
	for i, name := range names {
		seen[name]++
		out[i] = name
		if n := seen[name]; n > 1 {
			out[i] = name + "_" + strconv.Itoa(n)
		}
	}
	return out
}

// plain turns a value a driver returned into one that reads the same in every store and writes as JSON: text as a
// string, binary as base64, times as RFC 3339, whole numbers as integers. MySQL answers a query without parameters in
// text, so its numbers arrive as bytes and are read back by the column's type.
func plain(v any, typeName string) any {
	switch x := v.(type) {
	case []byte:
		return plainBytes(x, typeName)
	case time.Time:
		return x.UTC().Format(time.RFC3339Nano)
	}
	return v
}

func plainBytes(b []byte, typeName string) any {
	text := string(b)
	switch {
	case isWholeNumberType(typeName):
		if n, err := strconv.ParseInt(text, 10, 64); err == nil {
			return n
		}
		if n, err := strconv.ParseUint(text, 10, 64); err == nil {
			return n
		}
	case isFloatType(typeName):
		if f, err := strconv.ParseFloat(text, 64); err == nil {
			return f
		}
	case isBinaryType(typeName), !utf8.Valid(b):
		return base64.StdEncoding.EncodeToString(b)
	}
	return text
}

// MySQL names unsigned types "UNSIGNED INT"; the other stores name the same types without that.
func isWholeNumberType(name string) bool {
	switch strings.TrimPrefix(strings.ToUpper(name), "UNSIGNED ") {
	case "TINYINT", "SMALLINT", "MEDIUMINT", "INT", "BIGINT", "YEAR":
		return true
	}
	return false
}

// DECIMAL stays text: a float would round it.
func isFloatType(name string) bool {
	switch strings.ToUpper(name) {
	case "FLOAT", "DOUBLE":
		return true
	}
	return false
}

func isBinaryType(name string) bool {
	switch strings.ToUpper(name) {
	case "BLOB", "TINYBLOB", "MEDIUMBLOB", "LONGBLOB", "BINARY", "VARBINARY", "BIT", "BYTEA":
		return true
	}
	return false
}
