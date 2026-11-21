// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func loadedWorkspace(t *testing.T, b *vmtest.Backend) (*vm.Workspace, *vm.Worker) {
	t.Helper()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	return ws, w
}

func labels(ws *vm.Workspace, ids []string) []string {
	var out []string
	for _, id := range ids {
		out = append(out, ws.Node(id).Label)
	}
	return out
}

func TestWorkspaceVM_should_build_tree_from_collections(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, MissingIDs: []model.ID{"GONE"}, Paths: map[model.ID]string{"GONE": "~/old/api"}}

	ws, _ := loadedWorkspace(t, b)

	roots := ws.Children("")
	if got := labels(ws, roots); !slices.Equal(got, []string{"Shop", "~/old/api (missing)", "Environments"}) {
		t.Fatalf("roots %v", got)
	}
	shop := ws.Children(roots[0])
	if got := labels(ws, shop); !slices.Equal(got, []string{"Requests", "Flows"}) {
		t.Fatalf("collection children %v", got)
	}
	requests := ws.Children(shop[0])
	if got := labels(ws, requests); !slices.Equal(got, []string{"auth", "Health"}) {
		t.Fatalf("requests %v", got)
	}
	auth := ws.Children(requests[0])
	login := ws.Node(auth[0])
	if got := labels(ws, auth); !slices.Equal(got, []string{"Login", "Verify"}) || login.Kind != vm.NodeRequest || login.Request != "R1" || login.Collection != "C1" {
		t.Fatalf("auth %v, login %+v", got, login)
	}
	if ws.Node(roots[1]).Kind != vm.NodeMissing {
		t.Fatalf("missing node %+v", ws.Node(roots[1]))
	}
}

// Before: a request or flow file with merge conflict markers dropped out of the tree as if it had been deleted.
func TestWorkspaceVM_should_keep_a_file_that_cannot_be_read_in_the_tree_marked_with_why(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Broken = []model.BrokenFile{
		{File: "/shop/requests/auth/refresh.req.yaml", Path: "auth/refresh", Kind: "request", Code: "FLW-FILE-003", Message: "unresolved git merge conflict (line 14)"},
		{File: "/shop/flows/checkout.arazzo.yaml", Path: "checkout", Kind: "flow", Code: "FLW-IMPORT-002", Message: "not valid YAML"},
	}
	b := &vmtest.Backend{Cols: []model.Collection{c}}

	ws, _ := loadedWorkspace(t, b)

	var found []vm.TreeNode
	var walk func(ids []string)
	walk = func(ids []string) {
		for _, id := range ids {
			if n := ws.Node(id); n.Kind == vm.NodeBroken {
				found = append(found, n)
			}
			walk(ws.Children(id))
		}
	}
	walk(ws.Children(""))
	if len(found) != 2 {
		t.Fatalf("broken rows: %+v", found)
	}
	req, flow := found[0], found[1]
	if req.Tree != "requests" {
		req, flow = flow, req
	}
	if req.Label != "refresh" || req.Detail != "unresolved git merge conflict (line 14)" || req.Path != "auth/refresh" || req.Tree != "requests" || flow.Label != "checkout" || flow.Tree != "flows" {
		t.Fatalf("request row %+v flow row %+v", req, flow)
	}
}

func TestWorkspaceVM_should_default_env_from_settings_and_list_all(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"defaults.environment": "staging"}}

	ws, _ := loadedWorkspace(t, b)

	if ws.SelectedEnv() != "staging" || !slices.Equal(ws.Environments(), []string{"local", "staging"}) {
		t.Fatalf("selected %q of %v", ws.SelectedEnv(), ws.Environments())
	}
}

func TestWorkspaceVM_should_refresh_and_report_files_on_watched_change(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Changes: make(chan flowi.Change, 1)}
	ws, w := loadedWorkspace(t, b)
	changed := make(chan string, 1)
	ws.OnFileChanged(func(file string) { changed <- file })
	refreshed := make(chan struct{}, 1)
	ws.OnChange(func() { refreshed <- struct{}{} })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Watch(ctx)
	b.Mu.Lock()
	b.Cols[0].Requests = b.Cols[0].Requests[:1]
	b.Mu.Unlock()
	b.Changes <- flowi.Change{Root: "/c", File: "/c/requests/health.req.yaml", Op: flowi.ChangeRemoved}
	file := <-changed
	<-refreshed
	close(b.Changes)
	w.Wait()

	requests := ws.Children(ws.Children(ws.Children("")[0])[0])
	if file != "/c/requests/health.req.yaml" || !b.Called("Reload") || slices.Contains(labels(ws, requests), "Health") {
		t.Fatalf("file %q, requests %v", file, labels(ws, requests))
	}
}

func TestWorkspaceVM_Given_no_default_env_Then_first_environment(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}

	ws, _ := loadedWorkspace(t, b)

	if ws.SelectedEnv() != "local" {
		t.Fatalf("selected %q", ws.SelectedEnv())
	}
}

// Before: Watch ran once with the roots of startup, so collections mounted
// later were never watched.
func TestWorkspaceVM_rewatch_restarts_the_watch(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Changes: make(chan flowi.Change)}
	ws, w := loadedWorkspace(t, b)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Watch(ctx)
	ws.Rewatch()
	cancel()
	w.Wait()

	if got := b.WatchCalls(); got != 2 {
		t.Fatalf("watch started %d times", got)
	}
}

// Before: a watcher that failed to start made watch return at once, so live reload was off and the person was never told.
func TestWorkspaceVM_Given_watch_fails_Then_reports_once_and_retries(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, WatchErr: errors.New("too many open files")}
	ws, w := loadedWorkspace(t, b)
	ws.SetWatchRetry(time.Millisecond)
	problems := make(chan error, 8)
	ws.OnWatchProblem(func(err error) { problems <- err })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ws.Watch(ctx)
	w.Wait()

	if got := b.WatchCalls(); got != vm.WatchRetries+1 {
		t.Fatalf("watch started %d times, want %d", got, vm.WatchRetries+1)
	}
	if len(problems) != 1 {
		t.Fatalf("reported %d times, want once", len(problems))
	}
}

func draftsBackend() *vmtest.Backend {
	scratch := model.Collection{Root: "/d/scratch", Manifest: model.Manifest{Meta: model.Meta{ID: "S1", Name: "Scratch"}},
		Requests: []model.Request{{Meta: model.Meta{ID: "SR1", Name: "untitled"}, Path: "untitled", Versions: []model.RequestVersion{{Label: "1", Method: "POST", URL: "u"}}}}}
	other := model.Collection{Root: "/d/other", Manifest: model.Manifest{Meta: model.Meta{ID: "D2", Name: "Idea"}}}
	return &vmtest.Backend{
		Cols:     []model.Collection{vmtest.SampleCollection(), other, scratch},
		DraftIDs: map[model.ID]bool{"S1": true, "D2": true},
	}
}

func TestWorkspaceVM_should_list_the_scratch_space_first_as_Drafts_and_other_app_collections_as_ordinary_ones(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)

	ws.Load()
	w.Wait()

	roots := ws.Children("")
	if len(roots) != 4 || ws.Node(roots[0]).Label != "Drafts" || ws.Node(roots[1]).Label != "Shop" || ws.Node(roots[2]).Label != "Idea" || ws.Node(roots[3]).Label != "Environments" {
		t.Fatalf("roots %v", roots)
	}
}

func TestWorkspaceVM_should_mark_only_the_scratch_space_as_scratch(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)

	ws.Load()
	w.Wait()

	roots := ws.Children("")
	if !ws.Node(roots[0]).Scratch || ws.Node(roots[2]).Scratch || !ws.Node(roots[2]).Draft {
		t.Fatalf("scratch %v, an unsaved collection's scratch %v and draft %v", ws.Node(roots[0]).Scratch, ws.Node(roots[2]).Scratch, ws.Node(roots[2]).Draft)
	}
}

func TestWorkspaceVM_request_nodes_should_carry_their_method(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)

	ws.Load()
	w.Wait()

	got := ws.Node("r:S1:SR1")
	if got.Method != "POST" {
		t.Fatalf("node %+v", got)
	}
}

func TestWorkspaceVM_Touch_should_remember_when_an_item_was_last_edited(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	at := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)

	before := ws.Touched("r:S1:SR1")
	ws.TouchAt("r:S1:SR1", at)

	if !before.IsZero() || !ws.Touched("r:S1:SR1").Equal(at) {
		t.Fatalf("before %v after %v", before, ws.Touched("r:S1:SR1"))
	}
}

func TestWorkspaceVM_VariableNames_should_list_the_selected_environments_variables_and_secrets(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"defaults.environment": "staging"}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()

	env, secrets := ws.VariableNames()
	ws.SelectEnv("local")
	envLocal, secretsLocal := ws.VariableNames()

	if !slices.Equal(env, []string{"base_url"}) || !slices.Equal(secrets, []string{"password"}) || len(envLocal)+len(secretsLocal) != 0 {
		t.Fatalf("staging %v %v; local %v %v", env, secrets, envLocal, secretsLocal)
	}
}

func TestWorkspaceVM_VariableNames_Given_an_environment_that_builds_on_another_Then_inherited_names_count(t *testing.T) {
	// Before: only the selected environment's own rows counted, so an inherited variable showed as "not set".
	c := vmtest.SampleCollection()
	c.Environments = []model.Environment{
		{Meta: model.Meta{Name: "base"}, File: "/c/environments/base.yaml", Vars: []model.EnvVar{{Name: "base_url", Value: "https://b"}, {Name: "region", Value: "eu"}, {Name: "token", Secret: true}}},
		{Meta: model.Meta{Name: "prod"}, File: "/c/environments/prod.yaml", Extends: "base", Vars: []model.EnvVar{{Name: "base_url", Value: "https://p"}, {Name: "token", Value: "plain-here"}}},
	}
	b := &vmtest.Backend{Cols: []model.Collection{c}, Values: map[string]any{"defaults.environment": "prod"}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()

	vars, secrets := ws.VariableNames()
	chain := ws.SelectedChain()

	slices.Sort(vars)
	if !slices.Equal(vars, []string{"base_url", "region", "token"}) || len(secrets) != 0 {
		t.Fatalf("vars %v secrets %v: region is inherited, and prod's plain token overrides the base secret", vars, secrets)
	}
	if !slices.Equal(chain, []string{"base", "prod"}) {
		t.Fatalf("chain %v", chain)
	}
}

// Review finding 9: renaming a draft while typing must not rebuild the
// workspace (and re-validate it) on every keystroke.
func TestWorkspaceVM_SetLabel_should_redraw_the_tree_without_a_workspace_change(t *testing.T) {
	b := draftsBackend()
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()
	changes, labels := 0, 0
	ws.OnChange(func() { changes++ })
	ws.OnLabelChange(func() { labels++ })

	ws.SetLabel("r:S1:SR1", "a")
	ws.SetLabel("r:S1:SR1", "ab")

	if changes != 0 || labels != 2 {
		t.Fatalf("workspace changes %d label changes %d", changes, labels)
	}
}

// Before: choosing "no environment" was undone by the next reload (a send
// reloads the workspace), which put the default environment back, so the
// following send failed with FLW-REQ-004 in a collection without it.
func TestWorkspaceVM_Given_no_environment_chosen_Then_a_reload_keeps_it(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"defaults.environment": "staging"}}
	ws, w := loadedWorkspace(t, b)
	ws.SelectEnv("")

	ws.Load()
	w.Wait()

	if ws.SelectedEnv() != "" {
		t.Fatalf("selected %q after a reload, want none", ws.SelectedEnv())
	}
}

func TestWorkspaceVM_Given_a_chosen_environment_that_vanished_Then_the_default_returns(t *testing.T) {
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Values: map[string]any{"defaults.environment": "staging"}}
	ws, w := loadedWorkspace(t, b)
	ws.SelectEnv("gone")

	ws.Load()
	w.Wait()

	if ws.SelectedEnv() != "staging" {
		t.Fatalf("selected %q, want the default", ws.SelectedEnv())
	}
}

// A folder's order is kept in its _folder.yaml; entries it does not name follow, by name.
func TestWorkspace_tree_should_follow_the_order_of_a_folder(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Requests = []model.Request{
		{Meta: model.Meta{ID: "A", Name: "Alpha"}, Path: "alpha", Versions: []model.RequestVersion{{Label: "1", Method: "GET"}}},
		{Meta: model.Meta{ID: "B", Name: "Beta"}, Path: "beta", Versions: []model.RequestVersion{{Label: "1", Method: "GET"}}},
		{Meta: model.Meta{ID: "C", Name: "Gamma"}, Path: "gamma", Versions: []model.RequestVersion{{Label: "1", Method: "GET"}}},
	}
	c.Folders = []model.Folder{{Dir: "requests", Order: []string{"gamma", "alpha"}}}
	ws, _ := loadedWorkspace(t, &vmtest.Backend{Cols: []model.Collection{c}})

	var labels []string
	for _, id := range ws.Children("c:" + string(c.Manifest.ID) + "/requests") {
		labels = append(labels, ws.Node(id).Label)
	}

	if strings.Join(labels, ",") != "Gamma,Alpha,Beta" {
		t.Fatalf("labels %v", labels)
	}
}

func TestWorkspaceVM_ProtectedEnvs_should_list_the_environments_that_ask_first(t *testing.T) {
	c := vmtest.SampleCollection()
	c.Environments = []model.Environment{
		{Meta: model.Meta{Name: "staging"}, File: "/c/environments/staging.yaml"},
		{Meta: model.Meta{Name: "prod"}, File: "/c/environments/prod.yaml", Protected: true},
		{Meta: model.Meta{Name: "live"}, File: "/c/environments/live.yaml", Protected: true},
	}
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	w := vmtest.SyncWorker()
	ws := vm.NewWorkspace(b, w)
	ws.Load()
	w.Wait()

	if got := ws.ProtectedEnvs(); !slices.Equal(got, []string{"live", "prod"}) {
		t.Fatalf("protected %v, want live and prod", got)
	}
}
