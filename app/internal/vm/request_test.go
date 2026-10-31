// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func newRequestVM(t *testing.T, b *vmtest.Backend) (*vm.Request, *vm.Worker) {
	t.Helper()
	w := vmtest.SyncWorker()
	c := b.Cols[0]
	return vm.NewRequest(b, w, c, c.Requests[0]), w
}

func sampleBackend() *vmtest.Backend {
	return &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
}

func TestRequestVM_should_edit_and_save(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)

	r.Edit(func(v *model.RequestVersion) { v.URL = "{{env.base_url}}/signin" })
	dirty := r.Dirty()
	r.Save(false)
	w.Wait()

	if !dirty || r.Dirty() || len(b.Saved) != 1 || b.Saved[0].Versions[0].URL != "{{env.base_url}}/signin" {
		t.Fatalf("dirty before %v after %v, saved %+v", dirty, r.Dirty(), b.Saved)
	}
}

func TestRequestVM_Given_comments_Then_asks_confirmation_and_keeps_edits(t *testing.T) {
	b := sampleBackend()
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "login.req.yaml"})}.Err()
	r, w := newRequestVM(t, b)
	r.Edit(func(v *model.RequestVersion) { v.Method = "PUT" })

	r.Save(false)
	w.Wait()
	asked, dirty := r.NeedsDropConfirm(), r.Dirty()
	r.Save(true)
	w.Wait()

	if !asked || !dirty || r.NeedsDropConfirm() || r.Dirty() || len(b.Saved) != 1 {
		t.Fatalf("asked %v dirty %v; after confirm asked %v dirty %v saved %d", asked, dirty, r.NeedsDropConfirm(), r.Dirty(), len(b.Saved))
	}
}

func TestRequestVM_new_version_from_current_and_set_default(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	r.Edit(func(v *model.RequestVersion) { v.Headers = []model.KeyValue{{Key: "X-A", Value: "1"}} })

	errBad := r.NewVersionFromCurrent("v2!")
	errDup := r.NewVersionFromCurrent("1")
	errOK := r.NewVersionFromCurrent("2")
	r.SetDefault()

	if errBad == nil || errDup == nil || errOK != nil {
		t.Fatalf("errors %v / %v / %v", errBad, errDup, errOK)
	}
	cur := r.Current()
	if !slices.Equal(r.Labels(), []string{"1", "2"}) || r.SelectedLabel() != "2" || cur.Headers[0].Key != "X-A" || r.Draft().Default != "2" {
		t.Fatalf("labels %v selected %q current %+v default %q", r.Labels(), r.SelectedLabel(), cur, r.Draft().Default)
	}
}

func TestRequestVM_auth_helper_reads_and_writes_the_header(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	r.SetAuth(vm.AuthBearer, "{{vars.token}}")
	kind, value := r.Auth()
	header := r.Current().Headers
	r.SetAuth(vm.AuthNone, "")

	if kind != vm.AuthBearer || value != "{{vars.token}}" || !slices.Contains(header, model.KeyValue{Key: "Authorization", Value: "Bearer {{vars.token}}"}) {
		t.Fatalf("auth %v %q headers %v", kind, value, header)
	}
	if k, _ := r.Auth(); k != vm.AuthNone || len(r.Current().Headers) != 0 {
		t.Fatalf("after none: %v %v", k, r.Current().Headers)
	}
}

func TestRequestVM_settings_invalid_value_reported_and_save_refused(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)

	problems := r.SetSettings([]vm.SettingRow{{Key: "timeout", Raw: "soon"}, {Key: "nope", Raw: "1"}, {Key: "http.max_redirects", Raw: "3"}})
	r.Save(false)
	w.Wait()

	if len(problems) != 2 || problems[0] == "" || len(b.Saved) != 0 || r.Err() == "" {
		t.Fatalf("problems %v saved %d err %q", problems, len(b.Saved), r.Err())
	}
	ok := r.SetSettings([]vm.SettingRow{{Key: "http.max_redirects", Raw: "3"}})
	if len(ok) != 0 || r.Current().Settings["http.max_redirects"] != int64(3) {
		t.Fatalf("problems %v settings %v", ok, r.Current().Settings)
	}
}

func TestRequestVM_Given_external_change_while_dirty_Then_flags_not_overwrites(t *testing.T) {
	b := sampleBackend()
	r, _ := newRequestVM(t, b)
	disk := b.Cols[0].Requests[0]
	disk.Versions = slices.Clone(disk.Versions)
	disk.Versions[0].URL = "changed elsewhere"

	r.Edit(func(v *model.RequestVersion) { v.Method = "PUT" })
	r.Reloaded(b.Cols[0], disk, true)
	flagged, kept := r.ChangedOnDisk(), r.Current().Method
	r.Revert()

	if !flagged || kept != "PUT" || r.Current().URL != "changed elsewhere" || r.Dirty() || r.ChangedOnDisk() {
		t.Fatalf("flagged %v kept %q after revert %+v", flagged, kept, r.Current())
	}
}

func TestRequestVM_Given_external_change_while_clean_Then_takes_it(t *testing.T) {
	b := sampleBackend()
	r, _ := newRequestVM(t, b)
	disk := b.Cols[0].Requests[0]
	disk.Versions = slices.Clone(disk.Versions)
	disk.Versions[0].URL = "changed elsewhere"

	r.Reloaded(b.Cols[0], disk, true)

	if r.ChangedOnDisk() || r.Current().URL != "changed elsewhere" {
		t.Fatalf("current %+v", r.Current())
	}
}

func TestRequestVM_extract_as_variable_appends_saves_and_refuses_duplicate(t *testing.T) {
	b := sampleBackend()
	r, w := newRequestVM(t, b)

	err := r.ExtractAsVariable("token", "$.data.token")
	w.Wait()
	dup := r.ExtractAsVariable("token", "$.other")

	if err != nil || dup == nil || len(b.Saved) != 1 {
		t.Fatalf("err %v dup %v saved %d", err, dup, len(b.Saved))
	}
	if got := b.Saved[0].Versions[0].Extract; !slices.Equal(got, []model.Extraction{{Name: "token", From: "body", Path: "$.data.token"}}) {
		t.Fatalf("extract %+v", got)
	}
}

func TestRequestVM_hook_chain_is_listed(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Manifest.Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "a", Type: "assert"}}}}
	r, _ := newRequestVM(t, b)

	got := r.HookChain()

	if !slices.Equal(got, []string{"post · collection · inline"}) {
		t.Fatalf("chain %v", got)
	}
}

func TestRequestVM_target(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())

	if r.Target() != (flowi.RequestTarget{Collection: "C1", Request: "R1"}) {
		t.Fatalf("target %+v", r.Target())
	}
}

// Before: rows that did not parse (yet) vanished, and list values came
// back as "[5xx 429]", which no longer parsed.
func TestRequestVM_settings_keep_partial_rows_and_round_trip_lists(t *testing.T) {
	b := sampleBackend()
	b.Cols[0].Requests[0].Versions[0].Settings = model.Settings{"retry.on": []string{"5xx", "429"}}
	r, _ := newRequestVM(t, b)

	rows := r.SettingRows()
	again := r.SetSettings(rows)
	partial := r.SetSettings(append(rows, vm.SettingRow{Key: "time"}))

	if !slices.Equal(rows, []vm.SettingRow{{Key: "retry.on", Raw: "5xx, 429"}}) || len(again) != 0 {
		t.Fatalf("rows %+v problems %v", rows, again)
	}
	if len(partial) != 1 || len(r.SettingRows()) != 2 || !slices.Equal(r.Current().Settings["retry.on"].([]string), []string{"5xx", "429"}) {
		t.Fatalf("partial %v rows %+v settings %v", partial, r.SettingRows(), r.Current().Settings)
	}
}

// Before: the screen asked from every refresh while confirm was set, so
// each keystroke after Cancel opened the dialog again; and confirming a
// Send's save never sent.
func TestRequestVM_drop_confirm_asks_once_and_confirm_continues(t *testing.T) {
	b := sampleBackend()
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{}, map[string]string{"file": "x"})}.Err()
	r, w := newRequestVM(t, b)
	asked := 0
	r.OnDropConfirm(func() { asked++ })
	r.Edit(func(v *model.RequestVersion) { v.Method = "PUT" })
	continued := false

	r.SaveThen(false, func() { continued = true })
	w.Wait()
	r.Edit(func(v *model.RequestVersion) { v.URL = "x" })
	r.DeclineDrop()
	declined := r.NeedsDropConfirm()
	r.SaveThen(false, func() { continued = true })
	w.Wait()
	r.ConfirmDrop()
	w.Wait()

	if asked != 2 || declined || !continued || r.Dirty() {
		t.Fatalf("asked %d declined-still-pending %v continued %v dirty %v", asked, declined, continued, r.Dirty())
	}
}

func twoVersionBackend() *vmtest.Backend {
	b := sampleBackend()
	req := &b.Cols[0].Requests[0]
	second := req.Versions[0]
	second.Label = "2"
	req.Versions = append(req.Versions, second)
	req.Default = "1"
	return b
}

func TestRequestVM_DeleteVersion_should_save_the_request_without_the_selected_version(t *testing.T) {
	b := twoVersionBackend()
	r, w := newRequestVM(t, b)
	r.SelectVersion("2")
	got := errSentinel

	r.DeleteVersion(false, func(err error) { got = err })
	w.Wait()

	if got != nil || len(b.Saved) != 1 || len(b.Saved[0].Versions) != 1 || !slices.Equal(r.Labels(), []string{"1"}) || r.SelectedLabel() != "1" || r.Dirty() {
		t.Fatalf("err %v saved %+v labels %v selected %q dirty %v", got, b.Saved, r.Labels(), r.SelectedLabel(), r.Dirty())
	}
}

func TestRequestVM_DeleteVersion_should_refuse_the_default_and_the_only_version(t *testing.T) {
	b := twoVersionBackend()
	r, w := newRequestVM(t, b)
	var onDefault error

	r.DeleteVersion(false, func(err error) { onDefault = err }) // "1" is the default
	w.Wait()
	solo, w2 := newRequestVM(t, sampleBackend())
	var onOnly error
	solo.DeleteVersion(false, func(err error) { onOnly = err })
	w2.Wait()

	if onDefault == nil || onOnly == nil || len(b.Saved) != 0 || len(r.Labels()) != 2 {
		t.Fatalf("default: %v, only: %v, saved %d, labels %v", onDefault, onOnly, len(b.Saved), r.Labels())
	}
}

func TestRequestVM_DeleteVersion_should_leave_the_request_untouched_when_the_save_fails(t *testing.T) {
	b := twoVersionBackend()
	b.SaveErr = errors.New("disk full")
	r, w := newRequestVM(t, b)
	r.SelectVersion("2")
	var got error

	r.DeleteVersion(false, func(err error) { got = err })
	w.Wait()

	if got == nil || !slices.Equal(r.Labels(), []string{"1", "2"}) {
		t.Fatalf("err %v labels %v", got, r.Labels())
	}
}

var errSentinel = errors.New("not called")

func TestRequestVM_DeleteVersion_of_a_version_never_saved_should_leave_nothing_to_save(t *testing.T) {
	r, _ := newRequestVM(t, sampleBackend())
	if err := r.NewVersionFromCurrent("2"); err != nil {
		t.Fatal(err)
	}
	var got error

	r.DeleteVersion(false, func(err error) { got = err })

	// Before: the dot stayed on and closing asked Save / Keep / Discard about an edit that no longer existed.
	if got != nil || r.Dirty() || !slices.Equal(r.Labels(), []string{"1"}) {
		t.Fatalf("err %v dirty %v labels %v", got, r.Dirty(), r.Labels())
	}
}

func TestRequestVM_auth_scheme_should_be_chosen_filled_and_replaced_by_a_simple_kind(t *testing.T) {
	// Arrange
	r, _ := newRequestVM(t, sampleBackend())

	// Act
	r.SetAuthScheme("apikey")
	r.SetAuthParam("name", "X-Key")
	r.SetAuthParam("value", "{{secret.key}}")
	scheme, params := r.AuthScheme()
	r.SetAuthParam("value", "") // an empty value removes the parameter
	_, afterRemove := r.AuthScheme()
	r.SetAuth(vm.AuthBearer, "tok")
	schemeAfterSimple, _ := r.AuthScheme()

	// Assert
	if scheme != "apikey" || len(params) != 2 || params[1].Value != "{{secret.key}}" {
		t.Errorf("scheme %q params %+v", scheme, params)
	}
	if len(afterRemove) != 1 || afterRemove[0].Key != "name" {
		t.Errorf("after removing: %+v", afterRemove)
	}
	if schemeAfterSimple != "" {
		t.Errorf("a simple Authorization header replaces the auth block, got %q", schemeAfterSimple)
	}
}
