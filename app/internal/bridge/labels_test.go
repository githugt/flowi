// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func labelledApp(t *testing.T) (*bridge.App, *vmtest.Backend) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Folders = []model.Folder{{Dir: "requests/auth", Status: "in-development", Tags: []string{"auth"}}}
	c.Requests[1].Status = "review"
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	return app, b
}

func nodeNamed(t *testing.T, nodes []bridge.TreeNode, label string) bridge.TreeNode {
	t.Helper()
	for _, n := range nodes {
		if n.Label == label {
			return n
		}
		if got, ok := findLabel(n.Children, label); ok {
			return got
		}
	}
	t.Fatalf("no row %q", label)
	return bridge.TreeNode{}
}

func findLabel(nodes []bridge.TreeNode, label string) (bridge.TreeNode, bool) {
	for _, n := range nodes {
		if n.Label == label {
			return n, true
		}
		if got, ok := findLabel(n.Children, label); ok {
			return got, true
		}
	}
	return bridge.TreeNode{}, false
}

func TestWorkspaceService_Tree_should_give_rows_their_status_mark_and_tags(t *testing.T) {
	app, _ := labelledApp(t)

	tree := app.Workspace().Tree()
	login := nodeNamed(t, tree.Roots, "Login")
	verify := nodeNamed(t, tree.Roots, "Verify")
	health := nodeNamed(t, tree.Roots, "Health")
	folder := nodeNamed(t, tree.Roots, "auth")

	if login.Status == nil || login.Status.Name != "in-development" || !login.Status.Inherited || login.Status.From != "requests/auth" || login.Status.Color != "blue" || login.Status.Icon != "code" {
		t.Errorf("inherited: %+v", login.Status)
	}
	if verify.Status == nil || verify.Status.Name != "review" || verify.Status.Inherited {
		t.Errorf("own: %+v", verify.Status)
	}
	if health.Status != nil || len(health.Tags) != 0 {
		t.Errorf("none: %+v %v", health.Status, health.Tags)
	}
	if !slices.Equal(login.Tags, []string{"auth"}) || folder.Status == nil || folder.Status.Inherited {
		t.Errorf("folder: %+v, login tags %v", folder.Status, login.Tags)
	}
}

func TestWorkspaceService_SetLabels_should_write_and_show_the_new_mark_in_the_tree(t *testing.T) {
	app, b := labelledApp(t)
	health := nodeNamed(t, app.Workspace().Tree().Roots, "Health")

	ask, err := app.Workspace().SetLabels(health.ID, []string{"smoke"}, "testing", false)

	if ask || err != nil || !b.Called("SetRequestLabels R3 smoke testing") {
		t.Fatalf("ask %v err %v calls %v", ask, err, b.Calls)
	}
	if got := nodeNamed(t, app.Workspace().Tree().Roots, "Health"); got.Status == nil || got.Status.Name != "testing" || !slices.Equal(got.Tags, []string{"smoke"}) {
		t.Fatalf("row %+v %v", got.Status, got.Tags)
	}
}

func TestWorkspaceService_SetLabels_Given_a_status_the_collection_does_not_have_Then_a_plain_message_comes_back(t *testing.T) {
	app, _ := labelledApp(t)
	health := nodeNamed(t, app.Workspace().Tree().Roots, "Health")

	ask, err := app.Workspace().SetLabels(health.ID, nil, "revew", false)

	if ask || err == nil {
		t.Fatalf("ask %v err %v", ask, err)
	}
	for _, want := range []string{"revew", "planning"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("message %q lacks %q", err, want)
		}
	}
}

func TestWorkspaceService_Labels_should_give_the_editor_the_choices_and_what_is_inherited(t *testing.T) {
	app, _ := labelledApp(t)
	login := nodeNamed(t, app.Workspace().Tree().Roots, "Login")

	v, err := app.Workspace().Labels(login.ID)

	if err != nil {
		t.Fatal(err)
	}
	if v.Kind != "request" || v.Status != "" || v.Inherited == nil || v.Inherited.Name != "in-development" || v.Inherited.From != "requests/auth" {
		t.Errorf("%+v inherited %+v", v, v.Inherited)
	}
	if len(v.Statuses) != 5 || v.Statuses[0].Name != "planning" || len(v.Colors) == 0 || len(v.Icons) == 0 || v.Tags == nil || v.FolderTags == nil {
		t.Errorf("choices %+v colours %v icons %v (tags and folder tags must not be null in JSON)", v.Statuses, v.Colors, v.Icons)
	}
}

func TestWorkspaceService_AddStatus_should_add_a_status_the_collection_then_offers(t *testing.T) {
	app, _ := labelledApp(t)
	health := nodeNamed(t, app.Workspace().Tree().Roots, "Health")

	ask, err := app.Workspace().AddStatus(health.ID, "blocked", "red", "pause", false)
	_, bad := app.Workspace().AddStatus(health.ID, "Not Valid", "red", "pause", false)
	v, _ := app.Workspace().Labels(health.ID)

	if ask || err != nil || bad == nil {
		t.Fatalf("ask %v err %v bad %v", ask, err, bad)
	}
	if len(v.Statuses) != 6 || v.Statuses[5].Name != "blocked" {
		t.Fatalf("statuses %+v", v.Statuses)
	}
}

func TestWorkspaceService_Menu_should_offer_Tags_and_status_for_a_request_a_flow_and_a_folder_only(t *testing.T) {
	app, _ := labelledApp(t)
	tree := app.Workspace().Tree()
	has := func(label string) bool {
		for _, e := range app.Workspace().Menu([]string{nodeNamed(t, tree.Roots, label).ID}) {
			if e.Action == "labels" && e.Dialog == "labels" {
				return true
			}
		}
		return false
	}

	if !has("Login") || !has("signup") || !has("auth") || has("Shop") {
		t.Fatalf("request %v flow %v folder %v collection %v", has("Login"), has("signup"), has("auth"), has("Shop"))
	}
}
