// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"errors"
	"strings"
	"testing"
)

func TestHTTPHint_should_add_what_to_change_for_the_failures_people_meet(t *testing.T) {
	cases := []struct{ detail, want string }{
		{`Get "http://nope.test/": dial tcp: lookup nope.test: no such host`, "check its spelling"},
		{`dial tcp 127.0.0.1:1: connect: connection refused`, "nothing is listening"},
		{`Get "https://x.test": tls: failed to verify certificate: x509: certificate signed by unknown authority`, "certificate authority"},
		{`http: server gave HTTP response to HTTPS client`, "http:// instead of https://"},
		{`unsupported protocol scheme "ftp"`, "http:// or https://"},
		{`read tcp 10.0.0.1:5000->10.0.0.2:443: i/o timeout`, "firewall or VPN"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			got := httpHint(c.detail)

			if !strings.HasPrefix(got, c.detail+". To fix: ") || !strings.Contains(got, c.want) {
				t.Fatalf("got %q, want the detail then a hint with %q", got, c.want)
			}
		})
	}
}

func TestHTTPHint_should_leave_an_unknown_failure_as_it_is(t *testing.T) {
	if got := httpHint("something odd"); got != "something odd" {
		t.Fatalf("got %q", got)
	}
}

func TestTransportDiag_should_carry_the_hint(t *testing.T) {
	d := transportDiag(errors.New("dial tcp: lookup nope.test: no such host"))

	if got := d.Args["detail"]; !strings.Contains(got, "To fix: ") {
		t.Fatalf("detail %q lacks a hint", got)
	}
}
