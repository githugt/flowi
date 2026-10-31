// SPDX-License-Identifier: Apache-2.0

package dataset_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/githugt/flowi-lib/dataset"
	"github.com/githugt/flowi-lib/dataset/readonly"
	"github.com/githugt/flowi-lib/model"
)

// seededDatabase is a SQLite file in a temporary directory with a few rows; it returns the directory.
func seededDatabase(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, "shop.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, statement := range []string{
		"CREATE TABLE customers (id INTEGER PRIMARY KEY, name TEXT NOT NULL, balance REAL, note TEXT, avatar BLOB)",
		"INSERT INTO customers VALUES (1, 'Ada', 12.5, NULL, x'0102')",
		"INSERT INTO customers VALUES (2, 'Grace', 0, 'vip', NULL)",
		"INSERT INTO customers VALUES (3, 'Linus', -3.25, NULL, NULL)",
	} {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func runSQLite(t *testing.T, dir, statement string, params ...string) (dataset.Result, error) {
	t.Helper()
	return dataset.Default().Run(context.Background(), dataset.Query{
		Driver: model.DriverSQLite, Server: "shop.db", Root: dir, Statement: statement, Params: params,
	})
}

func TestRegistry_Run_Given_sqlite_should_return_typed_rows_in_column_order(t *testing.T) {
	dir := seededDatabase(t)

	res, err := runSQLite(t, dir, "SELECT id, name, balance, note, avatar FROM customers WHERE id = ?", "1")

	if err != nil {
		t.Fatal(err)
	}
	want := []any{int64(1), "Ada", 12.5, nil, "AQI="}
	if len(res.Rows) != 1 || !equalRows(res.Rows[0], want) {
		t.Fatalf("rows %v, want %v", res.Rows, want)
	}
	if got := res.Columns; len(got) != 5 || got[0] != "id" || got[4] != "avatar" {
		t.Fatalf("columns %v", got)
	}
}

func TestRegistry_Run_Given_more_rows_than_the_limit_should_cut_and_say_so(t *testing.T) {
	dir := seededDatabase(t)

	res, err := dataset.Default().Run(context.Background(), dataset.Query{
		Driver: model.DriverSQLite, Server: "shop.db", Root: dir, Statement: "SELECT id FROM customers ORDER BY id", Limit: 2,
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 2 || !res.Truncated {
		t.Fatalf("rows %d truncated %v", len(res.Rows), res.Truncated)
	}
}

func TestRegistry_Run_Given_exactly_the_limit_should_not_say_truncated(t *testing.T) {
	dir := seededDatabase(t)

	res, err := dataset.Default().Run(context.Background(), dataset.Query{
		Driver: model.DriverSQLite, Server: "shop.db", Root: dir, Statement: "SELECT id FROM customers", Limit: 3,
	})

	if err != nil || len(res.Rows) != 3 || res.Truncated {
		t.Fatalf("rows %d truncated %v err %v", len(res.Rows), res.Truncated, err)
	}
}

func TestRegistry_Run_Given_repeated_column_names_should_number_them(t *testing.T) {
	dir := seededDatabase(t)

	res, err := runSQLite(t, dir, "SELECT 1 AS n, 2 AS n, 3 AS n")

	if err != nil {
		t.Fatal(err)
	}
	if got := res.Columns; len(got) != 3 || got[0] != "n" || got[1] != "n_2" || got[2] != "n_3" {
		t.Fatalf("columns %v", got)
	}
}

func TestRegistry_Run_Given_a_statement_that_writes_should_refuse_it_before_sending(t *testing.T) {
	dir := seededDatabase(t)

	_, err := runSQLite(t, dir, "DELETE FROM customers")

	if !errors.Is(err, readonly.ErrNotReadOnly) {
		t.Fatalf("got %v", err)
	}
}

func TestSource_Given_sqlite_should_not_write_even_when_the_guard_is_skipped(t *testing.T) {
	dir := seededDatabase(t)
	source := dataset.Default()[model.DriverSQLite]

	_, err := source.Query(context.Background(), dataset.Query{
		Driver: model.DriverSQLite, Server: "shop.db", Root: dir, Statement: "DELETE FROM customers", Limit: 1,
	})
	after, countErr := runSQLite(t, dir, "SELECT count(*) FROM customers")

	if err == nil {
		t.Fatal("the delete ran")
	}
	if countErr != nil || after.Rows[0][0] != int64(3) {
		t.Fatalf("rows left %v (%v)", after.Rows, countErr)
	}
}

func TestRegistry_Run_Given_a_missing_file_should_fail_and_not_create_it(t *testing.T) {
	dir := t.TempDir()

	_, err := runSQLite(t, dir, "SELECT 1")
	_, statErr := filepath.Glob(filepath.Join(dir, "shop.db"))

	if err == nil {
		t.Fatal("a missing database was opened")
	}
	if matches, _ := filepath.Glob(filepath.Join(dir, "*")); len(matches) != 0 || statErr != nil {
		t.Fatalf("created %v", matches)
	}
}

func TestRegistry_Run_Given_an_unknown_driver_should_say_so(t *testing.T) {
	_, err := dataset.Default().Run(context.Background(), dataset.Query{Driver: "oracle", Statement: "SELECT 1"})

	if !errors.Is(err, dataset.ErrUnknownDriver) {
		t.Fatalf("got %v", err)
	}
}

func equalRows(a, b []any) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
