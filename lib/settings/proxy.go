// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"strings"

	"golang.org/x/net/http/httpproxy"
)

// ProxyConfig decides which proxy outgoing requests use. A non-empty
// proxy.http setting is used for both HTTP and HTTPS. Otherwise the
// conventional HTTP_PROXY, HTTPS_PROXY and NO_PROXY variables (or their
// lower-case forms) are read from environ, so a machine's usual proxy
// setup works without Flow-I-specific configuration. NO_PROXY applies in
// both cases.
func ProxyConfig(proxySetting string, environ []string) *httpproxy.Config {
	get := environLookup(environ)
	cfg := &httpproxy.Config{
		HTTPProxy:  firstSet(get, "HTTP_PROXY", "http_proxy"),
		HTTPSProxy: firstSet(get, "HTTPS_PROXY", "https_proxy"),
		NoProxy:    firstSet(get, "NO_PROXY", "no_proxy"),
	}
	if proxySetting != "" {
		cfg.HTTPProxy, cfg.HTTPSProxy = proxySetting, proxySetting
	}
	return cfg
}

func environLookup(environ []string) func(string) string {
	values := make(map[string]string, len(environ))
	for _, kv := range environ {
		if k, v, ok := strings.Cut(kv, "="); ok {
			values[k] = v
		}
	}
	return func(k string) string { return values[k] }
}

func firstSet(get func(string) string, names ...string) string {
	for _, n := range names {
		if v := get(n); v != "" {
			return v
		}
	}
	return ""
}
