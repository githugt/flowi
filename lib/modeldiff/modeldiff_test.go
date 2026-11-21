// SPDX-License-Identifier: Apache-2.0

package modeldiff_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/modeldiff"
	"github.com/githugt/flowi/lib/respdiff"
)

func request(url string, headers ...model.KeyValue) model.Request {
	return model.Request{Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: url, Headers: headers}}}
}

func TestCompare_Given_a_changed_url_Then_the_path_and_both_values_are_listed(t *testing.T) {
	r := modeldiff.Compare(request("https://theirs.example/orders"), request("https://mine.example/orders"))

	if len(r.Changes) != 1 {
		t.Fatalf("changes: %+v", r.Changes)
	}
	c := r.Changes[0]
	if c.Path != "$.Versions[0].URL" || c.Kind != respdiff.Changed || !strings.Contains(c.Disk, "theirs") || !strings.Contains(c.Mine, "mine") {
		t.Fatalf("change: %+v", c)
	}
}

func TestCompare_Given_the_same_content_Then_nothing_differs(t *testing.T) {
	if r := modeldiff.Compare(request("x"), request("x")); !r.Same() {
		t.Fatalf("result: %+v", r)
	}
}

func TestCompare_Given_a_header_only_one_side_has_Then_it_is_added_or_removed(t *testing.T) {
	r := modeldiff.Compare(request("x"), request("x", model.KeyValue{Key: "X-Trace", Value: "on"}))

	if len(r.Changes) != 1 || r.Changes[0].Kind != respdiff.Added || r.Changes[0].Mine == "" || r.Changes[0].Disk != "" {
		t.Fatalf("changes: %+v", r.Changes)
	}
}

// Before: a banner that listed both sides would have printed a token that a colleague committed.
func TestCompare_Given_a_credential_that_differs_Then_it_is_hidden_and_counted(t *testing.T) {
	a := request("x", model.KeyValue{Key: "Authorization", Value: "Bearer theirs-secret"})
	b := request("x", model.KeyValue{Key: "Authorization", Value: "Bearer my-secret"})

	r := modeldiff.Compare(a, b)

	if !r.Same() || r.Hidden != 1 {
		t.Fatalf("result: %+v", r)
	}
}

func TestCompare_Given_ignored_fields_Then_they_are_not_compared(t *testing.T) {
	a, b := request("x"), request("x")
	b.EditBase = "1"

	if r := modeldiff.Compare(a, b, "EditBase"); !r.Same() {
		t.Fatalf("result: %+v", r)
	}
	if r := modeldiff.Compare(a, b); r.Same() {
		t.Fatal("EditBase should differ when not ignored")
	}
}

func TestCompare_Given_more_than_the_limit_Then_the_rest_is_counted(t *testing.T) {
	var a, b model.Request
	for i := 0; i < 150; i++ {
		a.Versions = append(a.Versions, model.RequestVersion{URL: "a"})
		b.Versions = append(b.Versions, model.RequestVersion{URL: "b"})
	}

	r := modeldiff.Compare(a, b)

	if len(r.Changes) != 100 || r.More != 50 {
		t.Fatalf("listed %d, more %d", len(r.Changes), r.More)
	}
}
