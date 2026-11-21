// SPDX-License-Identifier: Apache-2.0

package bugreport_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/bugreport"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/vault"
)

func sample() engine.Record {
	return engine.Record{
		Request: engine.SentRequest{
			Method: "POST", URL: "https://app:hunter2@api.test/login?user=ann&api_key=abc123&page=2",
			Header: []engine.Header{{Name: "Authorization", Value: "Bearer " + vault.Mask}, {Name: "Content-Type", Value: "application/json"}, {Name: "X-Api-Key", Value: "k-999"}},
			Body:   &engine.Body{Text: `{"user":"ann","password":"p4ss"}`, JSON: map[string]any{"user": "ann", "password": "p4ss"}},
		},
		Status: 401, Proto: "HTTP/2", Version: "2", Timings: engine.Timings{Total: 142 * time.Millisecond},
		Header: []engine.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Set-Cookie", Value: "sid=zzz; HttpOnly"}},
		Body:   engine.Body{Size: 40, JSON: map[string]any{"error": "bad login", "access_token": "tok-123", "items": []any{map[string]any{"refresh_token": "r1"}}}},
	}
}

func TestBuild_should_mask_credentials_by_name_and_list_what_it_masked(t *testing.T) {
	rep := bugreport.Build(sample(), bugreport.Options{})

	for _, leaked := range []string{"hunter2", "abc123", "k-999", "p4ss", "sid=zzz", "tok-123", "r1", vault.Mask} {
		if strings.Contains(rep.Markdown, leaked) {
			t.Errorf("%q leaked:\n%s", leaked, rep.Markdown)
		}
	}
	for _, kept := range []string{"### POST https://api.test/login?user=ann&api_key=(redacted)&page=2 → 401 Unauthorized", "took 142 ms", "version 2", "Content-Type: application/json", `"error": "bad login"`, "curl -X POST", "Bearer <secret>"} {
		if !strings.Contains(rep.Markdown, kept) {
			t.Errorf("missing %q:\n%s", kept, rep.Markdown)
		}
	}
	for _, want := range []string{"login in the URL", "api_key query parameter", "X-Api-Key header (request)", "Set-Cookie header (response)", "Flowi secrets"} {
		if !slices.Contains(rep.Masked, want) {
			t.Errorf("masked list lacks %q: %v", want, rep.Masked)
		}
	}
	if !strings.Contains(rep.Markdown, "> Masked before copying: ") {
		t.Errorf("no visible summary:\n%s", rep.Markdown)
	}
}

func TestBuild_Given_nothing_sensitive_Then_it_says_so(t *testing.T) {
	rec := engine.Record{Request: engine.SentRequest{Method: "GET", URL: "https://api.test/health"}, Status: 200, Body: engine.Body{Text: "ok", Size: 2}}

	rep := bugreport.Build(rec, bugreport.Options{})

	if len(rep.Masked) != 0 || !strings.Contains(rep.Markdown, "Nothing here looked like a credential") || !strings.Contains(rep.Markdown, "### GET https://api.test/health → 200 OK") {
		t.Fatalf("%v\n%s", rep.Masked, rep.Markdown)
	}
}

func TestBuild_should_cap_a_large_body_on_a_character_boundary_and_say_how_much_was_left_out(t *testing.T) {
	rec := engine.Record{Request: engine.SentRequest{Method: "GET", URL: "https://a.test"}, Status: 200, Body: engine.Body{Text: strings.Repeat("é", 5000), Size: 10000}}

	rep := bugreport.Build(rec, bugreport.Options{MaxBody: 101})

	if !strings.Contains(rep.Markdown, "more bytes not shown") || strings.Contains(rep.Markdown, "\uFFFD") {
		t.Fatalf("%s", rep.Markdown)
	}
	if n := strings.Count(rep.Markdown, "é"); n > 51 {
		t.Fatalf("kept %d letters of a 50-letter budget", n)
	}
}

func TestBuild_should_not_let_a_body_close_its_own_code_fence(t *testing.T) {
	rec := engine.Record{Request: engine.SentRequest{Method: "GET", URL: "https://a.test"}, Status: 200, Body: engine.Body{Text: "x\n```\n# not a heading\n```", Size: 20}}

	rep := bugreport.Build(rec, bugreport.Options{})

	if !strings.Contains(rep.Markdown, "````\nx\n```\n# not a heading\n```\n````") {
		t.Fatalf("%s", rep.Markdown)
	}
}

func TestBuild_should_list_a_binary_body_without_its_bytes_and_cap_the_headers(t *testing.T) {
	rec := engine.Record{Request: engine.SentRequest{Method: "GET", URL: "https://a.test"}, Status: 200, Body: engine.Body{Base64: "AAAA", Size: 3}}
	for i := range 30 {
		rec.Header = append(rec.Header, engine.Header{Name: "X-H" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Value: "v"})
	}

	rep := bugreport.Build(rec, bugreport.Options{MaxHeaders: 5})

	if !strings.Contains(rep.Markdown, "Binary or file content, 3 bytes, not included.") || !strings.Contains(rep.Markdown, "… 25 more") || strings.Contains(rep.Markdown, "AAAA") {
		t.Fatalf("%s", rep.Markdown)
	}
}

func TestBuild_should_keep_a_template_reference_in_a_credential_field(t *testing.T) {
	rec := engine.Record{Request: engine.SentRequest{Method: "GET", URL: "https://a.test", Header: []engine.Header{{Name: "Authorization", Value: "{{secret.token}}"}}}, Status: 200}

	rep := bugreport.Build(rec, bugreport.Options{})

	if !strings.Contains(rep.Markdown, "Authorization: {{secret.token}}") || slices.Contains(rep.Masked, "Authorization header (request)") {
		t.Fatalf("%v\n%s", rep.Masked, rep.Markdown)
	}
}
