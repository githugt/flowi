// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func groupedEnv(name, group, extends string) model.Environment {
	e := model.Environment{Meta: model.Meta{Name: name}, File: "/e/" + name + ".yaml", Extends: extends}
	if group != "" {
		e.Groups = []string{group}
	}
	return e
}

func groupedBackend(envs ...model.Environment) *vmtest.Backend {
	return &vmtest.Backend{Cols: []model.Collection{{Environments: envs}}}
}

// Before: environments had no group, so the screen that lists them had nothing to show or move between.
func TestManageEnvironments_Given_grouped_environments_Then_the_groups_are_counted_and_the_default_comes_first(t *testing.T) {
	envs := []model.Environment{groupedEnv("b", "payments", ""), groupedEnv("a", "", ""), groupedEnv("c", "checkout", ""), groupedEnv("d", "payments", "")}

	m := vm.ManageEnvironments(envs, nil)

	var got []string
	for _, g := range m.Groups {
		got = append(got, g.Label+":"+string(rune('0'+g.Count)))
	}
	if !slices.Equal(got, []string{"Default:1", "checkout:1", "payments:2"}) {
		t.Fatalf("groups %v", got)
	}
	if names := []string{m.Envs[0].Name, m.Envs[1].Name, m.Envs[2].Name, m.Envs[3].Name}; !slices.Equal(names, []string{"a", "b", "c", "d"}) {
		t.Fatalf("order %v", names)
	}
}

func TestManageEnvironments_Given_no_environments_Then_the_default_group_is_still_listed(t *testing.T) {
	m := vm.ManageEnvironments(nil, nil)

	if len(m.Groups) != 1 || m.Groups[0].Name != "" || m.Groups[0].Count != 0 {
		t.Fatalf("groups %+v", m.Groups)
	}
}

func TestManageEnvironments_Given_a_base_in_another_group_or_missing_Then_the_environment_carries_a_warning(t *testing.T) {
	envs := []model.Environment{groupedEnv("base", "", ""), groupedEnv("prod", "checkout", "base"), groupedEnv("same", "", "base"), groupedEnv("lost", "", "gone")}

	m := vm.ManageEnvironments(envs, nil)

	warn := map[string]string{}
	for _, e := range m.Envs {
		warn[e.Name] = e.Warning
	}
	if !strings.Contains(warn["prod"], "base") || !strings.Contains(warn["prod"], "default group") || warn["same"] != "" || !strings.Contains(warn["lost"], "does not exist") || warn["base"] != "" {
		t.Fatalf("warnings %+v", warn)
	}
}

func TestMoveEnvironments_Given_a_good_name_Then_the_environments_move_and_a_bad_name_writes_nothing(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "", ""), groupedEnv("b", "", "")}
	b := groupedBackend(envs...)

	_, bad := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml"}, []string{"Default"}, false)
	_, err := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml", "/e/b.yaml"}, []string{"checkout"}, false)

	if bad == nil || b.Called("SetEnvironmentGroups Default") {
		t.Fatalf("a name the default group owns was accepted: %v", bad)
	}
	if err != nil || !slices.Equal(b.Environments()[0].Groups, []string{"checkout"}) || !slices.Equal(b.Environments()[1].Groups, []string{"checkout"}) {
		t.Fatalf("err %v, environments %+v", err, b.Environments())
	}
}

func TestMoveEnvironments_Given_a_file_with_comments_Then_it_is_named_and_nothing_is_moved_until_the_caller_agrees(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "", "")}
	b := groupedBackend(envs...)
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{File: "/e/a.yaml"}, map[string]string{"file": "a.yaml"})}.Err()

	res, err := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml"}, []string{"checkout"}, false)

	if err != nil || res.Refused != "a" {
		t.Fatalf("result %+v err %v", res, err)
	}
}

func TestRenameGroup_Given_the_rules_Then_a_rename_moves_the_whole_group_and_the_odd_ones_are_refused(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "old", ""), groupedEnv("b", "old", ""), groupedEnv("c", "other", ""), groupedEnv("d", "", "")}
	tests := []struct {
		name, from, to string
		wantErr        string
	}{
		{"the default group", "", "x", "default group cannot be renamed"},
		{"a group that is not there", "ghost", "x", "no group called ghost"},
		{"into an existing group", "old", "other", "already a group called other"},
		{"into the default's name", "old", "default", "name of the group"},
		{"to nothing", "old", "", "give the group a name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := groupedBackend(envs...)

			_, err := vm.RenameGroup(b, envs, tt.from, tt.to, false)

			if err == nil || !strings.Contains(err.Error(), tt.wantErr) || b.Called("RenameEnvironmentGroup "+tt.from+" "+tt.to) {
				t.Fatalf("err %v, calls %v", err, b.Calls)
			}
		})
	}

	b := groupedBackend(envs...)
	_, err := vm.RenameGroup(b, envs, "old", "new", false)
	got := model.EnvironmentGroups(b.Environments())
	if err != nil || !slices.Equal(got, []string{"new", "other"}) {
		t.Fatalf("err %v groups %v", err, got)
	}
}

func TestDeleteGroup_Given_a_group_Then_its_environments_go_to_the_default_group_and_none_is_deleted(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "old", ""), groupedEnv("b", "old", ""), groupedEnv("c", "other", "")}
	b := groupedBackend(envs...)

	_, err := vm.DeleteGroup(b, envs, "old", false)
	_, defaultErr := vm.DeleteGroup(b, envs, "", false)

	got := b.Environments()
	if err != nil || len(got) != 3 || len(got[0].Groups) != 0 || len(got[1].Groups) != 0 || !slices.Equal(got[2].Groups, []string{"other"}) {
		t.Fatalf("err %v environments %+v", err, got)
	}
	if defaultErr == nil {
		t.Fatal("the default group was deleted")
	}
}

// Before: a group existed only while an environment named it, so one could not be made first and filled afterwards.
func TestManageEnvironments_Given_a_group_with_no_environment_Then_it_is_listed_with_a_count_of_zero(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "", ""), groupedEnv("b", "payments", "")}

	m := vm.ManageEnvironments(envs, []string{"empty", "payments"})

	var got []string
	for _, g := range m.Groups {
		got = append(got, g.Label+":"+string(rune('0'+g.Count)))
	}
	if !slices.Equal(got, []string{"Default:1", "empty:0", "payments:1"}) {
		t.Fatalf("groups %v", got)
	}
}

func TestAddGroup_Given_names_Then_a_good_one_is_made_and_a_bad_or_taken_one_is_refused(t *testing.T) {
	b := groupedBackend(groupedEnv("a", "payments", ""))

	err := vm.AddGroup(b, "empty")
	taken := vm.AddGroup(b, "empty")
	used := vm.AddGroup(b, "payments")
	bad := vm.AddGroup(b, "default")
	none := vm.AddGroup(b, "")

	if err != nil || !slices.Equal(b.EnvironmentGroups(), []string{"empty", "payments"}) {
		t.Fatalf("err %v groups %v", err, b.EnvironmentGroups())
	}
	for name, e := range map[string]error{"taken": taken, "used": used, "bad": bad, "none": none} {
		if e == nil {
			t.Errorf("%s was accepted", name)
		}
	}
}

func TestMoveEnvironments_Given_a_new_group_name_Then_the_group_is_made_so_it_stays_when_it_empties(t *testing.T) {
	envs := []model.Environment{groupedEnv("a", "", "")}
	b := groupedBackend(envs...)

	_, err := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml"}, []string{"checkout"}, false)
	_, back := vm.MoveEnvironments(b, b.Environments(), []string{"/e/a.yaml"}, nil, false)

	if err != nil || back != nil || !slices.Equal(b.EnvironmentGroups(), []string{"checkout"}) {
		t.Fatalf("errs %v %v groups %v", err, back, b.EnvironmentGroups())
	}
}

func TestRenameGroup_Given_an_empty_group_Then_it_is_renamed_and_still_empty(t *testing.T) {
	b := groupedBackend(groupedEnv("a", "", ""))
	b.EnvGroups = []string{"old"}

	_, err := vm.RenameGroup(b, b.Environments(), "old", "new", false)

	if err != nil || !slices.Equal(b.EnvironmentGroups(), []string{"new"}) {
		t.Fatalf("err %v groups %v", err, b.EnvironmentGroups())
	}
}

func TestDeleteGroup_Given_an_empty_group_Then_it_is_gone(t *testing.T) {
	b := groupedBackend(groupedEnv("a", "", ""))
	b.EnvGroups = []string{"old"}

	_, err := vm.DeleteGroup(b, b.Environments(), "old", false)

	if err != nil || len(b.EnvironmentGroups()) != 0 {
		t.Fatalf("err %v groups %v", err, b.EnvironmentGroups())
	}
}

func multiEnv(name string, extends string, groups ...string) model.Environment {
	return model.Environment{Meta: model.Meta{Name: name}, File: "/e/" + name + ".yaml", Extends: extends, Groups: groups}
}

// Before: an environment had one group, so one shared by two sets of services was counted and listed in only one.
func TestManageEnvironments_Given_an_environment_in_two_groups_Then_each_group_counts_it(t *testing.T) {
	envs := []model.Environment{multiEnv("shared", "", "checkout", "payments"), multiEnv("a", "", "checkout"), multiEnv("plain", "")}

	m := vm.ManageEnvironments(envs, nil)

	counts := map[string]int{}
	for _, g := range m.Groups {
		counts[g.Label] = g.Count
	}
	if counts["Default"] != 1 || counts["checkout"] != 2 || counts["payments"] != 1 {
		t.Fatalf("counts %v", counts)
	}
	if got := m.Envs[2].Groups; m.Envs[2].Name != "shared" || !slices.Equal(got, []string{"checkout", "payments"}) {
		t.Fatalf("envs %+v", m.Envs)
	}
}

func TestManageEnvironments_Given_a_base_missing_from_one_of_two_groups_Then_the_warning_names_that_group(t *testing.T) {
	envs := []model.Environment{multiEnv("base", "", "checkout"), multiEnv("prod", "base", "checkout", "payments")}

	m := vm.ManageEnvironments(envs, nil)

	if w := m.Envs[1].Warning; !strings.Contains(w, "not in the group payments") || !strings.Contains(w, "it is in the group checkout") {
		t.Fatalf("warning %q", w)
	}
	if m.Envs[0].Warning != "" {
		t.Fatalf("the base was warned about: %q", m.Envs[0].Warning)
	}
}

func TestRenameGroup_Given_an_environment_in_two_groups_Then_it_keeps_the_other_one(t *testing.T) {
	envs := []model.Environment{multiEnv("a", "", "keep", "old")}
	b := groupedBackend(envs...)

	_, err := vm.RenameGroup(b, envs, "old", "new", false)

	if got := b.Environments()[0].Groups; err != nil || !slices.Equal(got, []string{"keep", "new"}) {
		t.Fatalf("err %v groups %v", err, got)
	}
}

func TestDeleteGroup_Given_an_environment_in_two_groups_Then_it_stays_in_the_other_one(t *testing.T) {
	envs := []model.Environment{multiEnv("a", "", "keep", "old")}
	b := groupedBackend(envs...)

	_, err := vm.DeleteGroup(b, envs, "old", false)

	if got := b.Environments()[0].Groups; err != nil || !slices.Equal(got, []string{"keep"}) {
		t.Fatalf("err %v groups %v", err, got)
	}
}

func TestMoveEnvironments_Given_a_repeat_or_empty_name_Then_it_is_tidied_and_no_empty_group_is_made(t *testing.T) {
	envs := []model.Environment{multiEnv("a", "")}
	b := groupedBackend(envs...)

	_, err := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml"}, []string{"x", "", "x", "w"}, false)

	if got := b.Environments()[0].Groups; err != nil || !slices.Equal(got, []string{"w", "x"}) || !slices.Equal(b.EnvironmentGroups(), []string{"w", "x"}) {
		t.Fatalf("err %v groups %v made %v", err, got, b.EnvironmentGroups())
	}
}

// Before: the colour was set only in the environment's own editor.
func TestColourEnvironments_Given_colours_Then_a_good_one_is_set_and_a_bad_one_writes_nothing(t *testing.T) {
	envs := []model.Environment{multiEnv("a", ""), multiEnv("b", "")}
	b := groupedBackend(envs...)

	_, err := vm.ColourEnvironments(b, envs, []string{"/e/a.yaml", "/e/b.yaml"}, "blue", false)
	_, bad := vm.ColourEnvironments(b, envs, []string{"/e/a.yaml"}, "mauve", false)
	_, none := vm.ColourEnvironments(b, envs, []string{"/e/b.yaml"}, "", false)

	got := b.Environments()
	if err != nil || bad == nil || none != nil || got[0].Colour != "blue" || got[1].Colour != "" {
		t.Fatalf("errs %v %v %v, environments %+v", err, bad, none, got)
	}
	if b.Called("SetEnvironmentColour mauve") {
		t.Fatal("a colour that is not offered reached the backend")
	}
}

func TestColourEnvironments_Given_a_file_with_comments_Then_it_is_named_until_the_caller_agrees(t *testing.T) {
	envs := []model.Environment{multiEnv("a", "")}
	b := groupedBackend(envs...)
	b.SaveErr = diag.List{diag.New(diag.CodeWriteDropsComments, diag.Location{File: "/e/a.yaml"}, map[string]string{"file": "a.yaml"})}.Err()

	res, err := vm.ColourEnvironments(b, envs, []string{"/e/a.yaml"}, "red", false)

	if err != nil || res.Refused != "a" {
		t.Fatalf("result %+v err %v", res, err)
	}
}

// Before: the default group could not sit beside a named one, so the screen could not show or set both.
func TestMoveEnvironments_Given_the_default_token_beside_a_group_Then_it_is_kept_and_makes_no_group_called_default(t *testing.T) {
	envs := []model.Environment{multiEnv("a", "")}
	b := groupedBackend(envs...)

	_, err := vm.MoveEnvironments(b, envs, []string{"/e/a.yaml"}, []string{"checkout", model.DefaultGroupToken}, false)

	got := b.Environments()[0]
	if err != nil || !got.InGroup("") || !got.InGroup("checkout") || !slices.Equal(b.EnvironmentGroups(), []string{"checkout"}) {
		t.Fatalf("err %v groups %v made %v", err, got.Groups, b.EnvironmentGroups())
	}
}

func TestManageEnvironments_Given_an_environment_in_default_and_a_group_Then_both_count_it_and_the_base_warning_names_default(t *testing.T) {
	envs := []model.Environment{multiEnv("base", "", "checkout"), multiEnv("prod", "base", "checkout", model.DefaultGroupToken)}

	m := vm.ManageEnvironments(envs, nil)

	counts := map[string]int{}
	for _, g := range m.Groups {
		counts[g.Label] = g.Count
	}
	if counts["Default"] != 1 || counts["checkout"] != 2 {
		t.Fatalf("counts %v", counts)
	}
	if w := m.Envs[1].Warning; !strings.Contains(w, "not in the default group") || !strings.Contains(w, "it is in the group checkout") {
		t.Fatalf("warning %q", w)
	}
}
