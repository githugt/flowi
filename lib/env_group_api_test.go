// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/vault"
	"github.com/githugt/flowi/lib/workspace"
)

const (
	groupEnvA = "id: 01JA0000000000000000000011\nvars:\n  base_url: http://a.example\n  token:\n    secret: true\n"
	groupEnvB = "id: 01JA0000000000000000000012\nextends: a\nvars:\n  base_url: http://b.example\n"
	groupEnvC = "id: 01JA0000000000000000000013\nvars:\n  x: 1\n"
)

func groupWorkspace(t *testing.T) (env, *Workspace) {
	t.Helper()
	e := newEnv(t)
	e.opts.Keys = &memKeys{}
	writeGlobalEnv(t, e, "a", groupEnvA)
	writeGlobalEnv(t, e, "b", groupEnvB)
	writeGlobalEnv(t, e, "c", groupEnvC)
	return e, e.open(t)
}

func envFile(t *testing.T, w *Workspace, name string) string {
	t.Helper()
	for _, e := range w.Environments() {
		if e.Name == name {
			return e.File
		}
	}
	t.Fatalf("no environment %q in %v", name, environmentNames(w.Environments()))
	return ""
}

// groupOf is the groups of an environment as one string, sorted and joined by "+"; "" is the default group.
func groupOf(t *testing.T, w *Workspace, name string) string {
	t.Helper()
	for _, e := range w.Environments() {
		if e.Name == name {
			return strings.Join(e.Groups, "+")
		}
	}
	t.Fatalf("no environment %q", name)
	return ""
}

// Before: environments had no group, so a comparison held all of them however many sets of services they served.
func TestSetEnvironmentGroups_Given_two_environments_When_moved_Then_they_are_in_the_group_after_a_reopen_and_keep_their_secrets(t *testing.T) {
	e, w := groupWorkspace(t)
	if err := w.SetSecret("a", "token", "fake-token"); err != nil {
		t.Fatal(err)
	}

	err := w.SetEnvironmentGroups([]string{envFile(t, w, "a"), envFile(t, w, "b")}, []string{"checkout"}, SaveOptions{})
	again := e.open(t)

	if err != nil {
		t.Fatal(err)
	}
	if groupOf(t, w, "a") != "checkout" || groupOf(t, w, "b") != "checkout" || groupOf(t, w, "c") != "" {
		t.Fatalf("groups %+v", w.Environments())
	}
	if groupOf(t, again, "a") != "checkout" {
		t.Fatalf("the group did not survive a reopen: %+v", again.Environments())
	}
	if got := model.EnvironmentGroups(again.Environments()); !slices.Equal(got, []string{"checkout"}) {
		t.Fatalf("groups %v", got)
	}
	stored, err := again.secrets().Names(vault.WorkspaceScope)
	if err != nil || len(stored) != 1 || stored[0].Name != "token" {
		t.Fatalf("the secret did not stay with its environment: %+v %v", stored, err)
	}
	if b := environmentByName(again, "b"); b.Extends != "a" {
		t.Fatalf("the base was lost: %+v", b)
	}
}

func environmentByName(w *Workspace, name string) model.Environment {
	for _, e := range w.Environments() {
		if e.Name == name {
			return e
		}
	}
	return model.Environment{}
}

func TestSetEnvironmentGroups_Given_the_empty_name_When_moved_Then_the_environment_is_back_in_the_default_group(t *testing.T) {
	_, w := groupWorkspace(t)
	a := envFile(t, w, "a")
	if err := w.SetEnvironmentGroups([]string{a}, []string{"checkout"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.SetEnvironmentGroups([]string{a}, nil, SaveOptions{})

	data, _ := os.ReadFile(a)
	if err != nil || groupOf(t, w, "a") != "" || strings.Contains(string(data), "groups:") {
		t.Fatalf("err %v, file:\n%s", err, data)
	}
}

func TestSetEnvironmentGroups_Given_a_bad_name_or_an_unknown_file_When_moved_Then_nothing_is_written(t *testing.T) {
	_, w := groupWorkspace(t)
	a := envFile(t, w, "a")
	before, _ := os.ReadFile(a)
	tests := []struct {
		name  string
		files []string
		group string
		want  diag.Code
	}{
		{"the default's own name", []string{a}, "Default", diag.CodeRequestEnvGroup},
		{"a name with a space at the end", []string{a}, "checkout ", diag.CodeRequestEnvGroup},
		{"an unknown file after a good one", []string{a, filepath.Join(t.TempDir(), "nope.yaml")}, "checkout", diag.CodeRequestEnvUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.SetEnvironmentGroups(tt.files, []string{tt.group}, SaveOptions{})

			if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{tt.want}) {
				t.Fatalf("err %v, want %s", err, tt.want)
			}
			after, _ := os.ReadFile(a)
			if string(after) != string(before) {
				t.Fatalf("the file changed:\n%s", after)
			}
		})
	}
}

func TestSetEnvironmentGroups_Given_a_file_with_comments_When_moved_Then_it_is_refused_until_comments_may_go(t *testing.T) {
	e := newEnv(t)
	writeGlobalEnv(t, e, "a", "# my notes\n"+groupEnvA)
	w := e.open(t)
	a := envFile(t, w, "a")

	err := w.SetEnvironmentGroups([]string{a}, []string{"checkout"}, SaveOptions{})

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeWriteDropsComments}) || groupOf(t, w, "a") != "" {
		t.Fatalf("err %v", err)
	}
	if err := w.SetEnvironmentGroups([]string{a}, []string{"checkout"}, SaveOptions{DropComments: true}); err != nil || groupOf(t, w, "a") != "checkout" {
		t.Fatalf("err %v", err)
	}
}

func TestSetEnvironmentGroups_Given_an_environment_already_in_the_group_When_moved_Then_its_file_is_not_rewritten(t *testing.T) {
	e := newEnv(t)
	writeGlobalEnv(t, e, "a", "# my notes\ngroups:\n  - checkout\n"+groupEnvA)
	w := e.open(t)

	err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"checkout"}, SaveOptions{})

	if err != nil {
		t.Fatalf("a no-op move was refused: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(workspace.EnvironmentsDir(e.opts.Paths, workspace.DefaultName), "a.yaml"))
	if !strings.Contains(string(data), "# my notes") {
		t.Fatalf("the file was rewritten:\n%s", data)
	}
}

// Before: a group existed only while an environment named it, so one could not be made first and filled afterwards.
func TestAddEnvironmentGroup_Given_a_new_name_Then_the_group_exists_with_no_environment_and_survives_a_reopen(t *testing.T) {
	e, w := groupWorkspace(t)

	err := w.AddEnvironmentGroup("checkout")
	again := e.open(t)

	if err != nil {
		t.Fatal(err)
	}
	for _, ws := range []*Workspace{w, again} {
		if got := ws.EnvironmentGroups(); !slices.Equal(got, []string{"checkout"}) {
			t.Fatalf("groups %v", got)
		}
		if n := len(model.EnvironmentsInGroup(ws.Environments(), "checkout")); n != 0 {
			t.Fatalf("%d environments in a group nobody was moved into", n)
		}
	}
}

func TestAddEnvironmentGroup_Given_a_bad_or_taken_name_Then_it_is_refused_with_FLW_REQ_014(t *testing.T) {
	_, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"used"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEnvironmentGroup("listed"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"", "Default", " x", "used", "listed"} {
		err := w.AddEnvironmentGroup(name)

		if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeRequestEnvGroupUse}) {
			t.Errorf("%q: err %v", name, err)
		}
	}
}

func TestRenameEnvironmentGroup_Given_a_group_with_environments_and_an_empty_one_Then_the_names_change_everywhere(t *testing.T) {
	e, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a"), envFile(t, w, "b")}, []string{"old"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEnvironmentGroup("empty"); err != nil {
		t.Fatal(err)
	}

	errFull := w.RenameEnvironmentGroup("old", "new", SaveOptions{})
	errEmpty := w.RenameEnvironmentGroup("empty", "still-empty", SaveOptions{})
	again := e.open(t)

	if errFull != nil || errEmpty != nil {
		t.Fatalf("errors %v %v", errFull, errEmpty)
	}
	if got := again.EnvironmentGroups(); !slices.Equal(got, []string{"new", "still-empty"}) {
		t.Fatalf("groups %v", got)
	}
	if groupOf(t, again, "a") != "new" || groupOf(t, again, "b") != "new" {
		t.Fatalf("environments %+v", again.Environments())
	}
}

func TestRenameEnvironmentGroup_Given_the_odd_cases_Then_each_is_refused_and_nothing_changes(t *testing.T) {
	_, w := groupWorkspace(t)
	for _, g := range []string{"one", "two"} {
		if err := w.AddEnvironmentGroup(g); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct{ name, from, to string }{
		{"the default group", "", "x"},
		{"a group that is not there", "ghost", "x"},
		{"into an existing group", "one", "two"},
		{"into a name the default group owns", "one", "default"},
		{"to nothing", "one", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.RenameEnvironmentGroup(tt.from, tt.to, SaveOptions{})

			if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeRequestEnvGroupUse}) {
				t.Fatalf("err %v", err)
			}
			if got := w.EnvironmentGroups(); !slices.Equal(got, []string{"one", "two"}) {
				t.Fatalf("groups %v", got)
			}
		})
	}
}

func TestDeleteEnvironmentGroup_Given_a_group_with_an_environment_Then_it_goes_to_the_default_group_and_none_is_deleted(t *testing.T) {
	e, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"old"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := w.AddEnvironmentGroup("empty"); err != nil {
		t.Fatal(err)
	}

	errFull := w.DeleteEnvironmentGroup("old", SaveOptions{})
	errEmpty := w.DeleteEnvironmentGroup("empty", SaveOptions{})
	errDefault := w.DeleteEnvironmentGroup("", SaveOptions{})
	again := e.open(t)

	if errFull != nil || errEmpty != nil || errDefault == nil {
		t.Fatalf("errors %v %v %v", errFull, errEmpty, errDefault)
	}
	if got := again.EnvironmentGroups(); len(got) != 0 {
		t.Fatalf("groups %v", got)
	}
	if len(again.Environments()) != 3 || groupOf(t, again, "a") != "" {
		t.Fatalf("environments %+v", again.Environments())
	}
}

// Before: an environment had one group, so a service shared by two sets of environments was in only one comparison.
func TestSetEnvironmentGroups_Given_several_groups_Then_the_environment_is_in_each_and_they_are_sorted_without_repeats(t *testing.T) {
	e, w := groupWorkspace(t)

	err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"payments", "checkout", "payments"}, SaveOptions{})
	again := e.open(t)

	if err != nil {
		t.Fatal(err)
	}
	if got := groupOf(t, again, "a"); got != "checkout+payments" {
		t.Fatalf("groups %q", got)
	}
	for _, g := range []string{"checkout", "payments"} {
		if in := model.EnvironmentsInGroup(again.Environments(), g); len(in) != 1 || in[0].Name != "a" {
			t.Errorf("%s holds %v", g, in)
		}
	}
	if in := model.EnvironmentsInGroup(again.Environments(), ""); len(in) != 2 {
		t.Errorf("the default group holds %v", in)
	}
}

func TestRenameEnvironmentGroup_Given_an_environment_in_two_groups_Then_only_the_renamed_one_changes(t *testing.T) {
	_, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"keep", "old"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.RenameEnvironmentGroup("old", "new", SaveOptions{})

	if err != nil || groupOf(t, w, "a") != "keep+new" {
		t.Fatalf("err %v, groups %q", err, groupOf(t, w, "a"))
	}
}

func TestDeleteEnvironmentGroup_Given_an_environment_in_two_groups_Then_it_stays_in_the_other_one(t *testing.T) {
	_, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"keep", "old"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.DeleteEnvironmentGroup("old", SaveOptions{})

	if err != nil || groupOf(t, w, "a") != "keep" {
		t.Fatalf("err %v, groups %q", err, groupOf(t, w, "a"))
	}
}

// Before: an environment's colour could be changed only in its own editor, one environment at a time.
func TestSetEnvironmentColour_Given_two_environments_Then_both_change_after_a_reopen_and_the_rest_of_the_file_stays(t *testing.T) {
	e, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"checkout"}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.SetEnvironmentColour([]string{envFile(t, w, "a"), envFile(t, w, "b")}, "purple", SaveOptions{})
	again := e.open(t)

	if err != nil {
		t.Fatal(err)
	}
	if a, b, c := environmentByName(again, "a"), environmentByName(again, "b"), environmentByName(again, "c"); a.Colour != "purple" || b.Colour != "purple" || c.Colour != "" {
		t.Fatalf("colours %q %q %q", a.Colour, b.Colour, c.Colour)
	}
	if a := environmentByName(again, "a"); !slices.Equal(a.Groups, []string{"checkout"}) || len(a.Vars) != 2 {
		t.Fatalf("the rest of the file changed: %+v", a)
	}
	if b := environmentByName(again, "b"); b.Extends != "a" {
		t.Fatalf("the base was lost: %+v", b)
	}
}

func TestSetEnvironmentColour_Given_the_empty_colour_Then_the_default_is_back_and_no_colour_line_stays(t *testing.T) {
	_, w := groupWorkspace(t)
	a := envFile(t, w, "a")
	if err := w.SetEnvironmentColour([]string{a}, "red", SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.SetEnvironmentColour([]string{a}, "", SaveOptions{})

	data, _ := os.ReadFile(a)
	if err != nil || strings.Contains(string(data), "colour:") {
		t.Fatalf("err %v, file:\n%s", err, data)
	}
}

func TestSetEnvironmentColour_Given_an_unknown_colour_file_or_comments_Then_it_is_refused_and_nothing_is_written(t *testing.T) {
	e := newEnv(t)
	writeGlobalEnv(t, e, "a", groupEnvA)
	writeGlobalEnv(t, e, "b", "# my notes\n"+groupEnvC)
	w := e.open(t)
	a, b := envFile(t, w, "a"), envFile(t, w, "b")
	before, _ := os.ReadFile(a)
	tests := []struct {
		name   string
		files  []string
		colour string
		want   diag.Code
	}{
		{"a colour that is not offered", []string{a}, "mauve", diag.CodeRequestEnvColour},
		{"an unknown file after a good one", []string{a, filepath.Join(t.TempDir(), "nope.yaml")}, "red", diag.CodeRequestEnvUnknown},
		{"a file with comments after a good one", []string{a, b}, "red", diag.CodeWriteDropsComments},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := w.SetEnvironmentColour(tt.files, tt.colour, SaveOptions{})

			if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{tt.want}) {
				t.Fatalf("err %v, want %s", err, tt.want)
			}
			if after, _ := os.ReadFile(a); string(after) != string(before) {
				t.Fatalf("the file changed:\n%s", after)
			}
		})
	}
	if err := w.SetEnvironmentColour([]string{b}, "red", SaveOptions{DropComments: true}); err != nil {
		t.Fatalf("with the comments allowed to go: %v", err)
	}
}

// Before: an environment was in the default group only with no named group, so "default and checkout" could not be kept.
func TestSetEnvironmentGroups_Given_the_default_token_beside_a_group_Then_it_is_in_both_after_a_reopen_and_alone_it_is_no_line(t *testing.T) {
	e, w := groupWorkspace(t)
	a := envFile(t, w, "a")

	err := w.SetEnvironmentGroups([]string{a}, []string{"checkout", model.DefaultGroupToken}, SaveOptions{})
	again := e.open(t)

	if err != nil {
		t.Fatal(err)
	}
	if got := environmentByName(again, "a"); !got.InGroup("") || !got.InGroup("checkout") {
		t.Fatalf("groups %v", got.Groups)
	}
	if got := model.EnvironmentsInGroup(again.Environments(), ""); len(got) != 3 {
		t.Fatalf("the default group holds %d environments, want all 3", len(got))
	}
	if got := again.EnvironmentGroups(); !slices.Equal(got, []string{"checkout"}) {
		t.Fatalf("the token showed up as a group: %v", got)
	}
	if err := w.SetEnvironmentGroups([]string{a}, []string{model.DefaultGroupToken}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(a)
	if strings.Contains(string(data), "groups:") {
		t.Fatalf("the token alone was written:\n%s", data)
	}
}

func TestDeleteEnvironmentGroup_Given_an_environment_in_default_and_the_group_Then_it_stays_in_default(t *testing.T) {
	_, w := groupWorkspace(t)
	if err := w.SetEnvironmentGroups([]string{envFile(t, w, "a")}, []string{"old", model.DefaultGroupToken}, SaveOptions{}); err != nil {
		t.Fatal(err)
	}

	err := w.DeleteEnvironmentGroup("old", SaveOptions{})

	if err != nil || !environmentByName(w, "a").InGroup("") || len(environmentByName(w, "a").Groups) != 0 {
		t.Fatalf("err %v groups %v", err, environmentByName(w, "a").Groups)
	}
}
