// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
)

func headerValue(r *vm.Request, name string) string {
	for _, h := range r.Current().Headers {
		if h.Key == name {
			return h.Value
		}
	}
	return ""
}

func TestBodyVM_should_start_with_no_body(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	if r.BodyKind() != vm.BodyNone || r.BodyFormat() != vm.FormatJSON || r.Multipart() {
		t.Fatalf("kind %q format %q multipart %v", r.BodyKind(), r.BodyFormat(), r.Multipart())
	}
}

func TestBodyVM_SetBodyKind_text_should_keep_typed_text_and_pick_the_json_field(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.SetBodyKind(vm.BodyText)
	r.Edit(func(v *model.RequestVersion) { v.Body.JSON = `{"a":1}` })

	if r.BodyKind() != vm.BodyText || r.BodyFormat() != vm.FormatJSON {
		t.Fatalf("kind %q format %q", r.BodyKind(), r.BodyFormat())
	}
}

func TestBodyVM_SetBodyFormat_should_move_text_between_fields_and_set_the_content_type(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.SetBodyKind(vm.BodyText)
	r.Edit(func(v *model.RequestVersion) { v.Body.JSON = "<a/>" })

	r.SetBodyFormat(vm.FormatXML)
	xml := r.Current()
	xmlType := headerValue(r, "Content-Type")
	r.SetBodyFormat(vm.FormatYAML)
	yamlType := headerValue(r, "Content-Type")
	r.SetBodyFormat(vm.FormatRaw)
	rawType := headerValue(r, "Content-Type")
	r.SetBodyFormat(vm.FormatJSON)

	if xml.Body.Text != "<a/>" || xml.Body.JSON != "" || xmlType != "application/xml" || yamlType != "application/yaml" || rawType != "text/plain" {
		t.Fatalf("xml body %+v types %q %q %q", xml.Body, xmlType, yamlType, rawType)
	}
	if got := r.Current(); got.Body.JSON != "<a/>" || headerValue(r, "Content-Type") != "" {
		t.Fatalf("json again: body %+v header %q", got.Body, headerValue(r, "Content-Type"))
	}
}

func TestBodyVM_SetBodyFormat_should_not_overwrite_a_content_type_the_user_set(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.SetBodyKind(vm.BodyText)
	r.Edit(func(v *model.RequestVersion) {
		v.Body.Text = "x"
		v.Headers = []model.KeyValue{{Key: "Content-Type", Value: "application/vnd.custom+json"}}
	})

	r.SetBodyFormat(vm.FormatXML)

	if headerValue(r, "Content-Type") != "application/vnd.custom+json" {
		t.Fatalf("content type %q", headerValue(r, "Content-Type"))
	}
}

func TestBodyVM_SetBodyKind_should_clear_the_other_kinds(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.SetBodyKind(vm.BodyText)
	r.Edit(func(v *model.RequestVersion) { v.Body.JSON = "{}" })

	r.SetBodyKind(vm.BodyFile)
	r.Edit(func(v *model.RequestVersion) { v.Body.File = "a.bin" })
	r.SetBodyKind(vm.BodyForm)

	if b := r.Current().Body; b.JSON != "" || b.Text != "" || b.File != "" {
		t.Fatalf("body %+v", b)
	}
}

func TestBodyVM_SetMultipart_should_convert_form_rows_both_ways(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.SetBodyKind(vm.BodyForm)
	r.Edit(func(v *model.RequestVersion) { v.Body.Form = []model.KeyValue{{Key: "a", Value: "1"}} })

	r.SetMultipart(true)
	asMultipart := r.Current().Body
	on := r.Multipart()
	r.SetMultipart(false)
	back := r.Current().Body

	if !on || len(asMultipart.Form) != 0 || len(asMultipart.Multipart) != 1 || asMultipart.Multipart[0].Key != "a" || asMultipart.Multipart[0].Value != "1" {
		t.Fatalf("multipart %+v on %v", asMultipart, on)
	}
	if r.Multipart() || len(back.Multipart) != 0 || len(back.Form) != 1 || back.Form[0].Value != "1" {
		t.Fatalf("form %+v", back)
	}
}

func TestBodyVM_Given_a_json_body_on_disk_Then_reports_text_json(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Requests[0].Versions[0].Body = model.Body{JSON: "{}"}
	r, _ := newRequestVM(t, b)

	if r.BodyKind() != vm.BodyText || r.BodyFormat() != vm.FormatJSON {
		t.Fatalf("kind %q format %q", r.BodyKind(), r.BodyFormat())
	}
}

func TestBodyVM_Given_text_with_an_xml_content_type_Then_reports_xml(t *testing.T) {
	b := sampleBackend()
	v := &b.Cols[0].Requests[0].Versions[0]
	v.Body = model.Body{Text: "<a/>"}
	v.Headers = []model.KeyValue{{Key: "content-type", Value: "application/xml; charset=utf-8"}}
	r, _ := newRequestVM(t, b)

	if r.BodyFormat() != vm.FormatXML {
		t.Fatalf("format %q", r.BodyFormat())
	}
}

// Review finding 6: a managed Content-Type from the format picker survived
// the switch to Form, so the form was sent as text/plain (or without the
// multipart boundary).
func TestBodyVM_SetBodyKind_to_form_or_file_should_drop_the_managed_content_type(t *testing.T) {
	for _, kind := range []vm.BodyKind{vm.BodyForm, vm.BodyFile, vm.BodyNone} {
		r, _ := newRequestVM(t, sampleBackend())
		r.SetBodyKind(vm.BodyText)
		r.Edit(func(v *model.RequestVersion) { v.Body.JSON = "x" })
		r.SetBodyFormat(vm.FormatRaw)

		r.SetBodyKind(kind)

		if got := headerValue(r, "Content-Type"); got != "" {
			t.Errorf("kind %s: content type %q survived", kind, got)
		}
	}
}

func TestBodyVM_SetBodyKind_should_keep_a_content_type_the_user_wrote(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.Edit(func(v *model.RequestVersion) {
		v.Headers = []model.KeyValue{{Key: "Content-Type", Value: "application/vnd.x+json"}}
	})

	r.SetBodyKind(vm.BodyForm)

	if headerValue(r, "Content-Type") != "application/vnd.x+json" {
		t.Fatalf("content type %q", headerValue(r, "Content-Type"))
	}
}
