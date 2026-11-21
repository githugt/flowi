// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openStaging opens the staging environment of the sample workspace and returns its node, the app and the fake backend.
func openStaging(t *testing.T) (string, *bridge.App, *vmtest.Backend) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots {
		for _, c := range n.Children {
			if c.Kind == "environment" && c.Label == "staging" {
				if _, err := app.Environments().Open(c.ID); err != nil {
					t.Fatal(err)
				}
				return c.ID, app, b
			}
		}
	}
	t.Fatal("no staging environment in the sample")
	return "", nil, nil
}

func TestEnvironmentService_Share_Given_a_saved_environment_When_shared_Then_the_backend_writes_it_into_the_collection(t *testing.T) {
	// Before: environments could be imported from a collection but never exported to one.
	node, app, b := openStaging(t)
	coll := string(vmtest.SampleCollection().Manifest.ID)
	b.ShareNotes = []string{"staging builds on base."}

	got, err := app.Environments().Share(node, coll, false)

	if err != nil || got.Exists || got.File == "" || len(got.Notes) != 1 {
		t.Fatalf("share %+v err %v", got, err)
	}
	if !b.Called("ShareEnvironment " + coll + " staging false") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestEnvironmentService_Share_Given_a_file_already_there_When_shared_Then_it_says_so_until_overwrite(t *testing.T) {
	node, app, b := openStaging(t)
	coll := vmtest.SampleCollection().Manifest.ID
	b.SharedEnvs = map[model.ID]bool{coll: true}

	first, err := app.Environments().Share(node, string(coll), false)
	again, errAgain := app.Environments().Share(node, string(coll), true)

	if err != nil || !first.Exists || errAgain != nil || again.Exists {
		t.Fatalf("first %+v (%v), again %+v (%v)", first, err, again, errAgain)
	}
}

func TestEnvironmentService_Share_Given_unsaved_edits_When_shared_Then_it_asks_to_save_first(t *testing.T) {
	node, app, b := openStaging(t)
	svc := app.Environments()
	v, _ := svc.Open(node)
	svc.SetRows(node, append(v.Rows, bridge.EnvRow{Name: "region", Value: "eu"}))

	_, err := svc.Share(node, string(vmtest.SampleCollection().Manifest.ID), false)

	if err == nil || !strings.Contains(err.Error(), "save staging first") {
		t.Fatalf("err %v", err)
	}
	if b.Called("ShareEnvironment " + string(vmtest.SampleCollection().Manifest.ID) + " staging false") {
		t.Fatal("it shared while the environment had unsaved edits")
	}
}

func TestEnvironmentService_Share_Given_a_node_that_is_not_open_When_shared_Then_it_refuses(t *testing.T) {
	_, app, _ := openStaging(t)

	_, err := app.Environments().Share("nope", "c", false)

	if err == nil {
		t.Fatal("no error for a node that is not open")
	}
}

func TestEnvironmentService_ShareTargets_Then_only_mounted_collections_are_offered(t *testing.T) {
	_, app, _ := openStaging(t)

	got := app.Environments().ShareTargets()

	if len(got) == 0 || got[0].Label == "" {
		t.Fatalf("targets %+v", got)
	}
}
