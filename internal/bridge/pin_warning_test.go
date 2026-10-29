// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"testing"
)

func TestPinWarningText_should_name_the_headers_and_say_why(t *testing.T) {
	// Act
	got := pinWarningText([]string{"Set-Cookie", "X-Auth-Token"})

	// Assert
	if !strings.Contains(got, "Set-Cookie, X-Auth-Token") || !strings.Contains(got, "committed") {
		t.Errorf("got %q", got)
	}
}

func TestPinWarningText_should_be_empty_without_credential_headers(t *testing.T) {
	// Act
	got := pinWarningText(nil)

	// Assert
	if got != "" {
		t.Errorf("got %q", got)
	}
}
