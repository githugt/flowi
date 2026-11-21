// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"net/url"
	"testing"
)

func TestLeavesOrigin_Given_redirect_targets_When_compared_Then_only_the_same_origin_keeps_credentials(t *testing.T) {
	tests := []struct {
		name, from, to string
		want           bool
	}{
		{"same origin", "https://a.test/x", "https://a.test/y", false},
		{"default https port spelled out", "https://a.test/x", "https://a.test:443/y", false},
		{"default http port spelled out", "http://a.test/x", "http://a.test:80/y", false},
		{"different port", "https://a.test/x", "https://a.test:8443/y", true},
		{"different host", "https://a.test/x", "https://b.test/y", true},
		{"https downgraded to http", "https://a.test/x", "http://a.test/y", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, _ := url.Parse(tt.from)
			to, _ := url.Parse(tt.to)
			if got := leavesOrigin(from, to); got != tt.want {
				t.Fatalf("leavesOrigin(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.want)
			}
		})
	}
}

func TestIsCredentialHeader_Given_custom_credential_names_When_checked_Then_they_are_recognised(t *testing.T) {
	for _, name := range []string{"X-Api-Key", "x-auth-token", "Proxy-Authorization", "Cookie", "X-Signature", "X-Session-Id"} {
		if !isCredentialHeader(name) {
			t.Errorf("%s should be treated as a credential", name)
		}
	}
	for _, name := range []string{"Accept", "Content-Type", "User-Agent"} {
		if isCredentialHeader(name) {
			t.Errorf("%s should not be treated as a credential", name)
		}
	}
}

func TestIsCredentialHeader_Given_jwt_pwd_and_apikey_names_Then_they_are_credentials(t *testing.T) {
	// Before: the redirect list lacked jwt, pwd and apikey, so these followed a redirect to another origin.
	for _, name := range []string{"X-Jwt", "X-Pwd", "Apikey", "X-Api-Key", "Authorization"} {
		if !isCredentialHeader(name) {
			t.Errorf("%q should be dropped on a cross-origin redirect", name)
		}
	}
	if isCredentialHeader("Accept") {
		t.Error("Accept is not a credential")
	}
}
