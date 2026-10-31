// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"net/url"
	"testing"
)

func proxyFor(t *testing.T, setting string, environ []string, target string) string {
	t.Helper()
	u, err := url.Parse(target)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ProxyConfig(setting, environ).ProxyFunc()(u)
	if err != nil {
		t.Fatal(err)
	}
	if p == nil {
		return ""
	}
	return p.String()
}

func TestProxyConfig_Given_setting_Then_used_for_http_and_https(t *testing.T) {
	env := []string{"HTTPS_PROXY=http://ignored:1", "NO_PROXY=internal.example"}

	httpProxy := proxyFor(t, "http://proxy.example:3128", env, "http://api.example/x")
	httpsProxy := proxyFor(t, "http://proxy.example:3128", env, "https://api.example/x")
	bypassed := proxyFor(t, "http://proxy.example:3128", env, "https://internal.example/x")

	if httpProxy != "http://proxy.example:3128" || httpsProxy != "http://proxy.example:3128" {
		t.Fatalf("got %q / %q", httpProxy, httpsProxy)
	}
	if bypassed != "" {
		t.Fatalf("NO_PROXY ignored: %q", bypassed)
	}
}

func TestProxyConfig_Given_no_setting_Then_reads_HTTPS_PROXY_and_NO_PROXY_from_environ(t *testing.T) {
	tests := []struct {
		name    string
		environ []string
		target  string
		want    string
	}{
		{"upper https", []string{"HTTPS_PROXY=http://p:1"}, "https://api.example/", "http://p:1"},
		{"lower https", []string{"https_proxy=http://p:2"}, "https://api.example/", "http://p:2"},
		{"http", []string{"HTTP_PROXY=http://p:3"}, "http://api.example/", "http://p:3"},
		{"no proxy", []string{"HTTPS_PROXY=http://p:1", "no_proxy=api.example"}, "https://api.example/", ""},
		{"nothing", nil, "https://api.example/", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := proxyFor(t, "", tt.environ, tt.target)

			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
