// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func requestStep(id, reqID, reqPath string) model.Node {
	return model.Node{ID: id, Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: model.ID(reqID), Path: reqPath}}}
}

// Before: a step made from a request was only an id and a ref in the file, so the flow's Code tab and the canvas
// said nothing about what each step was for.
func TestFillStepText_Given_a_request_step_with_no_text_Then_it_gets_the_request_name_and_a_line_about_the_call(t *testing.T) {
	c := vmtest.SampleCollection()
	n := requestStep("login", "R1", "auth/login")

	vm.FillStepText(c, &n)

	if n.Name != "Login" || n.Description != "POST /login" {
		t.Fatalf("name %q description %q", n.Name, n.Description)
	}
}

func TestFillStepText_Given_text_the_person_wrote_Then_it_is_kept(t *testing.T) {
	c := vmtest.SampleCollection()
	n := requestStep("login", "R1", "auth/login")
	n.Name, n.Description = "Sign in", "Gets the token"

	vm.FillStepText(c, &n)

	if n.Name != "Sign in" || n.Description != "Gets the token" {
		t.Fatalf("name %q description %q", n.Name, n.Description)
	}
}

func TestFillStepText_Given_a_request_found_by_path_only_Then_it_is_still_described(t *testing.T) {
	c := vmtest.SampleCollection()
	n := requestStep("health", "", "health")

	vm.FillStepText(c, &n)

	if n.Description != "GET /health" {
		t.Fatalf("description %q", n.Description)
	}
}

func TestFillStepText_Given_a_request_that_is_not_there_Then_only_the_name_from_its_path_is_offered(t *testing.T) {
	c := vmtest.SampleCollection()
	n := requestStep("gone", "R9", "orders/gone")

	vm.FillStepText(c, &n)

	if n.Name != "" || n.Description != "" {
		t.Fatalf("name %q description %q", n.Name, n.Description) // the id already says "gone"
	}
}

func TestFillStepText_Given_a_subflow_step_Then_it_is_named_after_the_flow(t *testing.T) {
	c := vmtest.SampleCollection()
	n := model.Node{ID: "again", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: "F1"}}}

	vm.FillStepText(c, &n)

	if n.Name != "Signup" || n.Description != "Runs Signup" {
		t.Fatalf("name %q description %q", n.Name, n.Description)
	}
}

func TestFillStepText_Given_a_step_that_calls_nothing_Then_nothing_is_added(t *testing.T) {
	n := model.Node{ID: "wait", Type: "delay"}

	vm.FillStepText(vmtest.SampleCollection(), &n)

	if n.Name != "" || n.Description != "" {
		t.Fatalf("name %q description %q", n.Name, n.Description)
	}
}
