// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/discover"
	"github.com/githugt/flowi/lib/grpcx"
)

func TestDiscover_should_send_the_ticked_paths_and_list_what_answered(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Discovery = discover.Report{Findings: []discover.Finding{
		{Path: "/pets", Status: 200, Kind: "json"}, {Path: "/openapi.json", Status: 200, Kind: "spec"},
	}, Stopped: "stopped after 40 requests, the limit"}

	got, err := app.Import().Discover("https://api.example.com", "X-Key", "k", []string{"/pets"})

	if err != nil || len(got.Findings) != 2 || got.Findings[1].Kind != "spec" || got.Stopped == "" {
		t.Fatalf("discovery %+v err %v", got, err)
	}
	if !b.Called("Discover https://api.example.com X-Key /pets") {
		t.Fatalf("calls %v", b.CallsStarting(""))
	}
}

func TestDiscover_Given_a_token_without_a_header_Then_it_goes_as_a_bearer_token(t *testing.T) {
	app, b := checksApp(t)

	_, err := app.Import().Discover("https://api.example.com", "", "tok", []string{"/"})

	if err != nil || !b.Called("Discover https://api.example.com Authorization /") {
		t.Fatalf("err %v calls %v", err, b.CallsStarting(""))
	}
}

func TestDiscover_Given_nothing_ticked_Then_nothing_is_sent(t *testing.T) {
	app, b := checksApp(t)

	_, err := app.Import().Discover("https://api.example.com", "", "", nil)

	if err == nil || len(b.CallsStarting("Discover ")) != 0 {
		t.Fatalf("err %v calls %v", err, b.CallsStarting(""))
	}
}

func TestSaveDiscovery_should_draft_from_the_last_discovery_without_asking_the_server_again(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Drafted = flowi.DiscoveredRequests{Created: []flowi.RequestTarget{{}, {}}, Skipped: []string{"/x: not JSON"}}
	if _, err := app.Import().Discover("https://api.example.com", "", "", []string{"/"}); err != nil {
		t.Fatal(err)
	}

	got, err := app.Import().SaveDiscovery("C1")

	if err != nil || got.Created != 2 || len(got.Skipped) != 1 {
		t.Fatalf("saved %+v err %v", got, err)
	}
	if n := len(b.CallsStarting("Discover ")); n != 1 {
		t.Fatalf("the server was asked %d times", n)
	}
}

func TestSaveDiscovery_Given_no_discovery_yet_Then_it_says_to_run_one(t *testing.T) {
	app, _ := checksApp(t)

	_, err := app.Import().SaveDiscovery("C1")

	if err == nil || !strings.Contains(err.Error(), "first") {
		t.Fatalf("err %v", err)
	}
}

func TestDiscoverPlan_Given_a_grpc_address_Then_the_one_request_is_a_reflection_ask(t *testing.T) {
	app, _ := checksApp(t)

	got, err := app.Import().DiscoverPlan("grpc://shop.test:50051")

	if err != nil || len(got) != 1 || got[0].Method != "reflect" {
		t.Fatalf("plan %+v err %v", got, err)
	}
}

func TestDiscover_Given_a_grpc_address_Then_it_lists_the_methods_the_server_offers(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Methods = []grpcx.Method{{Service: "shop.Shop", Name: "Get"}, {Service: "shop.Shop", Name: "List"}}

	got, err := app.Import().Discover("grpc://shop.test:50051", "", "", []string{"grpc://shop.test:50051"})

	if err != nil || len(got.Findings) != 2 || got.Findings[0].Path != "shop.Shop/Get" || got.Findings[0].Kind != "grpc" {
		t.Fatalf("discovery %+v err %v", got, err)
	}
	if !b.Called("DiscoverGRPC grpc://shop.test:50051") || len(b.CallsStarting("Discover ")) != 0 {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestSaveDiscovery_Given_a_grpc_discovery_Then_it_drafts_the_methods_not_http_findings(t *testing.T) {
	app, b := checksApp(t)
	b.Check.Methods = []grpcx.Method{{Service: "shop.Shop", Name: "Get"}}
	b.Check.Drafted = flowi.DiscoveredRequests{Created: []flowi.RequestTarget{{}}}
	if _, err := app.Import().Discover("grpc://shop.test:50051", "", "", []string{"x"}); err != nil {
		t.Fatal(err)
	}

	got, err := app.Import().SaveDiscovery("C1")

	if err != nil || got.Created != 1 || !b.Called("SaveGRPCDiscovery C1 grpc://shop.test:50051 1") || len(b.CallsStarting("SaveDiscovery ")) != 0 {
		t.Fatalf("saved %+v err %v calls %v", got, err, b.Calls)
	}
}
