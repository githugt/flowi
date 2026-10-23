// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"
)

func TestRequestService_SetSchemasInfer_should_keep_only_what_differs_from_the_collection(t *testing.T) {
	tests := []struct {
		name    string
		turns   []bool
		wantSet bool // the version keeps a schemas.infer of its own
		wantOn  bool
	}{
		{name: "turned off", turns: []bool{false}, wantSet: true, wantOn: false},
		{name: "turned off, then on again", turns: []bool{false, true}, wantSet: false},
		{name: "turned on while the collection has it on", turns: []bool{true}, wantSet: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, v, _, _ := openLogin(t)

			for _, on := range tt.turns {
				reqs.SetSchemasInfer(v.Node, on)
			}
			got, _ := reqs.Open(v.Node)

			own, set := got.Current.Settings["schemas.infer"]
			if set != tt.wantSet || (set && own != tt.wantOn) {
				t.Fatalf("settings %v, want schemas.infer set %v (to %v)", got.Current.Settings, tt.wantSet, tt.wantOn)
			}
		})
	}
}
