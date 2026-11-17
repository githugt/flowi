// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"strings"
	"testing"
)

func TestDriverHint_Given_known_driver_failures_Then_says_what_to_change(t *testing.T) {
	for in, want := range map[string]string{
		"failed SASL auth: password authentication failed for user x": "secret",
		"dial tcp 127.0.0.1:5432: connect: connection refused":        "host and port",
		"no such table: rna": "table name",
	} {
		if got := driverHint(in); !strings.HasPrefix(got, in) || !strings.Contains(got, want) {
			t.Errorf("%q -> %q", in, got)
		}
	}
	if got := driverHint("syntax error"); got != "syntax error" {
		t.Errorf("unknown changed: %q", got)
	}
}
