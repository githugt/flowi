// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/xuri/excelize/v2"
)

func runWrite(t *testing.T, driver, server, root, statement string, params ...string) (Result, error) {
	t.Helper()
	return Default().Run(context.Background(), Query{Driver: driver, Server: server, Root: root, Statement: statement, Params: params, AllowWrites: true})
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCSV_Given_a_writing_query_Then_the_file_is_rewritten_and_untouched_cells_keep_their_text(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "people.csv")
	_ = os.WriteFile(path, []byte("name,code,score\nAda,007,1.50\nBob,042,2\n"), 0o600)

	_, err := runWrite(t, "csv", "people.csv", dir, "UPDATE people SET score = ? WHERE name = 'Bob'", "9.5")

	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, path); got != "name,code,score\nAda,007,1.50\nBob,042,9.5\n" {
		t.Fatalf("file:\n%s", got)
	}
}

func TestCSV_Given_insert_and_delete_Then_rows_are_added_and_removed_and_quoted_when_needed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.csv")
	_ = os.WriteFile(path, []byte("id,note\n1,a\n2,b\n"), 0o600)

	if _, err := runWrite(t, "csv", "p.csv", dir, "INSERT INTO p VALUES (3, 'x, \"y\"')"); err != nil {
		t.Fatal(err)
	}
	if _, err := runWrite(t, "csv", "p.csv", dir, "DELETE FROM p WHERE id = 1"); err != nil {
		t.Fatal(err)
	}

	if got := readFile(t, path); got != "id,note\n2,b\n3,\"x, \"\"y\"\"\"\n" {
		t.Fatalf("file:\n%s", got)
	}
}

func TestCSV_Given_a_query_that_changes_nothing_Then_the_file_is_not_touched(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.csv")
	_ = os.WriteFile(path, []byte("id\r\n1\r\n"), 0o600) // CRLF would be lost by a rewrite
	before, _ := os.Stat(path)

	_, err := runWrite(t, "csv", "p.csv", dir, "UPDATE p SET id = 1 WHERE id = 1")

	after, _ := os.Stat(path)
	if err != nil || readFile(t, path) != "id\r\n1\r\n" || !after.ModTime().Equal(before.ModTime()) {
		t.Fatalf("err %v file %q", err, readFile(t, path))
	}
}

func TestCSV_Given_a_read_only_query_Then_a_write_is_refused_and_the_file_is_unchanged(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.csv")
	_ = os.WriteFile(path, []byte("id\n1\n"), 0o600)

	_, err := Default().Run(context.Background(), Query{Driver: "csv", Server: "p.csv", Root: dir, Statement: "DELETE FROM p"})

	if err == nil || readFile(t, path) != "id\n1\n" {
		t.Fatalf("err %v file %q", err, readFile(t, path))
	}
}

func TestCSV_Given_a_folder_Then_only_the_file_of_the_changed_table_is_rewritten(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.csv"), []byte("id\n1\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "b.csv"), []byte("id\r\n1\r\n"), 0o600)

	_, err := runWrite(t, "csv", dir, "", "UPDATE a SET id = 2")

	if err != nil || readFile(t, filepath.Join(dir, "a.csv")) != "id\n2\n" || readFile(t, filepath.Join(dir, "b.csv")) != "id\r\n1\r\n" {
		t.Fatalf("err %v", err)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, ".flowi-*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
}

func TestXLSX_Given_a_writing_query_Then_only_the_changed_cells_are_updated_and_other_sheets_stay(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.xlsx")
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "people")
	for i, r := range [][]any{{"id", "name"}, {1, "Ada"}, {2, "Bob"}, {3, "Cy"}} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetSheetRow("people", cell, &r)
	}
	_, _ = f.NewSheet("pets")
	_ = f.SetSheetRow("pets", "A1", &[]any{"pet"})
	_ = f.SetSheetRow("pets", "A2", &[]any{"Rex"})
	_ = f.SaveAs(path)

	_, err := runWrite(t, "xlsx", "book.xlsx", dir, "UPDATE people SET name = 'Bobby' WHERE id = 2")
	if err == nil {
		_, err = runWrite(t, "xlsx", "book.xlsx", dir, "DELETE FROM people WHERE id = 3")
	}

	if err != nil {
		t.Fatal(err)
	}
	got, _ := excelize.OpenFile(path)
	defer func() { _ = got.Close() }()
	people, _ := got.GetRows("people")
	pets, _ := got.GetRows("pets")
	if len(people) != 3 || people[2][1] != "Bobby" || people[1][1] != "Ada" || len(pets) != 2 || pets[1][0] != "Rex" {
		t.Fatalf("people %v pets %v", people, pets)
	}
}

func TestParquet_Given_a_writing_query_Then_the_file_is_rewritten_with_the_same_column_types(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.parquet")
	type row struct {
		Name  string  `parquet:"name"`
		Age   int64   `parquet:"age"`
		Score float64 `parquet:"score"`
		Note  *string `parquet:"note,optional"`
	}
	seed(t, path, []row{{"Ada", 36, 1.5, nil}, {"Bob", 41, 2, nil}})

	_, err := runWrite(t, "parquet", "p.parquet", dir, "UPDATE p SET age = age + 1, note = 'hi' WHERE name = 'Bob'")
	if err == nil {
		_, err = runWrite(t, "parquet", "p.parquet", dir, "INSERT INTO p (name, age, score) VALUES ('Cy', 5, 0.25)")
	}

	if err != nil {
		t.Fatal(err)
	}
	f, _ := os.Open(path)
	defer func() { _ = f.Close() }()
	rows, err := parquet.Read[row](f, mustSize(f))
	if err != nil || len(rows) != 3 || rows[1].Age != 42 || rows[1].Note == nil || *rows[1].Note != "hi" || rows[2].Name != "Cy" || rows[2].Score != 0.25 || rows[0].Note != nil {
		t.Fatalf("rows %+v err %v", rows, err)
	}
}

func TestParquet_Given_a_text_in_a_number_column_Then_nothing_is_written(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.parquet")
	type row struct {
		Age int64 `parquet:"age"`
	}
	seed(t, path, []row{{1}})
	before := readFile(t, path)

	_, err := runWrite(t, "parquet", "p.parquet", dir, "UPDATE p SET age = 'old'")

	if err == nil || !strings.Contains(err.Error(), "age") || readFile(t, path) != before {
		t.Fatalf("err %v", err)
	}
}

func seed[T any](t *testing.T, path string, rows []T) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := parquet.Write(f, rows); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func mustSize(f *os.File) int64 {
	info, _ := f.Stat()
	return info.Size()
}
