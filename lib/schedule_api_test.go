// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func nightly() model.Schedule {
	return model.Schedule{Name: "nightly", Flow: "user-creation", Cron: "0 3 * * *", Env: "local",
		Inputs: []model.KeyValue{{Key: "email", Value: "a@example.com"}}}
}

func workspaceFile(e env) string {
	return filepath.Join(e.opts.Paths.ConfigDir, "workspaces", "default-workspace", "workspace.yaml")
}

func TestAddSchedule_should_save_to_workspace_yaml_and_survive_reopening(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)

	if err := w.AddSchedule(nightly()); err != nil {
		t.Fatal(err)
	}

	got := e.open(t).Schedules()
	if len(got) != 1 || got[0].Name != "nightly" || got[0].Cron != "0 3 * * *" || got[0].Inputs[0].Value != "a@example.com" {
		t.Fatalf("schedules %+v", got)
	}
	data, _ := os.ReadFile(workspaceFile(e))
	if !strings.Contains(string(data), "schedules:") || !strings.Contains(string(data), "name: nightly") {
		t.Fatalf("workspace.yaml:\n%s", data)
	}
}

func TestAddSchedule_Given_a_bad_schedule_Then_refused_and_nothing_saved(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	dup := nightly()
	if err := w.AddSchedule(dup); err != nil {
		t.Fatal(err)
	}
	badCron := nightly()
	badCron.Name, badCron.Cron = "other", "nope"

	errDup := w.AddSchedule(dup)
	errCron := w.AddSchedule(badCron)

	for name, err := range map[string]error{"duplicate": errDup, "bad cron": errCron} {
		var list *diag.ListError
		if !errors.As(err, &list) || list.List[0].Code != diag.CodeServeSchedule {
			t.Errorf("%s: %v", name, err)
		}
	}
	if len(e.open(t).Schedules()) != 1 {
		t.Fatal("a refused schedule was saved")
	}
}

func TestRemoveSchedule_should_remove_by_name_and_refuse_unknown_names(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	if err := w.AddSchedule(nightly()); err != nil {
		t.Fatal(err)
	}

	errUnknown := w.RemoveSchedule("nope")
	errRemove := w.RemoveSchedule("nightly")

	if errUnknown == nil || errRemove != nil || len(e.open(t).Schedules()) != 0 {
		t.Fatalf("unknown: %v, remove: %v", errUnknown, errRemove)
	}
}

func TestValidate_should_report_schedule_problems_at_their_line_in_workspace_yaml(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	mustMount(t, w, copyCollection(t, "identity"))
	data, _ := os.ReadFile(workspaceFile(e))
	extra := "schedules:\n" +
		"  - name: bad-cron\n    flow: whatever\n    cron: not a cron\n" +
		"  - name: lost\n    flow: no-such-flow\n    cron: \"@daily\"\n" +
		"  - name: bad-cron\n    flow: whatever\n    cron: \"@daily\"\n"
	if err := os.WriteFile(workspaceFile(e), append(data, extra...), 0o600); err != nil {
		t.Fatal(err)
	}

	diags := e.open(t).Validate()

	byCode := map[diag.Code]diag.Diagnostic{}
	var serve []diag.Diagnostic
	for _, d := range diags {
		if d.Code == diag.CodeServeSchedule || d.Code == diag.CodeServeScheduleFlow {
			serve = append(serve, d)
			byCode[d.Code] = d
		}
	}
	if len(serve) != 5 {
		t.Fatalf("want 5 schedule diagnostics, got %d: %v", len(serve), codesOf(diags))
	}
	if d := byCode[diag.CodeServeSchedule]; d.File != workspaceFile(e) || d.Pos.Line == 0 {
		t.Errorf("schedule problem not positioned: %+v", d)
	}
	if d := byCode[diag.CodeServeScheduleFlow]; d.Severity != diag.SeverityWarning {
		t.Errorf("an unresolved flow should warn, not fail: %+v", d)
	}
}

func TestResolveSchedule_should_find_the_flow_it_names(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	mustMount(t, w, copyCollection(t, "identity"))
	s := nightly()
	s.Flow = "no-such-flow"

	_, err := w.ResolveSchedule(s)

	var list *diag.ListError
	if !errors.As(err, &list) || list.List[0].Code != diag.CodeFlowNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestSaveSchedule_should_replace_the_named_schedule_in_place(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	first, second := nightly(), nightly()
	second.Name = "hourly"
	for _, s := range []model.Schedule{first, second} {
		if err := w.AddSchedule(s); err != nil {
			t.Fatal(err)
		}
	}
	edited := first
	edited.Name, edited.Cron, edited.Disabled = "nightly-eu", "0 4 * * *", true

	err := e.open(t).SaveSchedule("nightly", edited)

	got := e.open(t).Schedules()
	if err != nil || len(got) != 2 || got[0].Name != "nightly-eu" || got[0].Cron != "0 4 * * *" || !got[0].Disabled || got[1].Name != "hourly" {
		t.Fatalf("schedules %+v err %v", got, err)
	}
}

func TestSaveSchedule_Given_a_bad_edit_Then_refused_and_the_schedule_is_unchanged(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	other := nightly()
	other.Name = "hourly"
	for _, s := range []model.Schedule{nightly(), other} {
		if err := w.AddSchedule(s); err != nil {
			t.Fatal(err)
		}
	}
	badCron, clash := nightly(), nightly()
	badCron.Cron = "nope"
	clash.Name = "hourly" // another schedule's name

	errCron := w.SaveSchedule("nightly", badCron)
	errClash := w.SaveSchedule("nightly", clash)
	errMissing := w.SaveSchedule("nothing", nightly())

	if errCron == nil || errClash == nil || errMissing == nil || w.Schedules()[0].Cron != "0 3 * * *" {
		t.Fatalf("errors %v / %v / %v; schedules %+v", errCron, errClash, errMissing, w.Schedules())
	}
}

func TestSaveSchedule_should_allow_keeping_the_name(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	if err := w.AddSchedule(nightly()); err != nil {
		t.Fatal(err)
	}
	same := nightly()
	same.Cron = "30 3 * * *"

	err := w.SaveSchedule("nightly", same)

	if err != nil || w.Schedules()[0].Cron != "30 3 * * *" {
		t.Fatalf("err %v schedules %+v", err, w.Schedules())
	}
}
