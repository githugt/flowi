// SPDX-License-Identifier: Apache-2.0

package trigger_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/serve/runs"
	"github.com/githugt/flowi/lib/serve/tokens"
	"github.com/githugt/flowi/lib/serve/trigger"
)

// scopedRig serves with a token store (and the legacy token too when legacy is set).
func scopedRig(t *testing.T, legacy bool) (*rig, *tokens.Store, *time.Time) {
	t.Helper()
	now := time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	store := tokens.Open(filepath.Join(t.TempDir(), "tokens.json"), func() time.Time { return now })
	r := newRig(t, func(to *trigger.Options, ro *runs.Options) {
		ro.Max = 10 // the fake runs never finish
		to.Tokens = store
		if !legacy {
			to.Token = ""
		}
	})
	return r, store, &now
}

func bearer(secret string) []string { return []string{"Authorization", "Bearer " + secret} }

func TestTrigger_Given_a_scoped_token_Then_it_starts_only_the_flows_and_environments_it_was_made_for(t *testing.T) {
	// Before: one token had full power: a CI job that needed one smoke flow in staging could run any flow in production.
	r, store, _ := scopedRig(t, false)
	_, secret, err := store.New("ci-smoke", tokens.Scopes{Flows: []string{"smoke/*"}, Envs: []string{"staging"}}, 0)
	if err != nil {
		t.Fatal(err)
	}

	ok := r.do("POST", "/flows/smoke/login/run", `{"env":"staging"}`, bearer(secret)...)
	otherFlow := r.do("POST", "/flows/admin/wipe/run", `{"env":"staging"}`, bearer(secret)...)
	otherEnv := r.do("POST", "/flows/smoke/login/run", `{"env":"prod"}`, bearer(secret)...)
	noEnv := r.do("POST", "/flows/smoke/login/run", ``, bearer(secret)...)
	byID := r.do("POST", "/flows/01JA00000000000000000000F1/run", `{"env":"staging"}`, bearer(secret)...)

	if ok.status != 202 {
		t.Fatalf("in scope: %d %s", ok.status, ok.body)
	}
	for name, p := range map[string]reply{"other flow": otherFlow, "other environment": otherEnv, "no environment": noEnv, "flow by id": byID} {
		if p.status != 403 {
			t.Errorf("%s: %d %s", name, p.status, p.body)
		}
	}
	if len(r.started) != 1 || r.started[0].Source != "trigger:ci-smoke" || r.started[0].Env != "staging" {
		t.Fatalf("only the one in-scope call started a run: %+v", r.started)
	}
	if !strings.Contains(noEnv.body, "staging") {
		t.Errorf("the refusal should name the allowed environments: %s", noEnv.body)
	}
}

func TestTrigger_Given_an_unrestricted_named_token_Then_it_starts_anything_and_sees_every_run(t *testing.T) {
	r, store, _ := scopedRig(t, true)
	_, secret, _ := store.New("ops", tokens.Scopes{}, 0)
	legacyRun := r.do("POST", "/flows/a/run", ``) // the legacy token

	started := r.do("POST", "/flows/anything/run", `{"env":"prod"}`, bearer(secret)...)
	seen := r.do("GET", "/runs/"+legacyRun.json(t)["run_id"].(string), "", bearer(secret)...)

	if started.status != 202 || seen.status != 200 {
		t.Fatalf("started %d %s, seen %d %s", started.status, started.body, seen.status, seen.body)
	}
}

func TestTrigger_Given_a_scoped_token_Then_it_reads_only_its_own_runs(t *testing.T) {
	r, store, _ := scopedRig(t, true)
	_, a, _ := store.New("a", tokens.Scopes{Flows: []string{"*"}}, 0)
	_, b, _ := store.New("b", tokens.Scopes{Flows: []string{"*"}}, 0)
	mine := r.do("POST", "/flows/x/run", ``, bearer(a)...).json(t)["run_id"].(string)
	theirs := r.do("POST", "/flows/x/run", ``, bearer(b)...).json(t)["run_id"].(string)
	legacy := r.do("POST", "/flows/x/run", ``).json(t)["run_id"].(string)

	own := r.do("GET", "/runs/"+mine, "", bearer(a)...)
	other := r.do("GET", "/runs/"+theirs, "", bearer(a)...)
	fromLegacy := r.do("GET", "/runs/"+legacy, "", bearer(a)...)
	full := r.do("GET", "/runs/"+mine, "") // the legacy token has no limit

	if own.status != 200 || other.status != 404 || fromLegacy.status != 404 || full.status != 200 {
		t.Fatalf("own %d, other %d, legacy %d, full %d", own.status, other.status, fromLegacy.status, full.status)
	}
}

func TestTrigger_Given_an_expired_token_Then_401_that_says_so_and_a_rotated_one_works_at_once(t *testing.T) {
	r, store, now := scopedRig(t, false)
	_, secret, _ := store.New("short", tokens.Scopes{}, time.Hour)
	*now = now.Add(2 * time.Hour)

	expired := r.do("POST", "/flows/x/run", ``, bearer(secret)...)
	_, fresh, err := store.Rotate("short")
	revived := r.do("POST", "/flows/x/run", ``, bearer(fresh)...)
	stale := r.do("POST", "/flows/x/run", ``, bearer(secret)...)

	if err != nil || expired.status != 401 || !strings.Contains(expired.body, "expired") {
		t.Fatalf("expired %d %s (%v)", expired.status, expired.body, err)
	}
	if revived.status != 202 || stale.status != 401 || strings.Contains(stale.body, "expired") {
		t.Fatalf("rotated: %d, old secret: %d %s", revived.status, stale.status, stale.body)
	}
}

func TestTrigger_Given_a_revoked_token_Then_it_is_refused_without_restarting_anything(t *testing.T) {
	r, store, _ := scopedRig(t, false)
	_, secret, _ := store.New("temp", tokens.Scopes{}, 0)
	before := r.do("POST", "/flows/x/run", ``, bearer(secret)...)

	if err := store.Revoke("temp"); err != nil {
		t.Fatal(err)
	}
	after := r.do("POST", "/flows/x/run", ``, bearer(secret)...)

	if before.status != 202 || after.status != 401 {
		t.Fatalf("before %d, after %d", before.status, after.status)
	}
}

func TestTrigger_New_Given_neither_a_token_nor_a_store_Then_it_is_refused(t *testing.T) {
	_, err := trigger.New(trigger.Options{})

	if err == nil {
		t.Fatal("triggers without authentication must never be offered")
	}
}
