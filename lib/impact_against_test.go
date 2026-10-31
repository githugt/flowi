// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// petsWithFlow is a collection with the petstore imported and a flow that reads the pet's name.
func petsWithFlow(t *testing.T) (*Workspace, model.ID) {
	t.Helper()
	w, c := petsCollection(t)
	importPetstore(t, w, c)
	plan, err := w.PreviewImport(ImportArazzo, []byte(fetchNameWorkflow), c)
	if err != nil {
		t.Fatal(err)
	}
	if err := w.ApplyImport(plan, ApplyOptions{}); err != nil {
		t.Fatal(err)
	}
	return w, c
}

func changedSpec(t *testing.T, edits ...string) []byte {
	t.Helper()
	spec := string(readFixture(t, "openapi/testdata/petstore30.yaml"))
	for i := 0; i+1 < len(edits); i += 2 {
		if !strings.Contains(spec, edits[i]) {
			t.Fatalf("the fixture has no %q", edits[i])
		}
		spec = strings.ReplaceAll(spec, edits[i], edits[i+1])
	}
	return []byte(spec)
}

func TestImpactAgainst_Given_the_new_spec_drops_a_field_a_flow_reads_Then_the_flow_and_step_are_named(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)
	spec := changedSpec(t, "        name: { type: string }", "        label: { type: string }", "required: [name]", "required: [label]")

	// Act
	got, _, err := w.ImpactAgainst(c, "", spec)

	// Assert
	if err != nil || len(got.Breakages) != 1 {
		t.Fatalf("err %v impact %+v", err, got)
	}
	b := got.Breakages[0]
	if b.Consumer.Flow != "arazzo/fetchName" || b.Consumer.Node != "fetch" || len(b.Consumer.Reads) != 1 || b.Consumer.Reads[0].Expr != "nodes.fetch.body.name" {
		t.Errorf("the breakage should name the flow, the step and the read: %+v", b)
	}
}

func TestImpactAgainst_Given_the_new_spec_drops_an_operation_Then_its_consumers_are_broken_as_removed(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)
	spec := changedSpec(t, "      operationId: getPet\n      tags: [pets]", "      operationId: fetchPet\n      tags: [pets]")

	// Act
	got, _, err := w.ImpactAgainst(c, "", spec)

	// Assert
	if err != nil || len(got.Breakages) == 0 {
		t.Fatalf("err %v impact %+v", err, got)
	}
	for _, b := range got.Breakages {
		if !strings.Contains(b.Consumer.Reason, "no longer has this request") {
			t.Errorf("breakage %+v", b)
		}
	}
}

func TestImpactAgainst_Given_an_unchanged_spec_Then_nothing_is_broken(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)

	// Act
	got, _, err := w.ImpactAgainst(c, "", changedSpec(t))

	// Assert
	if err != nil || got.Broken() {
		t.Fatalf("an unchanged spec breaks nothing, and a problem that was there before is not its doing: err %v %+v", err, got.Breakages)
	}
}

func TestImpactAgainst_Given_a_name_the_collection_does_not_keep_Then_it_says_so(t *testing.T) {
	// Arrange
	w, c := petsWithFlow(t)

	// Act
	_, _, err := w.ImpactAgainst(c, "nope", changedSpec(t))

	// Assert
	if !errors.Is(err, ErrNoKeptSpec) {
		t.Fatalf("err %v", err)
	}
}
