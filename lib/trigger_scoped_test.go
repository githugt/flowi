// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/workspace"
)

func TestScopedTriggerToken_should_be_listed_without_its_secret_and_after_the_legacy_one(t *testing.T) {
	// Before: the one token had full power and no expiry, and there was no way to tell callers apart.
	w, _ := tokenWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}

	secret, info, err := w.NewScopedTriggerToken("ci", []string{"shared/*"}, []string{"staging"}, 24*time.Hour)
	list, listErr := w.TriggerTokens()

	if err != nil || listErr != nil {
		t.Fatal(err, listErr)
	}
	if !strings.HasPrefix(secret, "flwt_") || info.Name != "ci" || info.Expires.IsZero() {
		t.Fatalf("secret %q info %+v", secret, info)
	}
	if len(list) != 2 || !list[0].Legacy || list[0].Name != "default" || list[1].Name != "ci" || list[1].Flows[0] != "shared/*" {
		t.Fatalf("list %+v", list)
	}
}

func TestScopedTriggerToken_Given_a_bad_or_taken_name_Then_it_is_a_diagnostic(t *testing.T) {
	w, _ := tokenWorkspace(t)
	if _, _, err := w.NewScopedTriggerToken("ci", nil, nil, 0); err != nil {
		t.Fatal(err)
	}

	_, _, taken := w.NewScopedTriggerToken("ci", nil, nil, 0)
	_, _, bad := w.NewScopedTriggerToken("Not A Name", nil, nil, 0)
	_, _, reserved := w.NewScopedTriggerToken("default", nil, nil, 0)
	revoke := w.RevokeTriggerToken("nobody")

	for _, err := range []error{taken, bad, reserved, revoke} {
		var le *diag.ListError
		if !errors.As(err, &le) || le.List[0].Code != diag.CodeServeToken {
			t.Errorf("want FLW-SERVE-003, got %v", err)
		}
	}
}

func TestScopedTriggerToken_should_rotate_and_revoke_by_name_and_remove_the_legacy_one(t *testing.T) {
	w, _ := tokenWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}
	first, _, _ := w.NewScopedTriggerToken("ci", nil, nil, 0)

	second, _, err := w.RotateTriggerToken("ci")
	revoked := w.RevokeTriggerToken("ci")
	legacy := w.RevokeTriggerToken("default")
	list, _ := w.TriggerTokens()

	if err != nil || second == first || revoked != nil || legacy != nil {
		t.Fatalf("rotate %v revoke %v %v same %v", err, revoked, legacy, second == first)
	}
	if len(list) != 0 {
		t.Fatalf("left %+v", list)
	}
}

func TestScopedTriggerToken_Given_the_token_file_Then_it_holds_no_secret_and_is_private(t *testing.T) {
	w, e := tokenWorkspace(t)
	secret, _, _ := w.NewScopedTriggerToken("ci", nil, nil, 0)

	data, err := os.ReadFile(filepath.Join(workspace.Dir(e.opts.Paths, workspace.DefaultName), "trigger_tokens.json"))
	info, _ := os.Stat(filepath.Join(workspace.Dir(e.opts.Paths, workspace.DefaultName), "trigger_tokens.json"))

	if err != nil || strings.Contains(string(data), secret) || strings.Contains(string(data), strings.TrimPrefix(secret, "flwt_")) {
		t.Fatalf("file %s err %v", data, err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 { // Windows has no owner-only mode bits
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestServe_Given_only_a_scoped_token_Then_triggers_are_on_and_the_scopes_hold(t *testing.T) {
	// Before: triggers stayed off unless the single full-power token existed.
	w := demoWorkspace(t)
	secret, _, err := w.NewScopedTriggerToken("ci", []string{"shared/login"}, []string{"local"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoReplay = true })
	base := "http://" + s.TriggerAddr()
	body := `{"env": "local", "inputs": {"email": "ada@example.com"}}`

	ok, _, _ := call(t, "POST", base+"/flows/shared/login/run?wait=30s", secret, body)
	otherFlow, _, _ := call(t, "POST", base+"/flows/shared/other/run", secret, body)
	otherEnv, _, _ := call(t, "POST", base+"/flows/shared/login/run", secret, `{"env": "prod"}`)
	noEnv, _, _ := call(t, "POST", base+"/flows/shared/login/run", secret, `{}`)
	anon, _, _ := call(t, "POST", base+"/flows/shared/login/run", "", body)

	if ok != http.StatusOK {
		t.Errorf("in scope: %d", ok)
	}
	for name, got := range map[string]int{"other flow": otherFlow, "other env": otherEnv, "no env": noEnv} {
		if got != http.StatusForbidden {
			t.Errorf("%s: %d, want 403", name, got)
		}
	}
	if anon != http.StatusUnauthorized {
		t.Errorf("no token: %d", anon)
	}
}

func TestServe_Given_a_token_revoked_while_running_Then_it_stops_working_without_a_restart(t *testing.T) {
	w := demoWorkspace(t)
	secret, _, _ := w.NewScopedTriggerToken("ci", nil, nil, 0)
	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoReplay = true })
	url := "http://" + s.TriggerAddr() + "/flows/no-such-flow/run"
	before, _, _ := call(t, "POST", url, secret, "")

	if err := w.RevokeTriggerToken("ci"); err != nil {
		t.Fatal(err)
	}
	after, _, _ := call(t, "POST", url, secret, "")

	if before != http.StatusNotFound || after != http.StatusUnauthorized {
		t.Fatalf("before %d after %d", before, after)
	}
}

func TestServe_Given_no_token_at_all_Then_the_note_names_the_command_that_makes_one(t *testing.T) {
	w := demoWorkspace(t)

	s := serveOnFreePorts(t, w, func(o *ServeOptions) { o.NoReplay = true })

	if s.TriggerAddr() != "" || !strings.Contains(strings.Join(s.Notes(), "\n"), "flowi serve tokens new") {
		t.Fatalf("addr %q notes %q", s.TriggerAddr(), s.Notes())
	}
}
