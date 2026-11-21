// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import "testing"

func TestEnvironmentService_Compare_should_list_the_workspace_environments_with_a_row_per_variable(t *testing.T) {
	app, _, _ := twoCollections(t)

	v := app.Environments().Compare()

	if len(v.Envs) == 0 || len(v.Rows) == 0 {
		t.Fatalf("view %+v", v)
	}
	for _, r := range v.Rows {
		if len(r.Cells) != len(v.Envs) {
			t.Fatalf("row %s has %d cells for %d environments", r.Name, len(r.Cells), len(v.Envs))
		}
		for _, c := range r.Cells {
			if c.State == "secret" && c.Value != "" {
				t.Fatalf("a secret's value is in the view: %+v", r)
			}
		}
	}
}
