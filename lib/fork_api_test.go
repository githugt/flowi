// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/report"
)

func openRecorded(t *testing.T) *BundleRun {
	t.Helper()
	data, _ := recordedBundle(t)
	br, err := newEnv(t).open(t).OpenBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(br.Close)
	return br
}

func TestFork_should_run_from_a_step_with_the_recorded_values_before_it_and_an_edited_one_after(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Set: map[string]any{"first_name": "Zed"}})

	if err != nil {
		t.Fatal(err)
	}
	ran := []string{}
	for _, n := range got.Run.Report.Nodes {
		ran = append(ran, n.ID)
	}
	if !slices.Equal(ran, []string{"page-1", "page-2"}) {
		t.Fatalf("ran %v, want only the steps from page-1", ran)
	}
	if got.Run.Status != engine.StatusPassed || got.Run.Report.Vars["first_name"] != "Zed" || got.Run.Report.Vars["token"] == nil {
		t.Fatalf("status %s vars %v", got.Run.Status, got.Run.Report.Vars)
	}
}

func TestFork_should_compare_only_the_steps_that_ran_again(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Set: map[string]any{"first_name": "Zed"}})

	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range got.Comparison.Changes {
		if ch.Node != "page-1" && ch.Node != "page-2" {
			t.Fatalf("a step that did not run again is in the comparison: %+v", ch)
		}
	}
}

func TestFork_Given_an_edit_that_changes_a_called_path_Then_the_run_says_the_recording_has_no_answer(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Set: map[string]any{"subject": "u-changed"}})

	if err != nil {
		t.Fatal(err)
	}
	if got.Run.Status == engine.StatusPassed || !strings.Contains(diagText(got.Run), "recording") {
		t.Fatalf("status %s\n%s", got.Run.Status, diagText(got.Run))
	}
}

func TestFork_Given_a_step_that_is_not_in_the_flow_Then_the_error_lists_the_ones_that_are(t *testing.T) {
	br := openRecorded(t)

	_, err := br.Fork(t.Context(), ForkOptions{From: "nope"})

	if err == nil || !strings.Contains(err.Error(), "page-1") {
		t.Fatalf("err %v", err)
	}
}

func TestForkReplay_should_fork_a_flow_of_a_collection_opened_from_a_bundle(t *testing.T) {
	data, _ := recordedBundle(t)
	other := newEnv(t).open(t)
	br, err := other.KeepBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(br.Close)

	got, err := other.ForkReplay(t.Context(), br.Target, ForkOptions{From: "page-1"})

	if err != nil || got.Run.Status != engine.StatusPassed || len(got.Run.Report.Nodes) != 2 {
		t.Fatalf("fork %+v err %v", got.Run.Status, err)
	}
}

func TestForkReplay_Given_an_ordinary_collection_Then_there_is_nothing_to_fork(t *testing.T) {
	w := demoWorkspace(t)
	target, _ := userCreation(t, w)

	_, err := w.ForkReplay(t.Context(), target, ForkOptions{From: "login"})

	if err == nil || !strings.Contains(err.Error(), "bundle") {
		t.Fatalf("err %v", err)
	}
}

func TestForkVariables_should_give_what_the_recording_held_before_the_step(t *testing.T) {
	data, _ := recordedBundle(t)
	other := newEnv(t).open(t)
	br, err := other.KeepBundle(data)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(br.Close)

	before, err := other.ForkVariables(br.Target, "profile")
	atStart, _ := other.ForkVariables(br.Target, "login")

	if err != nil || before["subject"] == nil || before["first_name"] != nil || len(atStart) != 0 {
		t.Fatalf("before profile %v, before login %v, err %v", before, atStart, err)
	}
}

func capturedFor(res engine.RunResult, requestPath string) (engine.Record, bool) {
	i := slices.IndexFunc(res.Captured, func(r engine.Record) bool { return r.RequestPath == requestPath })
	if i < 0 {
		return engine.Record{}, false
	}
	return res.Captured[i], true
}

func TestFork_Given_only_Then_just_that_step_runs_again(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true})

	if err != nil {
		t.Fatal(err)
	}
	ran := []string{}
	for _, n := range got.Run.Report.Nodes {
		ran = append(ran, n.ID)
	}
	if !slices.Equal(ran, []string{"page-1"}) {
		t.Fatalf("ran %v, want page-1 alone", ran)
	}
	for _, ch := range got.Comparison.Changes {
		if ch.Node != "page-1" {
			t.Fatalf("a step that did not run again is in the comparison: %+v", ch)
		}
	}
}

func TestFork_Given_a_response_for_a_step_Then_that_call_is_answered_with_it_instead_of_the_recording(t *testing.T) {
	br := openRecorded(t)
	respond := map[string]ForkResponse{"page-1": {Status: 418, Body: map[string]any{"x": 1.0}, HasBody: true}}

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Respond: respond})

	if err != nil {
		t.Fatal(err)
	}
	rec, ok := capturedFor(got.Run, "onboarding/form-page-1")
	body, _ := rec.Body.JSON.(map[string]any)
	if !ok || rec.Status != 418 || fmt.Sprint(body["x"]) != "1" {
		t.Fatalf("page-1 was answered with status %d body %+v (found %v)", rec.Status, rec.Body, ok)
	}
}

func TestFork_Given_only_a_status_for_a_step_Then_the_recorded_body_stays(t *testing.T) {
	br := openRecorded(t)
	recorded, _ := capturedFor(br.Original, "onboarding/form-page-1")

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Respond: map[string]ForkResponse{"page-1": {Status: 503}}})

	if err != nil {
		t.Fatal(err)
	}
	rec, _ := capturedFor(got.Run, "onboarding/form-page-1")
	// Compared as JSON: the text of a replayed body is re-encoded, so it may differ in whitespace.
	if rec.Status != 503 || !reflect.DeepEqual(rec.Body.JSON, recorded.Body.JSON) {
		t.Fatalf("status %d, body %+v; recorded body %+v", rec.Status, rec.Body, recorded.Body)
	}
}

func TestFork_Given_a_response_for_a_step_that_makes_no_request_Then_the_error_says_so(t *testing.T) {
	br := openRecorded(t)

	_, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Respond: map[string]ForkResponse{"profile": {Status: 500}}})

	if err == nil || !strings.Contains(err.Error(), "profile") {
		t.Fatalf("err %v", err)
	}
}

func TestFork_Given_a_response_for_a_step_before_the_fork_Then_the_error_says_it_does_not_run(t *testing.T) {
	br := openRecorded(t)

	_, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Respond: map[string]ForkResponse{"login": {Status: 500}}})

	if err == nil || !strings.Contains(err.Error(), "login") || !strings.Contains(err.Error(), "does not run") {
		t.Fatalf("err %v", err)
	}
}

func TestFork_Given_only_a_response_for_a_later_step_Then_the_error_says_it_does_not_run(t *testing.T) {
	br := openRecorded(t)

	_, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true, Respond: map[string]ForkResponse{"page-2": {Status: 500}}})

	if err == nil || !strings.Contains(err.Error(), "page-2") || !strings.Contains(err.Error(), "does not run") {
		t.Fatalf("err %v", err)
	}
}

func hasChange(c report.Comparison, what string) (report.Change, bool) {
	i := slices.IndexFunc(c.Changes, func(ch report.Change) bool { return ch.What == what })
	if i < 0 {
		return report.Change{}, false
	}
	return c.Changes[i], true
}

func TestFork_Given_a_status_for_a_step_Then_the_comparison_says_the_call_was_answered_differently(t *testing.T) {
	br := openRecorded(t)
	recorded, _ := capturedFor(br.Original, "onboarding/form-page-1")

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true, Respond: map[string]ForkResponse{"page-1": {Status: 503}}})

	if err != nil {
		t.Fatal(err)
	}
	ch, ok := hasChange(got.Comparison, "call status")
	if !ok || ch.Node != "onboarding/form-page-1" || ch.From != strconv.Itoa(recorded.Status) || ch.To != "503" {
		t.Fatalf("comparison %+v: want the call's status change", got.Comparison)
	}
	if got.Comparison.FirstDivergence == "" {
		t.Fatalf("a change with no first divergence: %+v", got.Comparison)
	}
}

func TestFork_Given_a_body_for_a_step_Then_the_comparison_says_the_body_changed(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true, Respond: map[string]ForkResponse{"page-1": {Body: map[string]any{"x": 1.0}, HasBody: true}}})

	if err != nil {
		t.Fatal(err)
	}
	if _, ok := hasChange(got.Comparison, "call body"); !ok {
		t.Fatalf("comparison %+v: want the call's body change", got.Comparison)
	}
	if _, ok := hasChange(got.Comparison, "call status"); ok {
		t.Fatalf("the status was not changed: %+v", got.Comparison)
	}
}

func TestFork_Given_no_response_given_Then_no_call_is_reported_as_answered_differently(t *testing.T) {
	br := openRecorded(t)

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1"})

	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range got.Comparison.Changes {
		if strings.HasPrefix(ch.What, "call ") {
			t.Fatalf("an unchanged call is reported: %+v", ch)
		}
	}
}

func TestBundleRunForkVariables_should_give_what_the_recording_held_before_the_step(t *testing.T) {
	br := openRecorded(t)

	before, err := br.ForkVariables("profile")
	_, unknown := br.ForkVariables("nope")

	if err != nil || before["subject"] == nil || before["first_name"] != nil {
		t.Fatalf("before profile %v, err %v", before, err)
	}
	if unknown == nil || !strings.Contains(unknown.Error(), "page-1") {
		t.Fatalf("an unknown step: %v", unknown)
	}
}

func TestFork_Given_a_text_body_for_a_step_Then_the_call_is_answered_with_text_that_is_not_json(t *testing.T) {
	br := openRecorded(t)
	respond := map[string]ForkResponse{"page-1": {Text: "<error>down</error>", HasText: true, ContentType: "application/xml"}}

	got, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true, Respond: respond})

	if err != nil {
		t.Fatal(err)
	}
	rec, ok := capturedFor(got.Run, "onboarding/form-page-1")
	if !ok || rec.Body.Text != "<error>down</error>" || rec.Body.JSON != nil || rec.Body.ContentType != "application/xml" || rec.Body.Size != int64(len("<error>down</error>")) {
		t.Fatalf("page-1 was answered with %+v (found %v)", rec.Body, ok)
	}
	if _, ok := hasChange(got.Comparison, "call body"); !ok {
		t.Fatalf("comparison %+v: want the call's body change", got.Comparison)
	}
}

func TestFork_Given_a_json_and_a_text_body_for_one_step_Then_it_is_refused(t *testing.T) {
	br := openRecorded(t)
	respond := map[string]ForkResponse{"page-1": {Body: 1.0, HasBody: true, Text: "x", HasText: true}}

	_, err := br.Fork(t.Context(), ForkOptions{From: "page-1", Only: true, Respond: respond})

	if err == nil || !strings.Contains(err.Error(), "both") {
		t.Fatalf("err %v", err)
	}
}
