// SPDX-License-Identifier: Apache-2.0

package dataset

import (
	"path/filepath"
	"strings"
	"testing"
)

// Before: a relative path was joined onto the root as it was and an absolute path was used as it was, so a cloned
// collection could make a query read any file the person could.
func TestConfine_should_keep_a_file_path_inside_the_collection(t *testing.T) {
	root := filepath.Join(t.TempDir(), "shop")
	outside := filepath.Join(filepath.Dir(root), "other", "x.csv")
	tests := []struct {
		name    string
		server  string
		path    string
		allowed bool
		ok      bool
	}{
		{"inside", "data/sales.csv", filepath.Join(root, "data", "sales.csv"), false, true},
		{"climbs out", "../../etc/passwd", filepath.Join(root, "..", "..", "etc", "passwd"), false, false},
		{"climbs out and back in", "../shop/a.csv", filepath.Join(root, "..", "shop", "a.csv"), false, true},
		{"absolute outside", outside, outside, false, false},
		{"absolute outside but allowed", outside, outside, true, true},
		{"sibling with the same prefix", "x", root + "-evil/x", false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := confine(Query{Root: root, Server: tt.server, AllowOutsideRoot: tt.allowed}, tt.path)

			if (err == nil) != tt.ok {
				t.Fatalf("err %v, want ok %v", err, tt.ok)
			}
			if err != nil && !strings.Contains(err.Error(), "datasets.allow_outside_collection") {
				t.Fatalf("the message does not say what to do: %v", err)
			}
		})
	}
}

func TestConfine_Given_no_root_Then_nothing_is_confined(t *testing.T) {
	_, err := confine(Query{Server: "/anywhere/x.csv"}, "/anywhere/x.csv")

	if err != nil {
		t.Fatal(err)
	}
}
