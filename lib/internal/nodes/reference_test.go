// SPDX-License-Identifier: Apache-2.0

package nodes

import (
	"flag"
	"os"
	"testing"
)

var updateReference = flag.Bool("update", false, "rewrite docs/reference/nodes.md") //nolint:gochecknoglobals // the standard golden-file flag; flags must be package-level to register before the test runs

const referenceFile = "../../docs/reference/nodes.md"

func TestReference_should_match_docs_reference_nodes_md(t *testing.T) {
	got := Reference()
	if *updateReference {
		if err := os.WriteFile(referenceFile, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	want, err := os.ReadFile(referenceFile)

	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("%s is out of date; run go test ./internal/nodes/ -run Reference -update", referenceFile)
	}
}
