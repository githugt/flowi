// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
)

func overlayLoginTarget(t *testing.T, w *Workspace) (RequestTarget, model.RequestVersion) {
	t.Helper()
	target, err := w.FindRequest("auth/login", "")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)
	return target, r.Versions[0]
}

func TestSaveAsOverlay_should_store_the_edit_split_into_shared_and_own_parts(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	edited := base
	edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-Tier", Value: "vip"})
	edited.Assert = append(append([]string{}, base.Assert...), "status == 200")
	diff, err := w.DiffForOverlay(target, base.Label, edited)
	if err != nil || len(diff.Ops) != 2 {
		t.Fatalf("diff %+v err %v", diff, err)
	}

	// Act
	err = w.SaveAsOverlay(target, SaveOverlayInput{Name: "vip", Base: base.Label, Edited: edited, Shared: []int{0}}, SaveOptions{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	ov, err := w.Overlay(target.Collection, "vip")
	if err != nil {
		t.Fatal(err)
	}
	if len(ov.Shared) != 1 || ov.Shared[0].Path != "headers.X-Tier" {
		t.Errorf("shared = %+v", ov.Shared)
	}
	if len(ov.Requests) != 1 || ov.Requests[0].Path != "auth/login" || len(ov.Requests[0].Ops) != 1 {
		t.Errorf("requests = %+v", ov.Requests)
	}
	if !strings.HasSuffix(ov.File, filepath.Join("overlays", "vip.flowi-overlay.yaml")) {
		t.Errorf("file = %s", ov.File)
	}
}

func TestSendRequest_should_send_with_a_flowi_overlay_and_keep_its_response_apart(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	edited := base
	edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-Tier", Value: "vip"})
	if err := w.SaveAsOverlay(target, SaveOverlayInput{Name: "vip", Base: base.Label, Edited: edited, Shared: []int{0}}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	opts := SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}, Save: "last"}

	// Act
	opts.Overlay = "vip"
	s, err := w.SendRequest(context.Background(), target, opts)
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	// Assert
	if res.Status != engine.StatusPassed || res.Record.Overlay != "vip" {
		t.Fatalf("result %+v diags %v", res.Status, res.Diags)
	}
	if _, ok := w.LastResponse(target, engine.ResponseKey(base.Label, "vip")); !ok {
		t.Error("the overlay's response was not kept under its own key")
	}
	if _, ok := w.LastResponse(target, base.Label); ok {
		t.Error("the base version's response must not be touched by an overlay send")
	}
}

func TestSendRequest_Given_unknown_overlay_Then_FLW_OVL_002_and_nothing_is_sent(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)

	// Act
	_, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", Overlay: "ghost"})

	// Assert
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeOverlayNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestSaveAsOverlay_should_refuse_a_name_that_is_not_a_file_name(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)

	// Act
	err := w.SaveAsOverlay(target, SaveOverlayInput{Name: "../x", Base: base.Label, Edited: base}, SaveOptions{})

	// Assert
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeOverlayName {
		t.Fatalf("got %v", err)
	}
}

func TestSaveAsOverlay_should_keep_other_requests_parts_when_the_overlay_exists(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	existing := model.Overlay{Name: "slow", Requests: []model.RequestOverlay{{Request: "other", Path: "other/req", Ops: []model.OverlayOp{{Verb: model.OverlaySet, Path: "url", Value: "x"}}}}}
	if err := w.WriteOverlay(target.Collection, existing, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	edited := base
	edited.URL = base.URL + "?slow=1"

	// Act
	err := w.SaveAsOverlay(target, SaveOverlayInput{Name: "slow", Base: base.Label, Edited: edited}, SaveOptions{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	ov, _ := w.Overlay(target.Collection, "slow")
	if len(ov.Requests) != 2 {
		t.Errorf("requests = %+v", ov.Requests)
	}
}

func TestWriteOverlay_should_refuse_an_operation_no_version_could_take(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)
	bad := model.Overlay{Name: "bad", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "colour", Value: "red"}}}

	// Act
	err := w.WriteOverlay(target.Collection, bad, SaveOptions{})

	// Assert
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeOverlayInvalid {
		t.Fatalf("got %v", err)
	}
}

func TestDeleteOverlay_should_remove_the_file(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)
	if err := w.WriteOverlay(target.Collection, model.Overlay{Name: "tmp", Shared: []model.OverlayOp{{Verb: model.OverlaySet, Path: "url", Value: "x"}}}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	ov, _ := w.Overlay(target.Collection, "tmp")

	// Act
	err := w.DeleteOverlay(target.Collection, "tmp")

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	if _, statErr := os.Stat(ov.File); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("file still there: %v", statErr)
	}
	if _, err := w.Overlay(target.Collection, "tmp"); err == nil {
		t.Error("overlay still listed")
	}
}

func TestSaveAsOverlay_should_not_repeat_shared_changes_in_the_request_part(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	tier := model.OverlayOp{Verb: model.OverlaySet, Path: "headers.X-Tier", Value: "vip"}
	if err := w.WriteOverlay(target.Collection, model.Overlay{Name: "vip", Shared: []model.OverlayOp{tier}}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	edited := base
	edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-Tier", Value: "vip"})
	edited.URL = base.URL + "?vip=1"

	// Act
	err := w.SaveAsOverlay(target, SaveOverlayInput{Name: "vip", Base: base.Label, Edited: edited}, SaveOptions{})

	// Assert
	if err != nil {
		t.Fatal(err)
	}
	ov, _ := w.Overlay(target.Collection, "vip")
	if len(ov.Requests) != 1 || len(ov.Requests[0].Ops) != 1 || ov.Requests[0].Ops[0].Path != "url" {
		t.Errorf("the request part should hold only the url change: %+v", ov.Requests)
	}
}

func TestWriteOverlayText_should_keep_the_text_as_given_and_refuse_bad_text(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)
	text := "# slow customer\nflowi_overlay: 1\nshared:\n  - set: headers.X-Slow\n    value: \"1\"\n"

	// Act
	err := w.WriteOverlayText(target.Collection, "slow", text)
	got, readErr := w.OverlayText(target.Collection, "slow")
	badErr := w.WriteOverlayText(target.Collection, "bad", "flowi_overlay: 1\nshared:\n  - set: colour\n    value: red\n")

	// Assert
	if err != nil || readErr != nil || got != text {
		t.Fatalf("write %v read %v text %q", err, readErr, got)
	}
	var le *diag.ListError
	if !errors.As(badErr, &le) || le.List[0].Code != diag.CodeOverlayInvalid {
		t.Errorf("bad text: %v", badErr)
	}
}

func TestSendRequest_should_name_the_overlay_an_edit_was_made_with(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	edited := base
	edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-Tier", Value: "vip"})
	if err := w.StoreEditUnder(target, base.Label, "vip", edited, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	// Act
	s, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}, UseEdit: true, Save: "last"})
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	// Assert
	if res.Record == nil || res.Record.Overlay != "vip" {
		t.Fatalf("record %+v diags %v", res.Record, res.Diags)
	}
	if _, ok := w.LastResponse(target, engine.ResponseKey(base.Label, "vip")); !ok {
		t.Error("the response should be kept under the overlay")
	}
}

func TestSendRequest_should_lay_stacked_overlays_over_the_request_and_name_the_stack(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, base := overlayLoginTarget(t, w)
	for _, name := range []string{"vip", "audited"} {
		edited := base
		edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-" + name, Value: "1"})
		if err := w.SaveAsOverlay(target, SaveOverlayInput{Name: name, Base: base.Label, Edited: edited, Shared: []int{0}}, SaveOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	opts := SendOptions{Env: "local", With: map[string]string{"email": "a@example.com"}, Save: "last", Overlay: "vip+audited"}

	// Act
	s, err := w.SendRequest(context.Background(), target, opts)
	if err != nil {
		t.Fatal(err)
	}
	res := drain(s)

	// Assert
	sent := res.Record.Request.Header
	if res.Record.Overlay != "vip+audited" || !hasHeader(sent, "X-vip") || !hasHeader(sent, "X-audited") {
		t.Fatalf("overlay %q, sent headers %v", res.Record.Overlay, sent)
	}
}

func TestSendRequest_Given_a_stack_with_an_unknown_overlay_Then_names_the_unknown_one(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := overlayLoginTarget(t, w)

	// Act
	_, err := w.SendRequest(context.Background(), target, SendOptions{Env: "local", Overlay: "ghost+phantom"})

	// Assert
	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Args["name"] != "ghost" {
		t.Fatalf("got %v", err)
	}
}

func hasHeader(headers []engine.Header, name string) bool {
	for _, h := range headers {
		if strings.EqualFold(h.Name, name) {
			return true
		}
	}
	return false
}

func TestRunFlow_Given_an_unknown_overlay_Then_FLW_OVL_002_and_nothing_runs(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	// Act
	_, err := w.RunFlow(context.Background(), target, RunOptions{Env: "local", Overlay: "ghost"})

	// Assert
	var list *diag.ListError
	if !errors.As(err, &list) || list.List[0].Code != diag.CodeOverlayNotFound {
		t.Fatalf("err %v", err)
	}
}

func TestRunFlow_Given_a_known_overlay_Then_the_result_says_which_one_it_ran_with(t *testing.T) {
	// Arrange
	w := demoWorkspace(t)
	reqTarget, base := overlayLoginTarget(t, w)
	edited := base
	edited.Headers = append(append([]model.KeyValue{}, base.Headers...), model.KeyValue{Key: "X-Tier", Value: "vip"})
	if err := w.SaveAsOverlay(reqTarget, SaveOverlayInput{Name: "vip", Base: base.Label, Edited: edited, Shared: []int{0}}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	target, _ := userCreation(t, w)

	// Act
	res := runToEnd(t, w, target, RunOptions{Env: "local", Overlay: "vip", Inputs: map[string]string{"email": "a@example.com"}})

	// Assert
	if res.Report.Overlay != "vip" {
		t.Fatalf("report overlay %q, status %s %v", res.Report.Overlay, res.Status, res.Diags)
	}
}
