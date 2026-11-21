// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"errors"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: the CLI could only list environments, so a script could not prepare one; the variable's type had no flag.
func TestParseVarKind_should_read_the_four_kinds_and_refuse_the_rest_as_usage(t *testing.T) {
	want := map[string]model.VarKind{"": model.VarString, "string": model.VarString, "number": model.VarNumber, "boolean": model.VarBoolean, "json": model.VarJSON}

	for in, kind := range want {
		got, err := parseVarKind(in)

		if err != nil || got != kind {
			t.Errorf("%q: got %q, %v", in, got, err)
		}
	}
	_, err := parseVarKind("int")
	var usage *UsageError
	if !errors.As(err, &usage) {
		t.Fatalf("a bad --type must be a usage error: %v", err)
	}
}
