// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/app/internal/vm"
	flowi "github.com/githugt/flowi/lib"
)

// Before: the Authentications screen could not say whether a credential still worked.
func TestTokenStatus_should_say_whether_a_token_is_good_renews_itself_or_needs_a_new_sign_in(t *testing.T) {
	soon := time.Date(2026, 3, 4, 14, 32, 0, 0, time.Local)

	live := vm.TokenStatus(flowi.AuthToken{Expires: soon})
	renews := vm.TokenStatus(flowi.AuthToken{Expired: true, HasRefresh: true})
	gone := vm.TokenStatus(flowi.AuthToken{Expired: true})

	if live != "Signed in until 14:32, 4 Mar" || !strings.Contains(renews, "renews by itself") || !strings.Contains(gone, "asks you to sign in again") {
		t.Fatalf("%q / %q / %q", live, renews, gone)
	}
}
