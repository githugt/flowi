// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: a data file request showed no table, sheet or column names, so the person had to know them to write `FROM sales`.
func TestDataFileTables_should_list_the_tables_of_a_file_inside_the_collection(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	if err := os.MkdirAll(filepath.Join(c.Root, "data"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(c.Root, "data", "sales.csv"), []byte("id,amount\n1,2.5\n2,3\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := w.DataFileTables(context.Background(), c.Manifest.ID, "csv", "data/sales.csv")

	if err != nil || len(got) != 1 || got[0].Name != "sales" || got[0].Rows != 2 || len(got[0].Columns) != 2 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestDataFileTables_should_refuse_a_templated_server_a_path_outside_and_an_unknown_collection(t *testing.T) {
	w := demoWorkspace(t)
	id := w.Collections()[0].Manifest.ID

	_, templated := w.DataFileTables(context.Background(), id, "csv", "{{env.dir}}/x.csv")
	_, outside := w.DataFileTables(context.Background(), id, "csv", "../../x.csv")
	_, unknown := w.DataFileTables(context.Background(), model.ID("nope"), "csv", "x.csv")

	if !errors.Is(templated, ErrTemplatedServer) || outside == nil || unknown == nil {
		t.Fatalf("templated %v outside %v unknown %v", templated, outside, unknown)
	}
}
