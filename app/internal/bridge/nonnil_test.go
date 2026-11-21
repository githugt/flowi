// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"encoding/json"
	"testing"
)

type inner struct {
	Tags []string `json:"tags"`
}

type outer struct {
	List    []string          `json:"list"`
	Map     map[string]string `json:"map"`
	Kept    []string          `json:"kept,omitempty"`
	Inner   inner             `json:"inner"`
	Items   []inner           `json:"items"`
	private []string          //nolint:unused // an unexported field the walk must leave alone
}

// Before: a nil slice in a model value reached the page as null (13 fields of RequestView.Current did), and the page
// held an `?? []` guard for each.
func TestEmptyCollections_Given_nil_slices_and_maps_Then_they_encode_as_empty(t *testing.T) {
	orig := outer{Items: []inner{{}}}

	got := emptyCollections(orig)
	b, _ := json.Marshal(got)

	want := `{"list":[],"map":{},"inner":{"tags":[]},"items":[{"tags":[]}]}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}

func TestEmptyCollections_Given_a_shared_slice_Then_the_original_is_not_changed(t *testing.T) {
	orig := outer{Items: []inner{{}}}

	_ = emptyCollections(orig)

	if orig.List != nil || orig.Items[0].Tags != nil {
		t.Fatalf("the original was changed: %+v", orig)
	}
}
