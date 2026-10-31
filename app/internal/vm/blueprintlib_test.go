// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"testing"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func libFixture(t *testing.T, seed string) (*vm.Workspace, *vmtest.Backend, *vm.Worker) {
	t.Helper()
	b := sampleBackend()
	b.Cols[0].Blueprints = hookBlueprints()
	if seed != "" {
		b.Values = map[string]any{"requests.seed_blueprint": seed}
	}
	ws, w := loadedWorkspace(t, b)
	return ws, b, w
}

func blueprintNamed(ws *vm.Workspace, path string) vm.BlueprintChoice {
	for _, bp := range ws.Blueprints() {
		if bp.Path == path {
			return bp
		}
	}
	return vm.BlueprintChoice{}
}

func TestBlueprints_Kind_should_tell_hooks_from_flow_blueprints(t *testing.T) {
	ws, _, _ := libFixture(t, "")

	checks, stamp, needs := blueprintNamed(ws, "checks"), blueprintNamed(ws, "stamp"), blueprintNamed(ws, "needs")

	if checks.Kind() != vm.BlueprintPostHooks || stamp.Kind() != vm.BlueprintPreHooks || needs.Kind() != vm.BlueprintFlow {
		t.Fatalf("kinds %v %v %v", checks.Kind(), stamp.Kind(), needs.Kind())
	}
}

func TestWorkspace_DeleteBlueprint_should_delete_it(t *testing.T) {
	ws, b, w := libFixture(t, "")
	var done bool

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), func() { done = true }, func(err error) { t.Fatal(err) })
	w.Wait()

	if !done || !b.Called("DeleteBlueprint BP-POST") {
		t.Fatalf("done %v calls %v", done, b.Calls)
	}
}

func TestWorkspace_DeleteBlueprint_Given_it_seeds_new_requests_Then_takes_it_out_of_the_setting(t *testing.T) {
	ws, b, w := libFixture(t, "checks, stamp")

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), nil, func(err error) { t.Fatal(err) })
	w.Wait()

	// Left in the list it would make every new request fail to seed.
	if !b.Called("SetUserSetting requests.seed_blueprint=stamp") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestWorkspace_DeleteBlueprint_Given_it_was_the_only_seed_Then_goes_back_to_the_default(t *testing.T) {
	ws, b, w := libFixture(t, "checks")

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), nil, func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("SetUserSetting requests.seed_blueprint=default") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestWorkspace_DeleteBlueprint_Given_it_is_not_in_the_setting_Then_leaves_the_setting_alone(t *testing.T) {
	ws, b, w := libFixture(t, "stamp")

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), nil, func(err error) { t.Fatal(err) })
	w.Wait()

	for _, c := range b.Calls {
		if len(c) > 14 && c[:14] == "SetUserSetting" {
			t.Fatalf("setting changed: %v", b.Calls)
		}
	}
}

func TestWorkspace_UseBlueprintAsDefault_should_replace_the_seed_for_its_phase(t *testing.T) {
	ws, b, w := libFixture(t, "")
	var note string
	var done bool

	ws.UseBlueprintAsDefault(blueprintNamed(ws, "stamp"), func(n string) { done, note = true, n }, func(err error) { t.Fatal(err) })
	w.Wait()

	if !done || note != "" || !b.Called("SetUserSetting requests.seed_blueprint=default, stamp") {
		t.Fatalf("done %v note %q calls %v", done, note, b.Calls)
	}
}

func TestWorkspace_UseBlueprintAsDefault_Given_a_flow_blueprint_Then_refuses(t *testing.T) {
	ws, b, w := libFixture(t, "")
	var failed error

	ws.UseBlueprintAsDefault(blueprintNamed(ws, "needs"), nil, func(err error) { failed = err })
	w.Wait()

	if failed == nil || b.Called("SetUserSetting requests.seed_blueprint=needs") {
		t.Fatalf("err %v calls %v", failed, b.Calls)
	}
}

func TestWorkspace_DeleteBlueprint_Given_another_collection_has_the_same_path_Then_keeps_the_seed_entry(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Blueprints = hookBlueprints()
	second := b.Cols[0]
	second.Manifest.ID, second.Manifest.Name = "C2", "Other"
	second.Blueprints = hookBlueprints()[:1] // also has "checks"
	for i := range second.Blueprints {
		second.Blueprints[i].ID = "BP-OTHER"
	}
	b.Cols = append(b.Cols, second)
	b.Values = map[string]any{"requests.seed_blueprint": "checks"}
	ws, w := loadedWorkspace(t, b)

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), nil, func(err error) { t.Fatal(err) })
	w.Wait()

	for _, c := range b.Calls {
		if len(c) > 14 && c[:14] == "SetUserSetting" {
			t.Fatalf("the other collection's checks no longer seeds: %v", b.Calls)
		}
	}
}

func TestWorkspace_DeleteBlueprint_should_not_rewrite_a_setting_it_did_not_change(t *testing.T) {
	ws, b, w := libFixture(t, "stamp,default")

	ws.DeleteBlueprint(blueprintNamed(ws, "checks"), nil, func(err error) { t.Fatal(err) })
	w.Wait()

	for _, c := range b.Calls {
		if len(c) > 14 && c[:14] == "SetUserSetting" {
			t.Fatalf("setting rewritten: %v", b.Calls)
		}
	}
}
