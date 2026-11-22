// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Before: nothing knew a status, so a typo in `status:` ("revew") would have been accepted and shown as no status.
func TestValidate_Given_an_unknown_status_on_a_request_a_flow_and_a_folder_Then_each_file_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	f.request(&c, "login", "1")
	c.Requests[0].Status = "revew"
	f.flow(&c, "checkout")
	c.Flows[0].Status = "done"
	c.Folders = []model.Folder{{Dir: "requests/auth", File: c.Root + "/requests/auth/_folder.yaml", Status: "later"}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	var files []string
	for _, d := range got {
		if d.Code == diag.CodeStatusUnknown {
			files = append(files, d.File)
			if d.Path != "$.status" {
				t.Errorf("path %q", d.Path)
			}
		}
	}
	if len(files) != 3 {
		t.Fatalf("want three, got %v", codesOf(got))
	}
}

func TestValidate_Given_an_unknown_status_Then_the_message_names_the_known_ones(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	f.request(&c, "login", "1")
	c.Requests[0].Status = "revew"
	c.Manifest.Statuses = []model.Status{{Name: "needs-design"}}

	// Act
	d := only(t, Validate(Target{Collections: []model.Collection{c}}), diag.CodeStatusUnknown)

	// Assert
	msg, hint := diag.English().Message(d)
	msg += " " + hint
	for _, want := range []string{"revew", "request login", "planning", "production", "needs-design", "flowi.yaml"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
}

func TestValidate_Given_a_default_a_custom_and_no_status_Then_nothing_is_reported(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	f.request(&c, "a", "1")
	f.request(&c, "b", "1")
	f.request(&c, "c", "1")
	c.Requests[0].Status = "testing"
	c.Requests[1].Status = "needs-design"
	c.Manifest.Statuses = []model.Status{{Name: "needs-design", Color: "pink", Icon: "bookmark"}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	for _, d := range got {
		if d.Code == diag.CodeStatusUnknown || d.Code == diag.CodeStatusInvalid {
			t.Fatalf("unexpected %v", d)
		}
	}
}

func TestValidate_Given_bad_status_definitions_Then_each_is_reported_at_its_place(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("app")
	c.Manifest.Statuses = []model.Status{
		{Name: "Needs Design"},
		{Name: "ok-one", Color: "mauve"},
		{Name: "ok-two", Icon: "nope"},
		{Name: "dup"},
		{Name: "dup"},
	}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	var paths []string
	for _, d := range got {
		if d.Code == diag.CodeStatusInvalid {
			paths = append(paths, d.Path)
			if d.File != c.Manifest.File {
				t.Errorf("file %q", d.File)
			}
		}
	}
	if strings.Join(paths, " ") != "$.statuses[0] $.statuses[1] $.statuses[2] $.statuses[4]" {
		t.Fatalf("got %v", paths)
	}
}
