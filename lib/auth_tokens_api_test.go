// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/authx"
)

func keepToken(t *testing.T, w *Workspace, key, label string, expires time.Time) {
	t.Helper()
	err := vaultTokens{w}.Save(key, authx.StoredToken{Label: label, Access: "ACCESS-SECRET", Refresh: "REFRESH-SECRET", Expires: expires})
	if err != nil {
		t.Fatal(err)
	}
}

// Before: an OAuth token lived in memory; the user could not see it, drop it, or keep it over a restart.
func TestAuthTokens_should_list_what_the_vault_kept_without_the_token_and_say_when_it_ran_out(t *testing.T) {
	w := demoWorkspace(t)
	w.opts.Environ = []string{"FLOWI_VAULT_KEY=test-passphrase"}
	w.opts.Now = func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) }
	keepToken(t, w, "aaaa1111bbbb2222", "device_code · auth.example.com · client tv", time.Date(2026, 1, 1, 13, 0, 0, 0, time.UTC))
	keepToken(t, w, "cccc3333dddd4444", "authorization_code · idp.example · client web", time.Date(2026, 1, 1, 11, 0, 0, 0, time.UTC))

	got := w.AuthTokens()

	// ordered by label
	if len(got) != 2 || got[1].ID != "aaaa1111" || got[1].Expired || !got[1].Stored || !got[1].HasRefresh ||
		got[0].ID != "cccc3333" || !got[0].Expired || got[0].Label != "authorization_code · idp.example · client web" {
		t.Fatalf("got %+v", got)
	}
	if text := strings.Join([]string{got[0].ID, got[0].Label, got[1].ID, got[1].Label}, " "); strings.Contains(text, "SECRET") {
		t.Errorf("a token leaked into the listing: %s", text)
	}
}

func TestForgetAuthToken_should_drop_a_token_by_the_start_of_its_id_and_refuse_an_unknown_or_ambiguous_one(t *testing.T) {
	w := demoWorkspace(t)
	w.opts.Environ = []string{"FLOWI_VAULT_KEY=test-passphrase"}
	keepToken(t, w, "aaaa1111bbbb2222", "one", time.Now().Add(time.Hour))
	keepToken(t, w, "aaaa9999bbbb2222", "two", time.Now().Add(time.Hour))
	keepToken(t, w, "eeee5555ffff6666", "three", time.Now().Add(time.Hour))

	ambiguous := w.ForgetAuthToken("aaaa")
	unknown := w.ForgetAuthToken("zzzz")
	empty := w.ForgetAuthToken("")
	one := w.ForgetAuthToken("eeee")
	left := w.AuthTokens()

	for name, err := range map[string]error{"ambiguous": ambiguous, "unknown": unknown, "empty": empty} {
		var le *diag.ListError
		if !errors.As(err, &le) || le.List[0].Code != diag.CodeAuthToken {
			t.Errorf("%s: %v", name, err)
		}
	}
	if one != nil || len(left) != 2 || left[0].Label != "one" || left[1].Label != "two" {
		t.Fatalf("one %v, left %+v", one, left)
	}
}

func TestPruneSecrets_should_leave_the_saved_tokens_alone(t *testing.T) {
	w := demoWorkspace(t)
	w.opts.Environ = []string{"FLOWI_VAULT_KEY=test-passphrase"}
	keepToken(t, w, "aaaa1111bbbb2222", "one", time.Now().Add(time.Hour))

	orphans, err := w.OrphanSecrets()
	_, pruneErr := w.PruneSecrets()

	if err != nil || pruneErr != nil || len(orphans) != 0 || len(w.AuthTokens()) != 1 {
		t.Fatalf("orphans %+v (%v), prune %v, tokens %+v", orphans, err, pruneErr, w.AuthTokens())
	}
}
