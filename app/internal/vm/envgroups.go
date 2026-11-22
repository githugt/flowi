// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// ManagedEnv is one environment as the Manage environments screen lists it.
type ManagedEnv struct {
	Name      string
	File      string
	Groups    []string // as the file holds them: named groups, sorted, and model.DefaultGroupToken when it is in the default group too; none means the default group alone
	Colour    string   // "" is the default colour
	Base      string   // the environment it builds on, or ""
	Protected bool
	Vars      int // how many variables it sets itself
	// Warning is a problem worth showing beside the environment, such as a base in another group; "" for none.
	Warning string
}

// EnvGroup is one group of environments and how many belong to it.
type EnvGroup struct {
	Name  string // "" is the default group
	Label string // what to show: "Default" for the default group, else Name
	Count int
}

// EnvManagement is what the Manage environments screen shows: every environment, and the groups they sit in.
type EnvManagement struct {
	Envs []ManagedEnv
	// Groups lists the default group first (always, even when empty) and then the named ones, sorted, empty ones included.
	Groups []EnvGroup
}

// ManageEnvironments describes the workspace's environments, in file-name order, for the Manage environments screen.
// groups are the named groups the workspace has (Backend.EnvironmentGroups): a group with no environment is listed too.
func ManageEnvironments(envs []model.Environment, groups []string) EnvManagement {
	m := EnvManagement{Envs: make([]ManagedEnv, 0, len(envs))}
	for _, e := range envs {
		m.Envs = append(m.Envs, ManagedEnv{
			Name: e.Name, File: e.File, Groups: slices.Clone(e.Groups), Colour: e.Colour, Base: e.Extends, Protected: e.Protected,
			Vars: len(e.Vars), Warning: baseWarning(e, envs),
		})
	}
	slices.SortStableFunc(m.Envs, func(a, b ManagedEnv) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	m.Groups = append(m.Groups, EnvGroup{Label: model.DefaultGroupLabel, Count: len(model.EnvironmentsInGroup(envs, ""))})
	for _, g := range namedGroups(envs, groups) {
		m.Groups = append(m.Groups, EnvGroup{Name: g, Label: g, Count: len(model.EnvironmentsInGroup(envs, g))})
	}
	return m
}

// namedGroups is the groups the workspace lists together with the ones the environments name, sorted.
func namedGroups(envs []model.Environment, listed []string) []string {
	all := slices.Clone(model.EnvironmentGroups(envs))
	for _, g := range listed {
		if g != "" && !slices.Contains(all, g) {
			all = append(all, g)
		}
	}
	slices.Sort(all)
	return all
}

// baseWarning says that e builds on an environment that is missing from one of e's groups, or on one that is not
// there.
func baseWarning(e model.Environment, envs []model.Environment) string {
	base := strings.TrimSpace(e.Extends)
	if base == "" {
		return ""
	}
	i := slices.IndexFunc(envs, func(x model.Environment) bool { return x.Name == base })
	if i < 0 {
		return fmt.Sprintf("Builds on %s, which does not exist.", base)
	}
	if g, missing := missingGroup(e, envs[i]); missing {
		return fmt.Sprintf("Builds on %s, which is not in %s (it is in %s). Compare shows one group at a time.", base, GroupLabel(g), groupsLabel(envs[i].Memberships()))
	}
	return ""
}

// missingGroup finds a group of e that its base is not in; "" is the default group. The default group comes first, then
// the named ones in sorted order.
func missingGroup(e, base model.Environment) (string, bool) {
	for _, g := range e.Memberships() {
		if !base.InGroup(g) {
			return g, true
		}
	}
	return "", false
}

// CheckGroupName says why name cannot name a group, in words for the screen, or returns nil. The empty name is the
// default group and is allowed here; callers that need a named group check for it.
func CheckGroupName(name string) error { return model.CheckGroup(name) }

// GroupMove is how a change to environment files ended: putting environments in groups, taking them out or colouring
// them.
type GroupMove struct {
	// Refused names the environment whose file holds comments that the change would delete. Nothing was changed; ask,
	// then change again with dropComments set.
	Refused string
}

// MoveEnvironments makes groups the groups of the environments in files (none is the default group alone; the entry
// model.DefaultGroupToken keeps them in the default group as well). The names are
// checked before anything is written, and a group that is not there yet is made first, so it stays when its
// environments leave. A file with comments is refused unless dropComments is set (see GroupMove).
//
// Before: environments had no group to move between, and then at most one.
func MoveEnvironments(b Backend, envs []model.Environment, files []string, groups []string, dropComments bool) (GroupMove, error) {
	groups = model.NormaliseGroups(groups)
	named := slices.DeleteFunc(slices.Clone(groups), func(g string) bool { return g == model.DefaultGroupToken })
	for _, g := range named {
		if err := CheckGroupName(g); err != nil {
			return GroupMove{}, err
		}
	}
	known := b.EnvironmentGroups()
	for _, g := range named {
		if !slices.Contains(known, g) {
			if err := b.AddEnvironmentGroup(g); err != nil {
				return GroupMove{}, fmt.Errorf("making the group: %s", ErrorText(err))
			}
		}
	}
	return groupMoved(envs, b.SetEnvironmentGroups(files, groups, dropComments))
}

// ColourEnvironments marks the environments in files with colour, one of model.EnvColours(); "" is the default colour.
// A file with comments is refused unless dropComments is set (see GroupMove).
//
// Before: a colour was set in the environment's own editor, one environment at a time.
func ColourEnvironments(b Backend, envs []model.Environment, files []string, colour string, dropComments bool) (GroupMove, error) {
	if colour != "" && !slices.Contains(model.EnvColours(), colour) {
		return GroupMove{}, fmt.Errorf("%s is not a colour an environment can have", colour)
	}
	return groupMoved(envs, b.SetEnvironmentColour(files, colour, dropComments))
}

// AddGroup makes a group with no environment in it.
//
// Before: a group existed only while an environment was in it, so it could not be made first and filled afterwards.
func AddGroup(b Backend, name string) error {
	if name == "" {
		return errors.New("give the group a name")
	}
	if err := CheckGroupName(name); err != nil {
		return err
	}
	if slices.Contains(b.EnvironmentGroups(), name) {
		return fmt.Errorf("there is already a group called %s", name)
	}
	if err := b.AddEnvironmentGroup(name); err != nil {
		return fmt.Errorf("making the group: %s", ErrorText(err))
	}
	return nil
}

// RenameGroup gives the group from the name to, with its environments. Renaming into a group that exists is refused:
// it would merge the two, which is a move, and should be one. The default group cannot be renamed.
func RenameGroup(b Backend, envs []model.Environment, from, to string, dropComments bool) (GroupMove, error) {
	groups := b.EnvironmentGroups()
	switch {
	case from == "":
		return GroupMove{}, errors.New("the default group cannot be renamed")
	case !slices.Contains(groups, from):
		return GroupMove{}, fmt.Errorf("there is no group called %s", from)
	case to == "":
		return GroupMove{}, errors.New("give the group a name")
	case from == to:
		return GroupMove{}, nil
	case slices.Contains(groups, to):
		return GroupMove{}, fmt.Errorf("there is already a group called %s: move the environments into it instead", to)
	}
	if err := CheckGroupName(to); err != nil {
		return GroupMove{}, err
	}
	return groupMoved(envs, b.RenameEnvironmentGroup(from, to, dropComments))
}

// DeleteGroup removes a group: its environments move to the default group and none is deleted. The default group
// cannot be deleted.
func DeleteGroup(b Backend, envs []model.Environment, group string, dropComments bool) (GroupMove, error) {
	switch {
	case group == "":
		return GroupMove{}, errors.New("the default group cannot be deleted")
	case !slices.Contains(b.EnvironmentGroups(), group):
		return GroupMove{}, fmt.Errorf("there is no group called %s", group)
	}
	return groupMoved(envs, b.DeleteEnvironmentGroup(group, dropComments))
}

// groupMoved turns what a move returned into a GroupMove: a refusal for comments names the environment.
func groupMoved(envs []model.Environment, err error) (GroupMove, error) {
	var le *diag.ListError
	if errors.As(err, &le) && hasCode(err, diag.CodeWriteDropsComments) {
		return GroupMove{Refused: nameOfFile(envs, le.List[0].File)}, nil
	}
	if err != nil {
		return GroupMove{}, fmt.Errorf("changing the environments: %s", ErrorText(err))
	}
	return GroupMove{}, nil
}

func nameOfFile(envs []model.Environment, file string) string {
	if i := slices.IndexFunc(envs, func(e model.Environment) bool { return e.File == file }); i >= 0 {
		return envs[i].Name
	}
	return strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
}
