// SPDX-License-Identifier: Apache-2.0

package engine

import "strings"

// httpHints are the failures of an HTTP call that Go words in its own terms, with what to change in the request. The
// first phrase found (lower case) decides, so the narrow phrases come before the wide ones.
func httpHints() []struct{ phrase, hint string } {
	return []struct{ phrase, hint string }{
		{"no such host", "the host name in the URL cannot be found: check its spelling, and your network or VPN"},
		{"server misbehaving", "the DNS server did not answer well: check your network or VPN"},
		{"connection refused", "nothing is listening at that host and port: check the URL, and that the server is running"},
		{"server gave http response to https client", "the server speaks plain HTTP: start the URL with http:// instead of https://"},
		{"unsupported protocol scheme", "the URL must start with http:// or https://"},
		{"no route to host", "the host cannot be reached: check your network or VPN"},
		{"network is unreachable", "the host cannot be reached: check your network or VPN"},
		{"connection reset", "the server closed the connection: check that it speaks this protocol, and the TLS settings"},
		{"unexpected eof", "the server closed the connection: check that it speaks this protocol, and the TLS settings"},
		{"proxyconnect", "the proxy could not be reached: check the proxy settings"},
		{"x509", "the server's certificate is not trusted: check the URL, or add the certificate authority in the TLS settings"},
		{"certificate", "the server's certificate is not trusted: check the URL, or add the certificate authority in the TLS settings"},
		{"tls:", "the secure connection failed: check that the server takes TLS at this port, and the TLS settings"},
		{"i/o timeout", "the server did not answer: check the host and port, and any firewall or VPN"},
		{"deadline exceeded", "the server did not answer in time: check the host and port, or raise the request's timeout"},
	}
}

// httpHint adds what to change to the failure of an HTTP call, when it is one it knows. Before: a plain HTTP failure
// showed Go's own text (for example "dial tcp: lookup x: no such host") with no word about what to do.
func httpHint(detail string) string {
	low := strings.ToLower(detail)
	for _, h := range httpHints() {
		if strings.Contains(low, h.phrase) {
			return detail + ". To fix: " + h.hint
		}
	}
	return detail
}
