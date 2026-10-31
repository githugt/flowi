// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openDataset opens a dataset request, returning the requests service, the request's node and what the window was
// sent.
func openDataset(t *testing.T) (*bridge.RequestService, string, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R8", Name: "Customers"}, Path: "customers", File: "/c/requests/customers.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", Dataset: &model.Dataset{Driver: "postgres", Server: "postgres://app@db/shop", Query: "SELECT 1"}}}})
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == "Customers" {
			if _, err := app.Requests().Open(n.ID); err != nil {
				t.Fatal(err)
			}
			return app.Requests(), n.ID, pub
		}
	}
	t.Fatal("no Customers request")
	return nil, "", nil
}

func TestRequestService_should_show_a_dataset_request_with_its_block_and_the_drivers(t *testing.T) {
	reqs, node, _ := openDataset(t)

	view, err := reqs.Open(node)

	if err != nil {
		t.Fatal(err)
	}
	if view.Kind != "dataset" || view.Verb != "SQL" || view.Dataset == nil || view.Dataset.Block.Driver != "postgres" {
		t.Fatalf("kind %q verb %q dataset %+v", view.Kind, view.Verb, view.Dataset)
	}
	if len(view.Dataset.Drivers) != 4 || view.Dataset.Drivers[0].ID != "postgres" || view.Dataset.Drivers[0].Label != "PostgreSQL" {
		t.Fatalf("drivers %+v", view.Dataset.Drivers)
	}
}

func TestRequestService_SetDataset_should_replace_the_block_and_mark_the_request_changed(t *testing.T) {
	reqs, node, pub := openDataset(t)

	reqs.SetDataset(node, model.Dataset{Driver: "sqlite", Server: "shop.db", Query: "SELECT 2", Params: []string{"a"}})

	got := pub.last["request"].(bridge.RequestView)
	if !got.Dirty || got.Dataset.Block.Driver != "sqlite" || got.Dataset.Block.Query != "SELECT 2" || len(got.Dataset.Block.Params) != 1 {
		t.Fatalf("dirty %v dataset %+v", got.Dirty, got.Dataset)
	}
}

func TestRequestService_should_not_show_a_dataset_block_for_an_http_request(t *testing.T) {
	c := vmtest.SampleCollection()
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)
	app.Start(t.Context())
	node := "r:C1:R1"

	view, err := app.Requests().Open(node)

	if err != nil || view.Dataset != nil || view.Kind == "dataset" {
		t.Fatalf("kind %q dataset %+v err %v", view.Kind, view.Dataset, err)
	}
}

func TestWorkspaceService_Menu_should_offer_a_new_dataset_query_that_asks_for_the_database(t *testing.T) {
	app, _ := fixture(t)
	app.Start(t.Context())
	coll := app.Workspace().Tree().Roots[0]

	menu := app.Workspace().Menu([]string{coll.ID})

	for _, e := range menu {
		if e.Action != "newDataset" {
			continue
		}
		if e.Label != "New dataset query…" || e.Prompt == nil || len(e.Prompt.Protocols) != 4 || e.Prompt.Protocols[0].ID != "postgres" || e.Prompt.Protocols[0].Label != "PostgreSQL" {
			t.Fatalf("entry %+v prompt %+v", e, e.Prompt)
		}
		return
	}
	t.Fatalf("no new dataset query entry in %+v", menu)
}

func TestWorkspaceService_Do_newDataset_should_create_it_with_the_chosen_database(t *testing.T) {
	app, pub := fixture(t)
	app.Start(t.Context())
	coll := app.Workspace().Tree().Roots[0]

	app.Workspace().Do("newDataset", []string{coll.ID}, "customers", "mysql")

	if opened, _ := pub.last["open"].(string); opened == "" {
		t.Fatalf("nothing was opened: %v", pub.last)
	}
}
