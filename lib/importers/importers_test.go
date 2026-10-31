// SPDX-License-Identifier: Apache-2.0

package importers

import (
	"encoding/json"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/template"
)

func TestSensitiveHeader(t *testing.T) {
	for name, want := range map[string]bool{
		"Authorization": true, "cookie": true, "Proxy-Authorization": true, "X-Api-Key": true,
		"X-Auth-Token": true, "Session-Id": true, "X-Password": true, "x-client-secret": true,
		"Content-Type": false, "Accept": false, "X-Trace": false, "Keep-Alive": false,
	} {
		if got := SensitiveHeader(name); got != want {
			t.Errorf("SensitiveHeader(%q) = %v, want %v", name, got, want)
		}
	}
}

func TestItem_Redact_should_replace_the_value_and_report_it(t *testing.T) {
	it := &Item{Name: "login"}

	got := it.Redact("header Authorization", "Authorization")

	if got != "{{secret.authorization}}" || len(it.Diags) != 1 || it.Diags[0].Code != diag.CodeImportSensitive ||
		it.Diags[0].Args["placeholder"] != "{{secret.authorization}}" {
		t.Fatalf("got %q diags %v", got, it.Diags)
	}
}

func TestFileName_and_Unique(t *testing.T) {
	for in, want := range map[string]string{
		"Get A": "Get-A", "pets:list": "pets-list", "../x": "x", ".hidden": "hidden", "": "",
	} {
		if got := FileName(in); got != want {
			t.Errorf("FileName(%q) = %q, want %q", in, got, want)
		}
	}
	var names Names
	if a, b := names.Unique("x"), names.Unique("x"); a != "x" || b != "x-2" {
		t.Fatalf("got %q %q", a, b)
	}
}

// Imported text is literal: a "{{" in it must not become a template.
func TestLiteral_should_render_back_to_the_text(t *testing.T) {
	in := `{"tpl": "{{name}}", "x": "}}"}`

	tpl, err := template.Parse(Literal(in))
	if err != nil {
		t.Fatal(err)
	}
	got, err := tpl.Render(template.Scope{})

	if err != nil || got.Value != in {
		t.Fatalf("got %q %v", got.Value, err)
	}
}

func TestRedactJSON_should_replace_credential_values_at_any_depth(t *testing.T) {
	body := map[string]any{"user": map[string]any{"id": 1.0, "password": "pw"}, "access_token": "t",
		"items": []any{map[string]any{"api_key": "k", "name": "n"}}}

	got, n := RedactJSON(body)

	want := `{"access_token":"(redacted)","items":[{"api_key":"(redacted)","name":"n"}],"user":{"id":1,"password":"(redacted)"}}`
	if data, _ := json.Marshal(got); string(data) != want || n != 3 {
		t.Fatalf("got %s (%d)", data, n)
	}
	if body["access_token"] != "t" {
		t.Fatal("the input must not change")
	}
}

// Before: names were split only on - and _, so camelCase credentials
// (accessToken, clientSecret…) were written to files.
func TestSensitiveHeader_should_catch_camel_case_and_run_together_names(t *testing.T) {
	for _, name := range []string{"accessToken", "refreshToken", "idToken", "clientSecret", "sessionId", "adminPassword",
		"jwt", "X-AccessToken", "privateKey", "secretKey", "passwd", "credentials", "x-auth"} {
		if !SensitiveHeader(name) {
			t.Errorf("%s is a credential", name)
		}
	}
	for _, name := range []string{"Author", ":authority", "Content-Type", "Accept", "Keep-Alive", "email", "baseUrl", "x-trace-id"} {
		if SensitiveHeader(name) {
			t.Errorf("%s is not a credential", name)
		}
	}
}

func TestStripUserinfo(t *testing.T) {
	got, had := StripUserinfo("https://alice:hunter2@api.example.com/v1?x=1")

	if got != "https://api.example.com/v1?x=1" || !had {
		t.Fatalf("got %q %v", got, had)
	}
	if got, had := StripUserinfo("{{env.base}}/v1/a@b"); had || got != "{{env.base}}/v1/a@b" {
		t.Fatalf("an @ in the path is not userinfo: %q", got)
	}
}

func TestRedactBodyText_should_redact_json_and_form_bodies_but_keep_variables(t *testing.T) {
	it := &Item{}
	cases := map[string]string{
		`{"password":"pw","name":"Ada"}`:     `{"name":"Ada","password":"(redacted)"}`,
		`{"password":"{{secret.password}}"}`: `{"password":"{{secret.password}}"}`,
		"user=ada&password=pw":               "user=ada&password=(redacted)",
		"plain text with password pw":        "plain text with password pw",
	}
	for in, want := range cases {
		if got := it.RedactBodyText(in, "the body"); got != want {
			t.Errorf("RedactBodyText(%q) = %q, want %q", in, got, want)
		}
	}
}

// Before: every call probed name-2, name-3… from the start.
func TestNames_Unique_should_stay_linear(t *testing.T) {
	var n Names
	for i := 0; i < 50000; i++ {
		n.Unique("same")
	}
	if got := n.Unique("same"); got != "same-50001" {
		t.Fatalf("got %q", got)
	}
	if a, b := n.Unique("X"), n.Unique("x"); a != "X" || b != "x-2" {
		t.Fatalf("names differing only by case collide on case-insensitive disks: %q %q", a, b)
	}
}
