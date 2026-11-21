// SPDX-License-Identifier: Apache-2.0

package credname

import "testing"

func TestSensitive(t *testing.T) {
	// Before: the redirect code had its own list without pwd, apikey and jwt.
	for _, name := range []string{"Authorization", "X-Api-Key", "x_jwt", "pwd", "access_token", "Set-Cookie", "client_secret", "privateKey"} {
		if !Sensitive(name) {
			t.Errorf("%q should be a credential name", name)
		}
	}
	for _, name := range []string{"Content-Type", "Accept", "author", "Authors", "keyboard", ""} {
		if Sensitive(name) {
			t.Errorf("%q should not be a credential name", name)
		}
	}
}

func TestSensitiveAnywhereCountsKeyInTheMiddle(t *testing.T) {
	if Sensitive("Key-Id") {
		t.Error("Sensitive only counts a trailing key")
	}
	if !SensitiveAnywhere("Key-Id") {
		t.Error("SensitiveAnywhere should count key anywhere")
	}
	if SensitiveAnywhere("Accept") {
		t.Error("Accept is not a credential")
	}
}
