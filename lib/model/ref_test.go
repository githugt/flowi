// SPDX-License-Identifier: Apache-2.0

package model

import "testing"

func TestRef_Label_Given_a_path_hint_Then_names_the_path_before_the_id(t *testing.T) {
	// Before: a message about a deleted request showed only its id, which nobody sees anywhere else.
	tests := map[string]struct {
		ref  Ref
		want string
	}{
		"with a path": {Ref{ID: "01J", Path: "auth/login"}, `"auth/login" (id 01J)`},
		"without one": {Ref{ID: "01J"}, "id 01J"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got := tt.ref.Label()

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
