// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/examples/demoserver"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
	"github.com/githugt/flowi-lib/workspace"
)

// memKeys is an in-memory OS keychain.
type memKeys struct {
	mu   sync.Mutex
	keys map[string]string
}

func (k *memKeys) Get(s, a string) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if v, ok := k.keys[s+"/"+a]; ok {
		return v, nil
	}
	return "", vault.ErrKeyNotFound
}

func (k *memKeys) Set(s, a, v string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.keys == nil {
		k.keys = map[string]string{}
	}
	k.keys[s+"/"+a] = v
	return nil
}

// copyDir copies a directory tree into a temp dir.
func copyDir(t *testing.T, src string) string {
	t.Helper()
	dst := filepath.Join(t.TempDir(), filepath.Base(src))
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dst, rel), 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dst, rel), data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

// demoWorkspace mounts a copy of the example collection whose local
// environment points at a demo server, with the password secret set.
func demoWorkspace(t *testing.T) *Workspace { return demoWorkspaceWith(t, nil) }

// demoWorkspaceWith is demoWorkspace with the options adjusted first.
func demoWorkspaceWith(t *testing.T, adjust func(*WorkspaceOptions)) *Workspace {
	t.Helper()
	srv := httptest.NewServer(demoserver.Handler(demoserver.Options{Password: "demo"}))
	t.Cleanup(srv.Close)
	dir := copyDir(t, filepath.Join("examples", "user-creation"))
	local := filepath.Join(dir, "environments", "local.yaml")
	data, _ := os.ReadFile(local)
	if err := os.WriteFile(local, []byte(strings.Replace(string(data), "http://localhost:8080", srv.URL, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	e := newEnv(t)
	e.opts.Keys = &memKeys{}
	if adjust != nil {
		adjust(&e.opts)
	}
	moveEnvironmentsToWorkspace(t, e, dir)
	w := e.open(t)
	mustMount(t, w, dir)
	if err := w.SetSecret("local", "password", "demo"); err != nil {
		t.Fatal(err)
	}
	return w
}

// moveEnvironmentsToWorkspace turns a collection's environment files into
// the workspace's global ones (environments are global-only now).
func moveEnvironmentsToWorkspace(t *testing.T, e env, collectionDir string) {
	t.Helper()
	from := filepath.Join(collectionDir, "environments")
	to := workspace.EnvironmentsDir(e.opts.Paths, workspace.DefaultName)
	if err := os.MkdirAll(filepath.Dir(to), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

// shareEnvironments gives the workspace of e the environments of w.
func shareEnvironments(t *testing.T, w *Workspace, e env) {
	t.Helper()
	to := workspace.EnvironmentsDir(e.opts.Paths, workspace.DefaultName)
	if err := os.MkdirAll(to, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, env := range w.Environments() {
		data, err := os.ReadFile(env.File)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(to, filepath.Base(env.File)), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func drain(s *engine.Send) engine.SendResult {
	for range s.Events() {
	}
	return s.Wait()
}

func TestFindRequest_should_match_id_path_and_path_with_suffix(t *testing.T) {
	w := demoWorkspace(t)

	for _, ref := range []string{"01JA0000000000000000000010", "auth/login", "auth/login.req.yaml", "requests/auth/login.req.yaml"} {
		got, err := w.FindRequest(ref, "")

		if err != nil || got.Request != "01JA0000000000000000000010" || got.Collection != "01JA0000000000000000000001" {
			t.Errorf("%s: got %+v %v", ref, got, err)
		}
	}
}

func TestFindRequest_Given_unknown_Then_FLW_REQ_001(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.FindRequest("auth/nope", "")

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeRequestNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestFindRequest_Given_same_path_in_two_collections_Then_FLW_REQ_002_until_collection_given(t *testing.T) {
	w := demoWorkspace(t)
	other := copyDir(t, filepath.Join("examples", "user-creation"))
	manifest := filepath.Join(other, "flowi.yaml")
	data, _ := os.ReadFile(manifest)
	data = []byte(strings.Replace(strings.Replace(string(data), "01JA0000000000000000000001", "01JA0000000000000000000009", 1), "name: User creation", "name: Copy", 1))
	_ = os.WriteFile(manifest, data, 0o644)
	mustMount(t, w, other)

	_, ambiguous := w.FindRequest("auth/login", "")
	got, err := w.FindRequest("auth/login", "Copy")

	var le *diag.ListError
	if !errors.As(ambiguous, &le) || le.List[0].Code != diag.CodeRequestAmbiguous || le.List[0].Args["count"] != "2" {
		t.Fatalf("got %v", ambiguous)
	}
	if err != nil || got.Collection != "01JA0000000000000000000009" {
		t.Fatalf("got %+v %v", got, err)
	}
}

func TestSendRequest_should_send_example_login_against_demoserver(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	if res.Status != engine.StatusPassed || res.Record.Status != 200 || res.Record.Outputs["token"] == nil || res.Record.Version != "1" {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	if len(res.Saved) != 1 || !strings.Contains(res.Saved[0], filepath.Join("data", "01JA0000000000000000000001", "responses")) {
		t.Fatalf("saved %v", res.Saved)
	}
}

func TestSendRequest_should_run_example_verify_hook_flow(t *testing.T) {
	w := demoWorkspace(t)
	login, _ := w.FindRequest("auth/login", "")
	verify, _ := w.FindRequest("auth/verify", "")
	s, _ := w.SendRequest(context.Background(), login, SendOptions{Env: "local", PreferVersion: "2", With: map[string]string{"email": "a@example.com"}})
	first := drain(s)

	s, err := w.SendRequest(context.Background(), verify, SendOptions{Env: "local", With: map[string]string{
		"token": first.Record.Outputs["token"].(string), "subject": first.Record.Outputs["subject"].(string), "code": "1",
	}})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	if res.Status != engine.StatusPassed || res.Record.Status != 204 || res.Vars["client_id"] != demoserver.ClientID {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
}

func TestSendRequest_Given_unknown_env_Then_FLW_REQ_004(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")

	_, err := w.SendRequest(context.Background(), target, SendOptions{Env: "prod"})

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeRequestEnvUnknown {
		t.Fatalf("got %v", err)
	}
}

func TestSendRequest_Given_collection_validation_error_Then_blocked(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	file := filepath.Join(c.Root, "requests", "auth", "login.req.yaml")
	data, _ := os.ReadFile(file)
	_ = os.WriteFile(file, []byte(strings.Replace(string(data), `default: "1"`, `default: "7"`, 1)), 0o644)
	if _, err := w.Reload(Change{Root: c.Root, File: file, Op: ChangeModified}); err != nil {
		t.Fatal(err)
	}
	target, _ := w.FindRequest("auth/login", "")

	_, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local"})

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeVersionDefaultAbsent {
		t.Fatalf("got %v", err)
	}
}

func TestPinExample_should_write_example_next_to_request(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	s, _ := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	res := drain(s)

	file, err := w.PinExample(*res.Record, "")

	want := filepath.Join(w.Collections()[0].Root, "requests", "auth", "login.examples", "200.json")
	if err != nil || file != want {
		t.Fatalf("file %s err %v", file, err)
	}
}

func TestClearCookies_should_remove_jar_files(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	s, _ := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	drain(s)
	jar := filepath.Join(w.DataDir(w.Collections()[0].Manifest.ID), "cookies", "jar.json")
	if _, err := os.Stat(jar); err != nil {
		t.Fatalf("no jar after send: %v", err)
	}

	err := w.ClearCookies("")

	if _, statErr := os.Stat(jar); err != nil || !os.IsNotExist(statErr) {
		t.Fatalf("clear %v, jar %v", err, statErr)
	}
}

func TestSendRequest_Given_passphrase_vault_Then_asks_before_the_send_starts(t *testing.T) {
	w := demoWorkspace(t) // keychain vault with the password set
	e := newEnv(t)
	prompts := 0
	e.opts.Passphrase = func() (string, error) { prompts++; return "correct horse", nil }
	shareEnvironments(t, w, e)
	pw := e.open(t)
	mustMount(t, pw, w.Collections()[0].Root)
	if err := pw.SetSecret("local", "password", "demo"); err != nil { // creates a passphrase vault
		t.Fatal(err)
	}
	reopened := e.open(t)
	prompts = 0
	target, _ := reopened.FindRequest("auth/login", "")

	s, err := reopened.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	askedBeforeEvents := prompts
	res := drain(s)

	if err != nil || askedBeforeEvents != 1 || prompts != 1 || res.Status != engine.StatusPassed {
		t.Fatalf("err %v, prompts before send %d, total %d, status %s %v", err, askedBeforeEvents, prompts, res.Status, res.Diags)
	}
}

func TestSendRequest_Given_overrides_for_every_secret_Then_vault_not_unlocked(t *testing.T) {
	w := demoWorkspace(t)
	e := newEnv(t, "FLOWI_SECRET_LOCAL_PASSWORD=demo")
	e.opts.Passphrase = func() (string, error) { t.Error("prompted although every secret is overridden"); return "", nil }
	shareEnvironments(t, w, e)
	pw := e.open(t)
	mustMount(t, pw, w.Collections()[0].Root)
	target, _ := pw.FindRequest("auth/login", "")

	s, err := pw.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	if res := drain(s); res.Status != engine.StatusPassed {
		t.Fatalf("status %s %v", res.Status, res.Diags)
	}
}

func TestPinExample_Given_name_with_path_Then_FLW_REQ_007_and_nothing_written(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	s, _ := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	res := drain(s)

	for _, name := range []string{"../../escape", "a/b", ".hidden", "x\\y", ""} {
		if name == "" {
			continue // "" means "name it after the status"
		}
		_, err := w.PinExample(*res.Record, name)

		var le *diag.ListError
		if !errors.As(err, &le) || le.List[0].Code != diag.CodeRequestExampleName {
			t.Errorf("%q: got %v", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(w.Collections()[0].Root, "requests", "escape.json")); !os.IsNotExist(err) {
		t.Fatal("file written outside the examples directory")
	}
	if err := CheckExampleName("ok-name_1.v2"); err != nil {
		t.Fatalf("valid name refused: %v", err)
	}
}

func TestWorkspace_should_hand_run_events_to_the_Observe_option(t *testing.T) {
	var mu sync.Mutex
	var names []string
	w := demoWorkspaceWith(t, func(o *WorkspaceOptions) {
		o.Observe = func(ev engine.Event) { mu.Lock(); names = append(names, ev.EventName()); mu.Unlock() }
	})
	target, _ := w.FindRequest("auth/login", "")

	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	drain(s)

	mu.Lock()
	defer mu.Unlock()
	if len(names) < 2 || names[0] != "RunStarted" || names[len(names)-1] != "RunFinished" {
		t.Fatalf("observed %v", names)
	}
}

// Before: a restarted app showed every request as "not sent", although the
// last response was saved on disk.
func TestLastResponse_should_return_what_the_last_send_saved(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := w.FindRequest("auth/login", "")
	_, before := w.LastResponse(target, "1")
	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}})
	if err != nil {
		t.Fatal(err)
	}
	drain(s)

	rec, after := w.LastResponse(target, "1")

	if before || !after || rec.Status != 200 || rec.Version != "1" {
		t.Fatalf("before %v after %v record %+v", before, after, rec)
	}
}

func TestSendRequest_should_run_an_async_request_end_to_end(t *testing.T) {
	w := demoWorkspace(t)
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(rw, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = c.CloseNow() }()
		_, data, err := c.Read(r.Context())
		if err == nil {
			_ = c.Write(r.Context(), websocket.MessageText, []byte(`{"echo":`+string(data)+`}`))
		}
		_, _, _ = c.Read(r.Context()) // hold until the client closes
	}))
	t.Cleanup(srv.Close)
	c := w.Collections()[0]
	target, err := w.NewAsyncRequest(c.Manifest.ID, "rpc/echo", "ws")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	r.Versions[0].Async.Server = "ws" + strings.TrimPrefix(srv.URL, "http")
	r.Versions[0].Async.Send = &model.AsyncSend{Body: model.Body{JSON: `{"n":1}`}}
	r.Versions[0].Async.Receive = &model.AsyncReceive{Timeout: 5 * time.Second}
	r.Versions[0].Assert = []string{`body.echo.n == 1`}
	if err := w.SaveRequest(target, r, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	s, err := w.SendRequest(t.Context(), target, SendOptions{Env: "local"})
	if err != nil {
		t.Fatal(err)
	}
	for range s.Events() {
	}
	res := s.Wait()

	if res.Status != engine.StatusPassed || res.Record == nil || res.Record.Kind != engine.KindAsync || len(res.Record.Messages) != 2 {
		t.Fatalf("result %+v diags %v", res, res.Diags)
	}
	if _, _, ok := w.SchemaFor(target, "1", SchemaPart{Kind: "message"}); !ok {
		t.Fatal("the received message was not learned")
	}
	if rec, ok := w.LastResponse(target, "1"); !ok || rec.Kind != engine.KindAsync {
		t.Fatalf("last response %+v ok %v", rec, ok)
	}
}

func TestNewAsyncRequest_should_refuse_an_unknown_protocol(t *testing.T) {
	w := demoWorkspace(t)

	_, err := w.NewAsyncRequest(w.Collections()[0].Manifest.ID, "x/y", "pigeon")

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeAsyncProtocol {
		t.Fatalf("err %v", err)
	}
}

func TestNewAsyncRequest_should_create_a_loadable_request_for_every_protocol(t *testing.T) {
	for i, protocol := range model.AsyncProtocols() {
		w := demoWorkspace(t)

		target, err := w.NewAsyncRequest(w.Collections()[0].Manifest.ID, fmt.Sprintf("new/r%d", i), protocol)

		if err != nil {
			t.Fatalf("%s: %v", protocol, err)
		}
		_, r, rerr := w.requestFor(target)
		if rerr != nil || r.Versions[0].Async == nil || r.Versions[0].Async.Protocol != protocol {
			t.Fatalf("%s: loaded back as %+v", protocol, r.Versions)
		}
	}
}

// Behaviour change: a Service Bus request received in peek-lock mode, which
// removes the messages it keeps. A new one now only peeks (binding read: peek);
// a request without the binding keeps receiving as it did.
func TestNewAsyncRequest_for_Service_Bus_should_only_peek_by_default(t *testing.T) {
	w := demoWorkspace(t)

	target, err := w.NewAsyncRequest(w.Collections()[0].Manifest.ID, "bus/orders", model.ProtocolServiceBus)
	_, r, _ := w.requestFor(target)
	other, _ := w.NewAsyncRequest(w.Collections()[0].Manifest.ID, "mq/orders", model.ProtocolMQTT)
	_, o, _ := w.requestFor(other)

	got := r.Versions[0].Async.Binding
	if err != nil || len(got) != 1 || got[0].Key != "read" || got[0].Value != "peek" {
		t.Fatalf("binding %+v, %v", got, err)
	}
	if len(o.Versions[0].Async.Binding) != 0 {
		t.Fatalf("another protocol got a binding: %+v", o.Versions[0].Async.Binding)
	}
}
