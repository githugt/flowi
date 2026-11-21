// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/internal/httpx"
)

func TestSeconds_Given_token_lifetimes_When_read_Then_numbers_and_strings_work_and_junk_is_refused(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want time.Duration
		ok   bool
	}{
		{"number", float64(3600), time.Hour, true},
		{"string from a lax server", "90", 90 * time.Second, true},
		{"zero", float64(0), 0, false},
		{"negative", float64(-5), 0, false},
		{"text", "soon", 0, false},
		{"missing", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := seconds(tt.in)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("seconds(%v) = %v, %v; want %v, %v", tt.in, got, ok, tt.want, tt.ok)
			}
		})
	}
	if got := lifetime(nil); got != defaultLifetime {
		t.Fatalf("a missing expires_in should fall back to %v, got %v", defaultLifetime, got)
	}
}

func TestOAuth2Redact_Given_an_error_quoting_credentials_When_redacted_Then_no_secret_survives(t *testing.T) {
	o := &oauth2{clientSecret: "s3cr3t-client", password: "hunter2-pass", refresh: "r-token-9"}
	err := o.redact(errors.New(`post failed: client_secret=s3cr3t-client password=hunter2-pass refresh_token=r-token-9`))
	for _, secret := range []string{o.clientSecret, o.password, o.refresh} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("%q leaked in %q", secret, err)
		}
	}
	if o.redact(nil) == nil {
		t.Fatal("a nil cause must still produce an error")
	}
}

func TestDigestHasher_Given_the_challenge_algorithm_When_chosen_Then_supported_ones_resolve_and_others_fail(t *testing.T) {
	for alg, want := range map[string]string{"": "MD5", "md5-sess": "MD5", "SHA-256": "SHA-256", "sha-256-sess": "SHA-256", "SHA-512-256": "SHA-512-256"} {
		d := &digest{challenge: &digestChallenge{algorithm: alg}}
		_, name, err := d.hasher()
		if err != nil || name != want {
			t.Errorf("algorithm %q: got %q, %v; want %q", alg, name, err, want)
		}
	}
	d := &digest{challenge: &digestChallenge{algorithm: "SHA-3"}}
	if _, _, err := d.hasher(); err == nil {
		t.Fatal("an unsupported algorithm must be refused, not silently downgraded")
	}
}

func TestFormParams_Given_request_bodies_When_signed_Then_only_a_form_body_is_covered(t *testing.T) {
	form := func(contentType, body string) *httpx.Request {
		r := &httpx.Request{Header: http.Header{}, Body: httpx.BytesBody([]byte(body), contentType)}
		r.Header.Set("Content-Type", contentType)
		return r
	}
	got, err := formParams(form("application/x-www-form-urlencoded; charset=utf-8", "a=1&b=two"))
	if err != nil || len(got) != 2 {
		t.Fatalf("form body: got %v, %v", got, err)
	}
	if got, err := formParams(form("application/json", `{"a":1}`)); err != nil || got != nil {
		t.Fatalf("a JSON body is not part of the signature: got %v, %v", got, err)
	}
}

func TestRequestTarget_Given_urls_When_built_Then_path_and_query_are_kept_verbatim(t *testing.T) {
	for in, want := range map[string]string{
		"https://a.test":             "/",
		"https://a.test/p%2Fq?x=1&y": "/p%2Fq?x=1&y",
		"https://a.test/p":           "/p",
	} {
		u, _ := url.Parse(in)
		if got := requestTarget(u); got != want {
			t.Errorf("requestTarget(%s) = %q, want %q", in, got, want)
		}
	}
}
