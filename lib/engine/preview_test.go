// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
)

func TestPreviewSend_should_resolve_variables_without_sending(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("p", model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/login",
		Headers: []model.KeyValue{{Key: "X-Id", Value: "{{email}}"}},
		Body:    model.Body{JSON: `{"email":"{{email}}"}`}})
	in := f.input(r)
	in.With = map[string]string{"email": "a@example.com"}

	got := PreviewSend(f.deps(), in, false)

	if got.Diags.HasErrors() || f.hits.Load() != 0 {
		t.Fatalf("diags %v hits %d", got.Diags, f.hits.Load())
	}
	m := got.Masked
	if m.Method != "POST" || !strings.HasSuffix(m.URL, "/login") || strings.Contains(m.URL, "{{") || m.Body != `{"email":"a@example.com"}` || m.BodyKind != "text" {
		t.Fatalf("preview %+v", m)
	}
	if len(m.Headers) == 0 || m.Headers[0].Name != "X-Id" || m.Headers[0].Value != "a@example.com" {
		t.Fatalf("headers %+v", m.Headers)
	}
	if got.Real != nil {
		t.Fatalf("real request returned without being asked: %+v", got.Real)
	}
}

func TestPreviewSend_Given_unknown_variable_or_generator_Then_kept_verbatim_without_error(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("p", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/{{nope}}/{{$no-such-fn}}",
		Query: []model.KeyValue{{Key: "q", Value: "{{missing.name}}"}}})

	got := PreviewSend(f.deps(), f.input(r), false)

	if got.Diags.HasErrors() {
		t.Fatalf("diags %v", got.Diags)
	}
	if !strings.Contains(got.Masked.URL, "{{nope}}") || !strings.Contains(got.Masked.URL, "{{$no-such-fn}}") {
		t.Fatalf("url %q", got.Masked.URL)
	}
}

func TestPreviewSend_should_mask_secrets_unless_real_values_are_requested(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("p", model.RequestVersion{Method: "POST", URL: "{{env.base_url}}/login",
		Headers: []model.KeyValue{{Key: "Authorization", Value: "Bearer {{secret.password}}"}}})

	masked := PreviewSend(f.deps(), f.input(r), false)
	withSecrets := PreviewSend(f.deps(), f.input(r), true)

	for _, h := range masked.Masked.Headers {
		if strings.Contains(h.Value, secretV) {
			t.Fatalf("secret in masked preview: %+v", masked.Masked.Headers)
		}
	}
	if withSecrets.Real == nil || withSecrets.Real.Headers[0].Value != "Bearer "+secretV {
		t.Fatalf("real %+v", withSecrets.Real)
	}
	if got := withSecrets.Masked.Headers[0].Value; !strings.Contains(got, vault.Mask) {
		t.Fatalf("masked form of the real preview %q", got)
	}
}

func TestPreviewSend_should_describe_form_multipart_and_file_bodies(t *testing.T) {
	f := newFixture(t)
	form := f.addRequest("form", model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{Form: []model.KeyValue{{Key: "a", Value: "1"}}}})
	multi := f.addRequest("multi", model.RequestVersion{Method: "POST", URL: "http://x", Body: model.Body{Multipart: []model.Field{{Key: "n", Value: "v"}}}})

	pf := PreviewSend(f.deps(), f.input(form), false).Masked
	pm := PreviewSend(f.deps(), f.input(multi), false).Masked

	if pf.BodyKind != "form" || pf.Body != "a=1" {
		t.Fatalf("form %+v", pf)
	}
	if pm.BodyKind != "multipart" || len(pm.Parts) != 1 || pm.Parts[0].Name != "n" || pm.Parts[0].Value != "v" {
		t.Fatalf("multipart %+v", pm)
	}
}

func TestPreviewSend_should_show_the_default_headers_and_the_clients_user_agent(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("p", model.RequestVersion{Method: "GET", URL: "{{env.base_url}}/x"})
	d := f.deps()
	d.UserAgent = "Flowi-UI/9"

	got := PreviewSend(d, f.input(r), false)

	seen := map[string]string{}
	for _, h := range got.Masked.Headers {
		seen[h.Name] = h.Value
	}
	if seen["Accept"] != "*/*" || seen["Cache-Control"] != "no-cache" || seen["User-Agent"] != "Flowi-UI/9" {
		t.Fatalf("headers %+v", got.Masked.Headers)
	}
}

func TestPreviewSend_should_use_request_variables_as_defaults_that_the_environment_overrides(t *testing.T) {
	f := newFixture(t)
	r := f.addRequest("p", model.RequestVersion{Method: "GET", URL: "http://x.test/{{var.id}}/{{var.base_url}}/{{id}}",
		Variables: []model.KeyValue{{Key: "id", Value: "7"}, {Key: "base_url", Value: "from-request"}}})

	got := PreviewSend(f.deps(), f.input(r), false)

	envBase := f.srv.URL
	if want := "http://x.test/7/" + envBase + "/7"; got.Masked.URL != want {
		t.Fatalf("url %q, want %q", got.Masked.URL, want)
	}
}
