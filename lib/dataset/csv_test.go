// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/parquet-go/parquet-go"
	"github.com/xuri/excelize/v2"
)

func TestCSV_Given_a_file_When_queried_Then_rows_come_back_with_typed_columns(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "people.csv"), []byte("name,age,score\nAda,36,1.5\nBob,41,2\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := Default().Run(context.Background(), Query{Driver: "csv", Server: "people.csv", Root: dir, Statement: "SELECT name, age + 1 AS next FROM people WHERE score > ? ORDER BY age", Params: []string{"1.6"}})

	if err != nil {
		t.Fatal(err)
	}
	if len(res.Rows) != 1 || res.Rows[0][0] != "Bob" {
		t.Fatalf("got %+v", res)
	}
}

func TestCSV_Given_a_folder_When_joined_Then_every_file_is_a_table(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "a.csv"), []byte("id,x\n1,p\n"), 0o600)
	_ = os.WriteFile(filepath.Join(dir, "b.csv"), []byte("id,y\n1,q\n"), 0o600)

	res, err := Default().Run(context.Background(), Query{Driver: "csv", Server: dir, Statement: "SELECT a.x, b.y FROM a JOIN b ON a.id = b.id"})

	if err != nil || len(res.Rows) != 1 {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestCSV_Given_a_missing_path_Then_the_error_names_it(t *testing.T) {
	_, err := Default().Run(context.Background(), Query{Driver: "csv", Server: "nope.csv", Root: t.TempDir(), Statement: "SELECT 1"})

	if err == nil {
		t.Fatal("want error")
	}
}

func TestXLSX_Given_a_workbook_When_joined_Then_each_sheet_is_a_table(t *testing.T) {
	dir := t.TempDir()
	f := excelize.NewFile()
	_ = f.SetSheetName("Sheet1", "people")
	for i, r := range [][]any{{"id", "name"}, {1, "Ada"}, {2, "Bob"}} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetSheetRow("people", cell, &r)
	}
	_, _ = f.NewSheet("pets")
	for i, r := range [][]any{{"owner", "pet"}, {2, "Rex"}} {
		cell, _ := excelize.CoordinatesToCellName(1, i+1)
		_ = f.SetSheetRow("pets", cell, &r)
	}
	if err := f.SaveAs(filepath.Join(dir, "book.xlsx")); err != nil {
		t.Fatal(err)
	}

	res, err := Default().Run(context.Background(), Query{Driver: "xlsx", Server: "book.xlsx", Root: dir, Statement: "SELECT p.name, q.pet FROM people p JOIN pets q ON q.owner = p.id"})

	if err != nil || len(res.Rows) != 1 || res.Rows[0][0] != "Bob" || res.Rows[0][1] != "Rex" {
		t.Fatalf("got %+v, %v", res, err)
	}
}

func TestParquet_Given_a_file_When_queried_Then_typed_rows_come_back(t *testing.T) {
	dir := t.TempDir()
	type row struct {
		Name  string  `parquet:"name"`
		Age   int64   `parquet:"age"`
		Score float64 `parquet:"score"`
	}
	out, err := os.Create(filepath.Join(dir, "people.parquet"))
	if err != nil {
		t.Fatal(err)
	}
	if err := parquet.Write(out, []row{{"Ada", 36, 1.5}, {"Bob", 41, 2.5}}); err != nil {
		t.Fatal(err)
	}
	_ = out.Close()

	res, err := Default().Run(context.Background(), Query{Driver: "parquet", Server: "people.parquet", Root: dir, Statement: "SELECT name, age FROM people WHERE score > 2"})

	if err != nil || len(res.Rows) != 1 || res.Rows[0][0] != "Bob" {
		t.Fatalf("got %+v, %v", res, err)
	}
}
