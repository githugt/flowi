// SPDX-License-Identifier: Apache-2.0

package synth

import (
	"slices"
	"testing"
)

func petstore() []Op {
	return []Op{
		{Request: "pets/create", Method: "POST", Path: "/pets", Fields: []string{"id", "name"}},
		{Request: "pets/list", Method: "GET", Path: "/pets", Fields: []string{"items"}},
		{Request: "pets/get", Method: "GET", Path: "/pets/{petId}", Params: []string{"petId"}, Fields: []string{"id", "name"}},
		{Request: "pets/update", Method: "PUT", Path: "/pets/{petId}", Params: []string{"petId"}},
		{Request: "pets/delete", Method: "DELETE", Path: "/pets/{petId}", Params: []string{"petId"}},
		{Request: "orders/create", Method: "POST", Path: "/orders", Fields: []string{"id"}},
		{Request: "orders/get", Method: "GET", Path: "/orders/{orderId}", Params: []string{"orderId"}},
		{Request: "owners/get", Method: "GET", Path: "/owners/{ownerId}", Params: []string{"ownerId"}},
	}
}

func edgeTargets(edges []Edge, from string) []string {
	var out []string
	for _, e := range edges {
		if e.From == from {
			out = append(out, e.To)
		}
	}
	slices.Sort(out)
	return out
}

func TestGraph_should_feed_the_id_a_create_returns_to_the_requests_of_that_resource(t *testing.T) {
	edges := Graph(petstore())

	if got := edgeTargets(edges, "pets/create"); !slices.Equal(got, []string{"pets/delete", "pets/get", "pets/update"}) {
		t.Errorf("pets/create feeds %v", got)
	}
	if got := edgeTargets(edges, "orders/create"); !slices.Equal(got, []string{"orders/get"}) {
		t.Errorf("orders/create feeds %v: an id of one resource must not feed another", got)
	}
}

func TestGraph_should_say_what_evidence_each_edge_rests_on(t *testing.T) {
	var found Edge
	for _, e := range Graph(petstore()) {
		if e.From == "pets/create" && e.To == "pets/get" {
			found = e
		}
	}

	if found.Field != "id" || found.Param != "petId" || found.Confidence < 0.8 {
		t.Fatalf("%+v", found)
	}
}

func TestGraph_Given_a_param_nothing_returns_Then_no_edge(t *testing.T) {
	for _, e := range Graph(petstore()) {
		if e.To == "owners/get" {
			t.Fatalf("owners/get has no maker: %+v", e)
		}
	}
}

func TestGraph_Given_a_producer_that_is_not_a_create_Then_no_edge_from_it(t *testing.T) {
	for _, e := range Graph(petstore()) {
		if e.From == "pets/get" || e.From == "pets/list" {
			t.Fatalf("only a POST makes a resource: %+v", e)
		}
	}
}

func TestPropose_should_order_a_resource_lifecycle_create_read_update_delete(t *testing.T) {
	ops := petstore()

	got := Propose(ops, Graph(ops))

	if len(got) != 2 {
		t.Fatalf("%d proposals: %+v", len(got), got)
	}
	var pets Proposal
	for _, p := range got {
		if p.Steps[0].Request == "pets/create" {
			pets = p
		}
	}
	order := []string{}
	for _, s := range pets.Steps {
		order = append(order, s.Request)
	}
	if !slices.Equal(order, []string{"pets/create", "pets/get", "pets/update", "pets/delete"}) {
		t.Fatalf("order %v", order)
	}
	if pets.Name != "pets" {
		t.Errorf("name %q", pets.Name)
	}
}
