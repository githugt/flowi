// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"net/url"
	"testing"

	"github.com/githugt/flowi/lib/serve/replay"
)

func TestForwardAllows_Given_an_allow_list_When_a_base_is_checked_Then_only_whole_path_segments_match(t *testing.T) {
	mustURL := func(s string) *url.URL {
		u, err := url.Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	f := replay.Forward{Only: []*url.URL{mustURL("https://api.test/v1")}}
	tests := []struct {
		base string
		want bool
	}{
		{"https://api.test/v1", true},
		{"https://API.test/v1/users", true},
		{"https://api.test/v10", false}, // a prefix of the text, not of the path
		{"https://api.test/v2", false},
		{"http://api.test/v1", false}, // another scheme
		{"https://evil.test/v1", false},
	}
	for _, tt := range tests {
		if got := f.Allows(mustURL(tt.base)); got != tt.want {
			t.Errorf("Allows(%s) = %v, want %v", tt.base, got, tt.want)
		}
	}
	if !(replay.Forward{}).Allows(mustURL("https://anywhere.test")) {
		t.Error("an empty list should allow every API")
	}
}
