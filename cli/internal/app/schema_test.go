// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestSchema_should_list_the_kinds_and_print_one_schema(t *testing.T) {
	// Before: no stable way to get a schema for an editor or a CI step.
	e := newSendEnv(t)

	code, out, errOut := e.run(context.Background(), "schema")
	if code != 0 || !strings.Contains(out, "request") || strings.Contains(out, "common") {
		t.Fatalf("list: exit %d\n%s\n%s", code, out, errOut)
	}
	code, out, errOut = e.run(context.Background(), "schema", "request")
	if code != 0 || !json.Valid([]byte(out)) {
		t.Fatalf("print: exit %d\n%s\n%s", code, out, errOut)
	}
	code, out, _ = e.run(context.Background(), "schema", "--modeline", "flow")
	if code != 0 || !strings.HasPrefix(out, "# yaml-language-server: $schema=") {
		t.Fatalf("modeline: exit %d\n%s", code, out)
	}
}

func TestSchema_Given_an_unknown_kind_Then_a_usage_error(t *testing.T) {
	e := newSendEnv(t)

	code, _, errOut := e.run(context.Background(), "schema", "nope")

	if code != 2 || !strings.Contains(errOut, "no schema for") {
		t.Fatalf("exit %d\n%s", code, errOut)
	}
}
