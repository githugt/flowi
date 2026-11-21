// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
)

func TestNewDatasetRequest_should_create_a_dataset_version_that_loads_back(t *testing.T) {
	w := demoWorkspace(t)
	collection := w.Collections()[0].Manifest.ID

	target, err := w.NewDatasetRequest(collection, "customers/list", model.DriverPostgres)

	if err != nil {
		t.Fatal(err)
	}
	_, r, err := w.requestFor(target)
	if err != nil {
		t.Fatal(err)
	}
	d := r.Versions[0].Dataset
	if d == nil || d.Driver != model.DriverPostgres || d.Server == "" || d.Query == "" || r.Versions[0].Family() != model.FamilyDataset {
		t.Fatalf("version %+v", r.Versions[0])
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("a new dataset request must validate: %v", diags)
	}
}

func TestNewDatasetRequest_should_start_each_driver_with_its_own_server_and_placeholder(t *testing.T) {
	cases := map[string][2]string{
		model.DriverPostgres: {"postgres://", "$1"},
		model.DriverMySQL:    {"mysql://", "?"},
		model.DriverMariaDB:  {"mariadb://", "?"},
		model.DriverSQLite:   {"", "?"},
	}
	for driver, want := range cases {
		t.Run(driver, func(t *testing.T) {
			w := demoWorkspace(t)

			target, err := w.NewDatasetRequest(w.Collections()[0].Manifest.ID, "a/b", driver)

			if err != nil {
				t.Fatal(err)
			}
			_, r, _ := w.requestFor(target)
			d := r.Versions[0].Dataset
			if len(d.Server) < len(want[0]) || d.Server[:len(want[0])] != want[0] || !strings.Contains(d.Query, want[1]) {
				t.Fatalf("server %q query %q", d.Server, d.Query)
			}
		})
	}
}

func TestNewDatasetRequest_Given_an_unknown_driver_should_refuse(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.NewDatasetRequest(w.Collections()[0].Manifest.ID, "a/b", "oracle")

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeDatasetRequest {
		t.Fatalf("got %v", err)
	}
}

func TestSendRequest_Given_a_sqlite_dataset_request_should_return_its_rows(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	db, err := sql.Open("sqlite", filepath.Join(c.Root, "shop.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{"CREATE TABLE customers (id INTEGER, name TEXT)", "INSERT INTO customers VALUES (1, 'Ada'), (2, 'Grace')"} {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()
	target, err := w.NewDatasetRequest(c.Manifest.ID, "customers/by_id", model.DriverSQLite)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions[0].Dataset.Server = "shop.db"
	r.Versions[0].Dataset.Query = "SELECT name FROM customers WHERE id = ?"
	r.Versions[0].Dataset.Params = []string{"{{id}}"}
	r.Versions[0].Assert = []string{`row_count == 1`, `rows[0].name == "Grace"`}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"id": "2"}})
	if err != nil {
		t.Fatal(err)
	}
	res := s.Wait()

	if res.Status != engine.StatusPassed || res.Record == nil || res.Record.Kind != engine.KindDataset {
		t.Fatalf("status %s diags %v record %+v", res.Status, res.Diags, res.Record)
	}
}

// Rows can hold private data, so none of their values may end up in a file the collection keeps (a learned schema, an
// example, a stored response). Today a dataset record learns no schema at all; this pins that no value is written
// until one that keeps names and types only is built.
func TestSendRequest_Given_a_dataset_request_Then_no_row_value_is_written_to_the_collection(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	db, err := sql.Open("sqlite", filepath.Join(c.Root, "shop.db"))
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{"CREATE TABLE customers (id INTEGER, name TEXT)", "INSERT INTO customers VALUES (1, 'Zebediah-the-private')"} {
		if _, err := db.ExecContext(t.Context(), statement); err != nil {
			t.Fatal(err)
		}
	}
	_ = db.Close()
	target, err := w.NewDatasetRequest(c.Manifest.ID, "customers/all", model.DriverSQLite)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions[0].Dataset.Server, r.Versions[0].Dataset.Query, r.Versions[0].Dataset.Params = "shop.db", "SELECT id, name FROM customers", nil
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	res := s.Wait()

	if res.Status != engine.StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	var leaks []string
	_ = filepath.WalkDir(c.Root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Base(path) == "shop.db" {
			return nil
		}
		if data, readErr := os.ReadFile(path); readErr == nil && strings.Contains(string(data), "Zebediah-the-private") {
			leaks = append(leaks, path)
		}
		return nil
	})
	if len(leaks) > 0 {
		t.Fatalf("a row value is in %v", leaks)
	}
}

func TestTestConnection_Given_a_sqlite_request_Then_it_connects_without_running_the_saved_statement(t *testing.T) {
	// Before: a wrong database path showed only after the whole request ran.
	w := demoWorkspace(t)
	c := w.Collections()[0]
	db, err := sql.Open("sqlite", filepath.Join(c.Root, "shop.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(t.Context(), "CREATE TABLE customers (id INTEGER)"); err != nil {
		t.Fatal(err)
	}
	_ = db.Close()
	target, err := w.NewDatasetRequest(c.Manifest.ID, "customers/wipe", model.DriverSQLite)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	v := r.Versions[0]
	v.Dataset.Server, v.Dataset.Query, v.Dataset.Params, v.Dataset.Writes = "shop.db", "DROP TABLE customers", nil, true

	got, err := w.TestConnection(t.Context(), target, v, SendOptions{Env: "local"})
	rows := sqliteRowCount(t, filepath.Join(c.Root, "shop.db"))

	if err != nil || got.Status != engine.ProbeOK || got.Target != "shop.db" {
		t.Fatalf("probe %+v err %v", got, err)
	}
	if rows != 0 {
		t.Fatalf("the table is gone or changed: %d", rows)
	}
}

func sqliteRowCount(t *testing.T, file string) int {
	t.Helper()
	db, err := sql.Open("sqlite", file)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var n int
	if err := db.QueryRowContext(t.Context(), "SELECT count(*) FROM customers").Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestTestConnection_Given_a_missing_sqlite_file_Then_it_fails_and_does_not_create_it(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	target, err := w.NewDatasetRequest(c.Manifest.ID, "customers/list", model.DriverSQLite)
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	v := r.Versions[0]
	v.Dataset.Server = "nope.db"

	got, err := w.TestConnection(t.Context(), target, v, SendOptions{Env: "local"})

	if err != nil || got.Status != engine.ProbeFailed {
		t.Fatalf("probe %+v err %v", got, err)
	}
	if _, statErr := os.Stat(filepath.Join(c.Root, "nope.db")); statErr == nil {
		t.Fatal("the test created the database file")
	}
}
