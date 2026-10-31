// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import "testing"

func TestDetails_Given_a_request_tab_Then_the_view_lists_its_resolved_values(t *testing.T) {
	// Arrange
	reqs, v, _, app, _ := openLoginWith(t)
	reqs.Send(v.Node, "all")

	// Act
	d := app.Panels().Details(v.Node)

	// Assert
	if d.Kind != "request" || d.Title != "Login" {
		t.Fatalf("kind %q title %q", d.Kind, d.Title)
	}
	got := map[string]string{}
	for _, r := range d.Rows {
		got[r.Label] = r.Value
	}
	for _, label := range []string{"Method", "URL", "Version", "Overlay", "Auth", "Environment", "Hooks", "Last result"} {
		if _, ok := got[label]; !ok {
			t.Errorf("no %q row in %+v", label, d.Rows)
		}
	}
	if got["Method"] != "POST" || got["Version"] != v.Selected || got["Overlay"] != "none" {
		t.Errorf("rows %v", got)
	}
}

func TestDetails_Given_a_flow_tab_Then_the_view_lists_inputs_steps_and_the_last_run(t *testing.T) {
	// Arrange
	app, v := openWithIOApp(t)

	// Act
	d := app.Panels().Details(v.Node)

	// Assert
	got := map[string]string{}
	for _, r := range d.Rows {
		got[r.Label] = r.Value
	}
	if d.Kind != "flow" || got["Inputs"] != "email, count, debug" || got["Steps"] != "1" || got["Last run"] != "not run yet" {
		t.Fatalf("kind %q rows %v", d.Kind, got)
	}
}

func TestDetails_Given_a_node_that_is_not_open_Then_the_view_is_empty(t *testing.T) {
	_, _, _, app, _ := openLoginWith(t)

	d := app.Panels().Details("nope")

	if d.Kind != "" || len(d.Rows) != 0 {
		t.Fatalf("%+v", d)
	}
}
