// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

func TestDiffFlowAgainst_Given_an_earlier_file_Then_the_steps_that_changed_are_listed(t *testing.T) {
	// Before: a reviewer of a flow change read raw YAML.
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	older, err := os.ReadFile(f.File)
	if err != nil {
		t.Fatal(err)
	}
	edited := f
	edited.Nodes = append([]model.Node(nil), f.Nodes...)
	edited.Nodes[0].Name = "Log in as the new user"
	edited.Nodes = append(edited.Nodes, model.Node{ID: "extra", Type: "log"})
	if err := w.SaveFlow(target, edited, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	changes, diags, err := w.DiffFlowAgainst(target, older)

	if err != nil || len(diags) != 0 {
		t.Fatalf("err %v, diags %v", err, diags)
	}
	var kinds []string
	for _, c := range changes {
		kinds = append(kinds, string(c.Kind)+" "+c.Step)
	}
	if !strings.Contains(strings.Join(kinds, ","), "renamed "+f.Nodes[0].ID) || !strings.Contains(strings.Join(kinds, ","), "added extra") {
		t.Fatalf("changes %v", kinds)
	}
}

func TestDiffFlowAgainst_Given_the_same_file_Then_there_are_no_changes(t *testing.T) {
	w := demoWorkspace(t)
	target, f := userCreation(t, w)
	same, _ := os.ReadFile(f.File)

	changes, _, err := w.DiffFlowAgainst(target, same)

	if err != nil || len(changes) != 0 {
		t.Fatalf("err %v, changes %v", err, changes)
	}
}

func TestDiffFlowAgainst_Given_text_that_is_not_a_flow_Then_no_changes_and_diagnostics(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	changes, diags, err := w.DiffFlowAgainst(target, []byte("not: [a flow"))

	if err != nil || len(changes) != 0 || len(diags) == 0 {
		t.Fatalf("err %v, changes %v, diags %v", err, changes, diags)
	}
}
