// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func firstCollection(t *testing.T, w *Workspace) model.Collection {
	t.Helper()
	for _, c := range w.Collections() {
		if len(c.Requests) > 0 {
			return c
		}
	}
	t.Fatal("no collection with requests")
	return model.Collection{}
}

func onlyCode(t *testing.T, err error, want diag.Code) {
	t.Helper()
	var le *diag.ListError
	if !errors.As(err, &le) || len(le.List) != 1 || le.List[0].Code != want {
		t.Fatalf("want %s, got %v", want, err)
	}
}

// Before: a request had tags only by editing the file; the app had no way to set either.
func TestSetRequestLabels_Given_tags_and_a_status_Then_they_are_saved_cleaned_and_the_versions_are_untouched(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	r := c.Requests[0]
	target := RequestTarget{Collection: c.Manifest.ID, Request: r.ID}

	// Act
	err := w.SetRequestLabels(target, []string{" smoke ", "needs review", "smoke", ""}, "review", SaveOptions{DropComments: true})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	_, got, _ := w.requestFor(target)
	if !slices.Equal(got.Tags, []string{"smoke", "needs-review"}) || got.Status != "review" {
		t.Fatalf("tags %q status %q", got.Tags, got.Status)
	}
	if len(got.Versions) != len(r.Versions) || got.Name != r.Name {
		t.Fatalf("the versions or the name changed: %d versions, name %q", len(got.Versions), got.Name)
	}
	data, _ := os.ReadFile(r.File)
	if !strings.Contains(string(data), "status: review\n") || !strings.Contains(string(data), "- needs-review\n") {
		t.Fatalf("file:\n%s", data)
	}
}

func TestSetRequestLabels_Given_no_status_Then_the_status_line_goes_from_the_file(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	r := c.Requests[0]
	target := RequestTarget{Collection: c.Manifest.ID, Request: r.ID}
	if err := w.SetRequestLabels(target, nil, "testing", SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}

	// Act
	err := w.SetRequestLabels(target, nil, "", SaveOptions{DropComments: true})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(r.File)
	if strings.Contains(string(data), "\nstatus:") {
		t.Fatalf("file still has a status:\n%s", data)
	}
}

func TestSetRequestLabels_Given_an_unknown_status_Then_it_is_refused_and_nothing_is_written(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	r := c.Requests[0]
	before, _ := os.ReadFile(r.File)

	// Act
	err := w.SetRequestLabels(RequestTarget{Collection: c.Manifest.ID, Request: r.ID}, nil, "revew", SaveOptions{DropComments: true})

	// Assert
	onlyCode(t, err, diag.CodeStatusUnknown)
	after, _ := os.ReadFile(r.File)
	if string(before) != string(after) {
		t.Fatal("the file changed")
	}
}

func TestSetFlowLabels_should_save_tags_and_status_on_a_flow(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, f := userCreation(t, w)

	// Act
	err := w.SetFlowLabels(target, []string{"nightly", "smoke"}, "production", SaveOptions{DropComments: true})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	_, got, _ := w.flowFor(target)
	if !slices.Equal(got.Tags, []string{"nightly", "smoke"}) || got.Status != "production" || len(got.Nodes) != len(f.Nodes) {
		t.Fatalf("tags %q status %q nodes %d", got.Tags, got.Status, len(got.Nodes))
	}
	if err := w.SetFlowLabels(target, nil, "nope", SaveOptions{}); err == nil {
		t.Fatal("an unknown status was accepted")
	}
}

// Before: a folder's _folder.yaml could hold tags only by hand, and writing one in code would have dropped its settings.
func TestSetFolderLabels_should_keep_what_the_folder_file_already_holds_and_create_one_when_missing(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	if err := w.SetFolderOrder(c.Manifest.ID, "requests", "auth", []string{"b", "a"}, SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}

	// Act
	err := w.SetFolderLabels(c.Manifest.ID, "requests", "auth", []string{"slow"}, "in-development", SaveOptions{DropComments: true})
	fresh := w.SetFolderLabels(c.Manifest.ID, "flows", "brand-new", nil, "planning", SaveOptions{})

	// Assert
	if err != nil || fresh != nil {
		t.Fatal(err, fresh)
	}
	var folder, created model.Folder
	for _, f := range w.Collections()[0].Folders {
		switch f.Dir {
		case "requests/auth":
			folder = f
		case "flows/brand-new":
			created = f
		}
	}
	if !slices.Equal(folder.Order, []string{"b", "a"}) || !slices.Equal(folder.Tags, []string{"slow"}) || folder.Status != "in-development" {
		t.Fatalf("folder %+v", folder)
	}
	if created.Status != "planning" || filepath.Base(created.File) != "_folder.yaml" {
		t.Fatalf("created %+v", created)
	}
	onlyCode(t, w.SetFolderLabels(c.Manifest.ID, "requests", "auth", nil, "nope", SaveOptions{}), diag.CodeStatusUnknown)
	if err := w.SetFolderLabels(c.Manifest.ID, "elsewhere", "auth", nil, "", SaveOptions{}); err == nil {
		t.Fatal("a tree that is neither requests nor flows was accepted")
	}
}

func TestSaveStatuses_should_write_the_manifest_and_make_the_status_usable(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	target := RequestTarget{Collection: c.Manifest.ID, Request: c.Requests[0].ID}
	if err := w.SetRequestLabels(target, nil, "needs-design", SaveOptions{DropComments: true}); err == nil {
		t.Fatal("the status existed before it was defined")
	}

	// Act
	err := w.SaveStatuses(c.Manifest.ID, []model.Status{{Name: "needs-design", Color: "pink", Icon: "bookmark"}}, SaveOptions{DropComments: true})
	use := w.SetRequestLabels(target, nil, "needs-design", SaveOptions{DropComments: true})

	// Assert
	if err != nil || use != nil {
		t.Fatal(err, use)
	}
	list, _ := w.Statuses(c.Manifest.ID)
	if len(list) != 6 || list[5] != (model.Status{Name: "needs-design", Color: "pink", Icon: "bookmark"}) {
		t.Fatalf("list %+v", list)
	}
	data, _ := os.ReadFile(c.Manifest.File)
	if !strings.Contains(string(data), "statuses:\n  - name: needs-design\n    color: pink\n    icon: bookmark\n") {
		t.Fatalf("manifest:\n%s", data)
	}
}

func TestSaveStatuses_Given_a_bad_or_repeated_definition_Then_it_is_refused(t *testing.T) {
	w := demoWorkspace(t)
	c := firstCollection(t, w)
	bad := [][]model.Status{{{Name: "Needs Design"}}, {{Name: "a", Color: "mauve"}}, {{Name: "a"}, {Name: "a"}}}

	for _, list := range bad {
		onlyCode(t, w.SaveStatuses(c.Manifest.ID, list, SaveOptions{DropComments: true}), diag.CodeStatusInvalid)
	}
}

func TestTagsInUse_should_list_what_the_collection_carries(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)
	if err := w.SetFlowLabels(target, []string{"zeta", "alpha"}, "", SaveOptions{DropComments: true}); err != nil {
		t.Fatal(err)
	}

	got, err := w.TagsInUse(target.Collection)

	if err != nil || !slices.Equal(got, []model.TagUse{{Tag: "alpha", Count: 1}, {Tag: "zeta", Count: 1}}) {
		t.Fatalf("got %+v %v", got, err)
	}
}
