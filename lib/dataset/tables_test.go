// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// Before: a data file request showed no table or column names, so the person had to know them to write `FROM sales`.
func TestTables_Given_a_folder_of_CSV_files_Then_each_file_is_a_table_with_typed_columns_and_a_row_count(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "sales.csv"), []byte("id,amount,region\n1,9.5,eu\n2,3,us\n3,4,eu\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "my users.csv"), []byte("name,\nAda,x\n"), 0o600)

	got, err := Default().Tables(context.Background(), Query{Driver: "csv", Server: dir})

	if err != nil || len(got) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
	users, sales := got[0], got[1] // the folder is read in file name order
	names := func(t TableInfo) []string {
		var out []string
		for _, c := range t.Columns {
			out = append(out, c.Name+":"+c.Type)
		}
		return out
	}
	if sales.Name != "sales" || sales.File != "sales.csv" || sales.Rows != 3 || !slices.Equal(names(sales), []string{"id:INTEGER", "amount:REAL", "region:TEXT"}) {
		t.Fatalf("sales %+v", sales)
	}
	if users.Name != "my_users" || !slices.Equal(names(users), []string{"name:TEXT", "column_2:TEXT"}) {
		t.Fatalf("users %+v (a table name is made safe, a blank column is numbered)", users)
	}
}

func TestTables_Given_a_database_driver_or_an_unknown_one_Then_it_says_so(t *testing.T) {
	_, db := Default().Tables(context.Background(), Query{Driver: "postgres", Server: "x"})
	_, unknown := Default().Tables(context.Background(), Query{Driver: "nope"})

	if db == nil || !errors.Is(unknown, ErrUnknownDriver) {
		t.Fatalf("db %v unknown %v", db, unknown)
	}
}

func TestTables_Given_a_path_that_leaves_the_root_Then_it_is_refused_as_a_query_would_be(t *testing.T) {
	dir := t.TempDir()

	_, err := Default().Tables(context.Background(), Query{Driver: "csv", Server: "../elsewhere.csv", Root: dir})

	if err == nil {
		t.Fatal("a path outside the collection must be refused")
	}
}
