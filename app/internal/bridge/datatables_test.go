// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	"github.com/githugt/flowi/lib/dataset"
	"github.com/githugt/flowi/lib/model"
)

func openDataFile(t *testing.T, b *vmtest.Backend) (*bridge.RequestService, string) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R9", Name: "Sales"}, Path: "sales", File: "/c/requests/sales.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", File: &model.DataFile{Format: "csv", Path: "data/sales.csv", Query: "SELECT 1"}}}})
	b.Cols = []model.Collection{c}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == "Sales" {
			if _, err := app.Requests().Open(n.ID); err != nil {
				t.Fatal(err)
			}
			return app.Requests(), n.ID
		}
	}
	t.Fatal("no Sales request")
	return nil, ""
}

// Before: a data file request showed no table, sheet or column names, so the person had to know them to write `FROM sales`.
func TestRequestService_DataTables_should_list_each_table_with_the_statement_that_looks_at_it(t *testing.T) {
	b := &vmtest.Backend{Tables: []dataset.TableInfo{{Name: "sales", File: "sales.csv", Rows: 3, Columns: []dataset.Column{{Name: "id", Type: "INTEGER"}, {Name: "region", Type: "TEXT"}}}}}
	reqs, node := openDataFile(t, b)

	got := reqs.DataTables(node)

	if len(got.Tables) != 1 || got.Note != "" {
		t.Fatalf("got %+v", got)
	}
	tbl := got.Tables[0]
	if tbl.Select != "SELECT * FROM sales LIMIT 100" || tbl.Rows != 3 || len(tbl.Columns) != 2 || tbl.Columns[1].Type != "TEXT" {
		t.Fatalf("table %+v", tbl)
	}
}

func TestRequestService_DataTables_Given_the_files_cannot_be_read_Then_a_note_not_an_error(t *testing.T) {
	b := &vmtest.Backend{TablesErr: errors.New("the path \"data/sales.csv\" cannot be read")}
	reqs, node := openDataFile(t, b)

	got := reqs.DataTables(node)

	if got.Tables == nil || len(got.Tables) != 0 || got.Note == "" {
		t.Fatalf("got %+v", got)
	}
}

func TestRequestService_DataTables_Given_a_database_request_Then_nothing_is_listed_and_no_file_is_read(t *testing.T) {
	reqs, node, _ := openDataset(t)

	got := reqs.DataTables(node)

	if len(got.Tables) != 0 || got.Note != "" {
		t.Fatalf("got %+v", got)
	}
}
