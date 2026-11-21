// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Before: nothing read expect:, so a typo in it was never reported.
func TestExpect_should_report_what_is_not_a_status_and_what_has_no_status(t *testing.T) {
	tests := []struct {
		name string
		v    model.RequestVersion
		want bool
	}{
		{name: "none", v: model.RequestVersion{Method: "GET", URL: "https://example.test"}},
		{name: "a_status", v: model.RequestVersion{Method: "GET", URL: "https://example.test", Expect: "401"}},
		{name: "a_class_and_a_list", v: model.RequestVersion{Method: "GET", URL: "https://example.test", Expect: "2xx, 404"}},
		{name: "a_word", v: model.RequestVersion{Method: "GET", URL: "https://example.test", Expect: "unauthorized"}, want: true},
		{name: "a_message_request", v: model.RequestVersion{Async: &model.Async{Protocol: "mqtt", Server: "mqtt://x", Channel: "c", Send: &model.AsyncSend{}}, Expect: "200"}, want: true},
		{name: "a_grpc_call", v: model.RequestVersion{GRPC: &model.GRPC{}, Expect: "200"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			tt.v.Label = "1"
			c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "01JA00000000000000000000E1", Name: "r"}, Path: "r", Versions: []model.RequestVersion{tt.v}})

			got := Validate(Target{Collections: []model.Collection{c}})

			var found []diag.Diagnostic
			for _, d := range got {
				if d.Code == diag.CodeExpectInvalid {
					found = append(found, d)
				}
			}
			if tt.want != (len(found) == 1) || len(found) > 1 {
				t.Fatalf("want a problem: %v, got %+v", tt.want, found)
			}
			if tt.want && found[0].Path != "$.versions.1.expect" {
				t.Fatalf("path %s", found[0].Path)
			}
		})
	}
}
