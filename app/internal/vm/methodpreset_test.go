// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

func presetHeader(v model.RequestVersion, name string) (string, bool) {
	for _, h := range v.Headers {
		if strings.EqualFold(h.Key, name) {
			return h.Value, true
		}
	}
	return "", false
}

func TestApplyMethodPreset_should_fill_what_the_method_usually_needs(t *testing.T) {
	tests := []struct {
		method      string
		wantHeaders map[string]string
		wantBody    string // a fragment of the example body; "" for none
	}{
		{method: "PROPFIND", wantHeaders: map[string]string{"Depth": "1", "Content-Type": "application/xml"}, wantBody: "<D:propfind"},
		{method: "PROPPATCH", wantHeaders: map[string]string{"Content-Type": "application/xml"}, wantBody: "<D:propertyupdate"},
		{method: "REPORT", wantHeaders: map[string]string{"Depth": "1", "Content-Type": "application/xml"}, wantBody: "calendar-query"},
		{method: "MKCALENDAR", wantHeaders: map[string]string{"Content-Type": "application/xml"}, wantBody: "<C:mkcalendar"},
		{method: "COPY", wantHeaders: map[string]string{"Destination": "{{env.base_url}}/new-location", "Overwrite": "F"}},
		{method: "MOVE", wantHeaders: map[string]string{"Destination": "{{env.base_url}}/new-location", "Overwrite": "F"}},
		{method: "LOCK", wantHeaders: map[string]string{"Timeout": "Second-3600", "Content-Type": "application/xml"}, wantBody: "<D:lockinfo"},
		{method: "UNLOCK", wantHeaders: map[string]string{"Lock-Token": "<{{lock_token}}>"}},
		{method: "LINK", wantHeaders: map[string]string{"Link": `<{{env.base_url}}/target>; rel="related"`}},
		{method: "SEARCH", wantHeaders: map[string]string{"Content-Type": "application/xml"}, wantBody: "basicsearch"},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			v := model.RequestVersion{Method: tt.method}

			vm.ApplyMethodPreset(&v)

			for name, want := range tt.wantHeaders {
				if got, ok := presetHeader(v, name); !ok || got != want {
					t.Fatalf("header %s = %q (set %v), want %q", name, got, ok, want)
				}
			}
			if tt.wantBody != "" && !strings.Contains(v.Body.Text, tt.wantBody) {
				t.Fatalf("body %q lacks %q", v.Body.Text, tt.wantBody)
			}
			if tt.wantBody == "" && v.Body.Text != "" {
				t.Fatalf("body %q, want none", v.Body.Text)
			}
		})
	}
}

func TestApplyMethodPreset_Given_a_value_already_typed_Then_it_is_kept(t *testing.T) {
	v := model.RequestVersion{
		Method:  "PROPFIND",
		Headers: []model.KeyValue{{Key: "depth", Value: "0"}},
		Body:    model.Body{Text: "<mine/>"},
	}

	vm.ApplyMethodPreset(&v)

	if got, _ := presetHeader(v, "Depth"); got != "0" || len(v.Headers) != 1 || v.Body.Text != "<mine/>" {
		t.Fatalf("headers %+v body %q", v.Headers, v.Body.Text)
	}
}

func TestApplyMethodPreset_Given_a_body_of_another_kind_Then_no_example_and_no_content_type(t *testing.T) {
	v := model.RequestVersion{Method: "PROPFIND", Body: model.Body{JSON: `{"a":1}`}}

	vm.ApplyMethodPreset(&v)

	if _, ok := presetHeader(v, "Content-Type"); ok || v.Body.Text != "" {
		t.Fatalf("headers %+v body %+v", v.Headers, v.Body)
	}
}

func TestApplyMethodPreset_Given_other_methods_Then_nothing_changes(t *testing.T) {
	for _, method := range []string{"GET", "POST", "QUERY", "MKCOL", "Purge", "NOPE"} {
		v := model.RequestVersion{Method: method}

		vm.ApplyMethodPreset(&v)

		if len(v.Headers) != 0 || v.Body.Text != "" {
			t.Fatalf("%s: headers %+v body %q", method, v.Headers, v.Body.Text)
		}
	}
}
