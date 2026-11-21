// SPDX-License-Identifier: Apache-2.0

package diag

import (
	"strings"
	"testing"
)

// errorsWithoutHint is how many error codes say what is wrong but not what to do. It may only go down: a new error
// code needs a hint, and writing one for an old code lowers this number.
// Before: nothing counted them, so error codes were added without a fix text.
const errorsWithoutHint = 75

func TestCatalogue_should_not_add_error_codes_without_a_hint(t *testing.T) {
	var missing []string
	for code, info := range codeTable() {
		if info.severity != SeverityError {
			continue
		}
		if _, hint, _ := strings.Cut(info.english, hintMarker); strings.TrimSpace(hint) == "" {
			missing = append(missing, string(code))
		}
	}

	if len(missing) > errorsWithoutHint {
		t.Fatalf("%d error codes have no hint (limit %d), so the user is told what is wrong but not what to do: %v", len(missing), errorsWithoutHint, missing)
	}
}

func TestCatalogue_should_not_leak_a_prefix_or_the_hint_marker_into_a_message(t *testing.T) {
	for code, info := range codeTable() {
		msg, _, _ := strings.Cut(info.english, hintMarker)
		if strings.HasPrefix(strings.ToLower(msg), "error:") || strings.Contains(msg, "hint:") {
			t.Errorf("%s: message %q carries a prefix or a marker", code, msg)
		}
	}
}
