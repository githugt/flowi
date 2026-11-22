// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
)

// Before: environments had no group, so there was nothing to list, move, rename or delete.
func TestEnvironmentService_Manage_Given_moves_Then_the_screen_sees_the_groups_change(t *testing.T) {
	app := compareApp(t, env("a", ""), env("b", ""), env("c", ""))
	svc := app.Environments()

	_, errMove := svc.MoveToGroups([]string{"/e/a.yaml", "/e/b.yaml"}, []string{"checkout"}, false)
	afterMove := svc.Manage()
	_, errRename := svc.RenameGroup("checkout", "payments", false)
	afterRename := svc.Manage()
	_, errDelete := svc.DeleteGroup("payments", false)
	afterDelete := svc.Manage()

	if errMove != nil || len(afterMove.Groups) != 2 || afterMove.Groups[0].Count != 1 || afterMove.Groups[1].Name != "checkout" || afterMove.Groups[1].Count != 2 {
		t.Fatalf("after the move: %v %+v", errMove, afterMove.Groups)
	}
	if errRename != nil || afterRename.Groups[1].Name != "payments" || !slices.Equal(afterRename.Envs[0].Groups, []string{"payments"}) {
		t.Fatalf("after the rename: %v %+v", errRename, afterRename)
	}
	if errDelete != nil || len(afterDelete.Groups) != 1 || afterDelete.Groups[0].Count != 3 || len(afterDelete.Envs) != 3 {
		t.Fatalf("after the delete: %v %+v", errDelete, afterDelete)
	}
}

func TestEnvironmentService_MoveToGroups_Given_a_name_that_cannot_be_used_Then_it_is_an_error_and_nothing_moves(t *testing.T) {
	app := compareApp(t, env("a", ""))

	_, err := app.Environments().MoveToGroups([]string{"/e/a.yaml"}, []string{"Default"}, false)

	if err == nil || len(app.Environments().Manage().Envs[0].Groups) != 0 {
		t.Fatalf("err %v", err)
	}
}

func TestEnvironmentService_Manage_Given_a_base_in_another_group_Then_the_environment_has_a_warning(t *testing.T) {
	prod := env("prod", "checkout")
	prod.Extends = "base"
	app := compareApp(t, env("base", "", model.EnvVar{Name: "x", Value: "1"}), prod)

	v := app.Environments().Manage()

	if v.Envs[0].Name != "base" || v.Envs[1].Warning == "" || v.Envs[1].Base != "base" || v.Envs[0].Warning != "" {
		t.Fatalf("envs %+v", v.Envs)
	}
}

// Before: there was no way to make a group before an environment was in it.
func TestEnvironmentService_AddGroup_Given_a_new_name_Then_the_screen_lists_an_empty_group_that_can_be_filled_and_removed(t *testing.T) {
	app := compareApp(t, env("a", ""))
	svc := app.Environments()

	errAdd := svc.AddGroup("checkout")
	listed := svc.Manage()
	_, errMove := svc.MoveToGroups([]string{"/e/a.yaml"}, []string{"checkout"}, false)
	_, errBack := svc.MoveToGroups([]string{"/e/a.yaml"}, nil, false)
	stays := svc.Manage()
	_, errDelete := svc.DeleteGroup("checkout", false)
	gone := svc.Manage()

	if errAdd != nil || len(listed.Groups) != 2 || listed.Groups[1].Name != "checkout" || listed.Groups[1].Count != 0 {
		t.Fatalf("after add: %v %+v", errAdd, listed.Groups)
	}
	if errMove != nil || errBack != nil || len(stays.Groups) != 2 {
		t.Fatalf("an emptied group went away: %v %v %+v", errMove, errBack, stays.Groups)
	}
	if errDelete != nil || len(gone.Groups) != 1 {
		t.Fatalf("after delete: %v %+v", errDelete, gone.Groups)
	}
	if svc.AddGroup("default") == nil {
		t.Fatal("a name the default group owns was accepted")
	}
}

// Before: an environment had one group, so the screen could not show or set a second one.
func TestEnvironmentService_MoveToGroups_Given_two_groups_Then_the_environment_is_listed_in_both_and_each_group_counts_it(t *testing.T) {
	app := compareApp(t, env("a", ""), env("b", "checkout"))
	svc := app.Environments()

	_, err := svc.MoveToGroups([]string{"/e/a.yaml"}, []string{"payments", "checkout"}, false)
	v := svc.Manage()

	if err != nil || !slices.Equal(v.Envs[0].Groups, []string{"checkout", "payments"}) {
		t.Fatalf("err %v envs %+v", err, v.Envs)
	}
	if len(v.Groups) != 3 || v.Groups[0].Count != 0 || v.Groups[1].Count != 2 || v.Groups[2].Count != 1 {
		t.Fatalf("groups %+v", v.Groups)
	}
}

// Before: the colour could be set only in the environment's own editor.
func TestEnvironmentService_ColourEnvironments_Given_a_colour_Then_the_screen_shows_it_and_a_bad_one_is_refused(t *testing.T) {
	app := compareApp(t, env("a", ""), env("b", ""))
	svc := app.Environments()

	_, err := svc.ColourEnvironments([]string{"/e/a.yaml"}, "purple", false)
	_, bad := svc.ColourEnvironments([]string{"/e/b.yaml"}, "mauve", false)
	v := svc.Manage()

	if err != nil || v.Envs[0].Colour != "purple" {
		t.Fatalf("err %v envs %+v", err, v.Envs)
	}
	if bad == nil || v.Envs[1].Colour != "" {
		t.Fatalf("a colour that is not offered was accepted: %v %+v", bad, v.Envs[1])
	}
}
