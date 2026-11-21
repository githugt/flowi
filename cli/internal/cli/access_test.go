// SPDX-License-Identifier: AGPL-3.0-or-later

package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	flowi "github.com/githugt/flowi/lib"
)

// Before: the access check could only print a table; CI had no file to show which user reached which step.
func TestWriteAccessReports_should_write_the_junit_file_a_check_that_found_an_exposure_too(t *testing.T) {
	// Arrange
	path := filepath.Join(t.TempDir(), "access.xml")
	rep := flowi.AccessReport{Flow: "notes", Owner: "alice", Steps: []flowi.AccessStep{{
		ID: "read", Owner: 200, Attempts: []flowi.AccessAttempt{{Identity: "bob", Status: 200, Verdict: flowi.AccessAllowed}},
	}}}

	// Act
	err := writeAccessReports([]string{"junit=" + path}, rep)

	// Assert
	data, readErr := os.ReadFile(path) //nolint:gosec // a path in the test's own temporary directory
	if err != nil || readErr != nil || !strings.Contains(string(data), `<failure message="bob reached read`) {
		t.Fatalf("%v %v\n%s", err, readErr, data)
	}
}

func TestCheckAccessReports_should_refuse_a_format_it_cannot_write(t *testing.T) {
	for _, bad := range []string{"json=out.json", "junit", "junit="} {
		if err := checkAccessReports([]string{bad}); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	if err := checkAccessReports([]string{"junit=out.xml"}); err != nil {
		t.Errorf("junit=out.xml refused: %v", err)
	}
}
