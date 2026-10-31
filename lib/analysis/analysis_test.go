// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
)

// workspaceCopy copies the usage fixtures to a temp dir so a test can edit
// examples, and loads them.
type workspaceCopy struct {
	t    *testing.T
	dir  string
	data string
}

func newWorkspaceCopy(t *testing.T) *workspaceCopy {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("testdata/usage")); err != nil {
		t.Fatal(err)
	}
	return &workspaceCopy{t: t, dir: dir, data: t.TempDir()}
}

func (w *workspaceCopy) write(rel, content string) {
	w.t.Helper()
	file := filepath.Join(w.dir, rel)
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		w.t.Fatal(err)
	}
}

func (w *workspaceCopy) load() []model.Collection {
	w.t.Helper()
	s := store.New(yamlcodec.New(settings.Schema()))
	var out []model.Collection
	for _, name := range []string{"identity", "onboarding"} {
		c, diags, err := s.Load(filepath.Join(w.dir, name))
		if err != nil || diags.HasErrors() {
			w.t.Fatalf("loading %s: %v %v", name, err, diags)
		}
		out = append(out, c)
	}
	return out
}

func (w *workspaceCopy) contracts() Contracts {
	return FileContracts{DataDir: func(id model.ID) string { return filepath.Join(w.data, string(id)) }}
}

func (w *workspaceCopy) impact(request model.ID, version string) Report {
	w.t.Helper()
	got, _, err := Impact(w.load(), Target{Collection: identityID, Request: request}, version, w.contracts())
	if err != nil {
		w.t.Fatal(err)
	}
	return got
}

// tokenRemoved is login's v2 example without "token".
const tokenRemoved = `{"version": "2", "status": 200,
  "headers": [{"name": "X-Request-Id", "value": "r2"}, {"name": "Set-Cookie", "value": "sid=1"}],
  "body": {"user": {"id": "u"}}}`

func consumerStatus(r Report, match func(Consumer) bool) (Status, bool) {
	for _, v := range r.Versions {
		for _, c := range v.Consumers {
			if match(c) {
				return c.Status, true
			}
		}
	}
	return "", false
}

func TestImpact_Given_unchanged_examples_Then_signup_compatible(t *testing.T) {
	w := newWorkspaceCopy(t)

	got := w.impact(loginID, "2")

	signup, _ := consumerStatus(got, nodeIn("signup", "login"))
	hook, _ := consumerStatus(got, hookLabelled("check-login"))
	if signup != Compatible || hook != Compatible || len(got.Versions) != 1 || got.Versions[0].Contract.Source != SourceExample {
		t.Fatalf("got %+v", got)
	}
}

func TestImpact_Given_field_removed_from_example_Then_node_and_hook_broken(t *testing.T) {
	w := newWorkspaceCopy(t)
	w.write("identity/requests/auth/login.examples/v2-200.json", tokenRemoved)

	got := w.impact(loginID, "2")

	signup, _ := consumerStatus(got, nodeIn("signup", "login"))
	hook, _ := consumerStatus(got, hookLabelled("check-login"))
	versionHook, _ := consumerStatus(got, hookLabelled("version 2"))
	if signup != Broken || hook != Broken || versionHook != Compatible || got.Status != Broken {
		t.Fatalf("signup %s hook %s version hook %s report %s", signup, hook, versionHook, got.Status)
	}
}

func TestImpact_Given_other_versions_example_has_field_Then_still_broken(t *testing.T) {
	w := newWorkspaceCopy(t)
	w.write("identity/requests/auth/login.examples/v2-200.json", tokenRemoved)
	w.write("identity/requests/auth/login.examples/200.json", `{"version": "1", "status": 200, "body": {"token": "t", "data": {"token": "t", "sub": "s"}}}`)

	got := w.impact(loginID, "2")

	if status, _ := consumerStatus(got, nodeIn("signup", "login")); status != Broken {
		t.Fatalf("signup %s, want broken", status)
	}
}

func TestImpact_Given_no_contract_Then_unknown(t *testing.T) {
	w := newWorkspaceCopy(t)

	got := w.impact(profileID, "")

	audit, ok := consumerStatus(got, hookLabelled("collection identity"))
	me, _ := consumerStatus(got, nodeIn("local", "me"))
	if !ok || audit != Unknown || me != Compatible || got.Status != Unknown || got.Versions[0].Contract.Source != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestImpact_Given_saved_response_Then_used_as_contract(t *testing.T) {
	w := newWorkspaceCopy(t)
	saved := filepath.Join(w.data, string(identityID), "responses", string(profileID), "1", "last.json")
	if err := os.MkdirAll(filepath.Dir(saved), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(saved, []byte(`{"status": 200, "headers": [{"name": "X-Request-Id", "value": "r"}], "body": {"json": {}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got := w.impact(profileID, "1")

	if got.Status != Compatible || got.Versions[0].Contract.Source != SourceResponse {
		t.Fatalf("got %+v", got)
	}
}

func TestImpact_should_report_cross_collection_consumer(t *testing.T) {
	w := newWorkspaceCopy(t)

	got := w.impact(loginID, "")

	if _, ok := consumerStatus(got, func(c Consumer) bool { return c.Collection == "onboarding" && c.Flow == "signup" }); !ok {
		t.Fatalf("got %+v, want the onboarding/signup consumer", got)
	}
	if len(got.Versions) != 2 || got.Request != "auth/login" || got.Collection != "identity" {
		t.Fatalf("got %+v, want both versions of identity auth/login", got)
	}
}

func TestImpact_Given_unknown_version_Then_ErrUnknownVersion(t *testing.T) {
	w := newWorkspaceCopy(t)

	_, _, err := Impact(w.load(), Target{Collection: identityID, Request: loginID}, "9", w.contracts())

	if !errors.Is(err, ErrUnknownVersion) {
		t.Fatalf("err %v", err)
	}
}

func TestImpact_Given_unknown_request_Then_ErrUnknownRequest(t *testing.T) {
	w := newWorkspaceCopy(t)

	_, _, err := Impact(w.load(), Target{Collection: identityID, Request: "01JB0000000000000000000099"}, "", w.contracts())

	if !errors.Is(err, ErrUnknownRequest) {
		t.Fatalf("err %v", err)
	}
}

func TestWarnings_should_point_at_the_reading_template(t *testing.T) {
	w := newWorkspaceCopy(t)
	w.write("identity/requests/auth/login.examples/v2-200.json", tokenRemoved)
	reports, _ := Analyze(w.load(), w.contracts())

	got := Warnings(reports)

	var after, input *diag.Diagnostic
	for i, d := range got {
		if d.Code == diag.CodeContractBroken && strings.HasSuffix(d.File, "signup.arazzo.yaml") && d.Path == "$.nodes[1].value" {
			after = &got[i]
		}
		if d.Code == diag.CodeContractInput && strings.HasSuffix(d.File, "legacy.arazzo.yaml") {
			input = &got[i]
		}
	}
	if after == nil || input == nil || after.Severity != diag.SeverityWarning {
		t.Fatalf("got %v, want a token warning at signup $.nodes[1].value and a tenant input warning in legacy", got)
	}
	msg, _ := diag.English().Message(*after)
	if want := `node "after" reads nodes.login.token ($.token) from auth/login version 2, which example v2-200.json does not contain`; msg != want {
		t.Fatalf("message %q, want %q", msg, want)
	}
}

func TestAnalyze_should_report_unreadable_examples_once(t *testing.T) {
	w := newWorkspaceCopy(t)
	w.write("identity/requests/auth/login.examples/bad.json", `{`)

	_, diags := Analyze(w.load(), w.contracts())

	count := 0
	for _, d := range diags {
		if d.Code == diag.CodeContractExample {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("got %d FLW-CONTRACT-003, want 1: %v", count, diags)
	}
}
