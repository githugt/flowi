// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// Environment groups come from two places: the list in workspace.yaml (`environment_groups:`), which is what lets a
// group exist with no environment in it, and the `groups:` line of the environment files, so a group that came with a
// shared file is there without being listed. EnvironmentGroups is their union.

// EnvironmentGroups lists the named environment groups, sorted: those listed in workspace.yaml and those an
// environment names. A listed name that cannot be a group (model.CheckGroup) is left out.
func (w *Workspace) EnvironmentGroups() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.environmentGroups()
}

// environmentGroups is EnvironmentGroups for a caller that holds mu.
func (w *Workspace) environmentGroups() []string {
	all := slices.Clone(model.EnvironmentGroups(w.globalEnvs))
	for _, g := range w.model.EnvironmentGroups {
		if g != "" && model.CheckGroup(g) == nil && !slices.Contains(all, g) {
			all = append(all, g)
		}
	}
	slices.Sort(all)
	return all
}

// AddEnvironmentGroup makes a group with no environment in it, kept in workspace.yaml. A name that cannot be a group
// or that is taken is FLW-REQ-014 and nothing is written.
//
// Before: a group existed only while an environment named it, so one could not be made first and filled afterwards.
func (w *Workspace) AddEnvironmentGroup(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if err := checkNewGroup(name, w.environmentGroups()); err != nil {
		return err
	}
	next := w.model
	next.EnvironmentGroups = append(slices.Clone(w.model.EnvironmentGroups), name)
	return w.saveModel(next)
}

// RenameEnvironmentGroup gives the group from the name to: each of its environments swaps the one name for the other
// and keeps its other groups (see SetEnvironmentGroups, whose refusals apply), and the listed name changes with them. Renaming into a group that exists, or renaming one that is not
// there, is FLW-REQ-014. The default group has no name to change.
func (w *Workspace) RenameEnvironmentGroup(from, to string, o SaveOptions) error {
	w.mu.RLock()
	groups := w.environmentGroups()
	w.mu.RUnlock()
	switch {
	case from == "":
		return groupDiagErr(from, "the default group cannot be renamed")
	case !slices.Contains(groups, from):
		return groupDiagErr(from, "there is no such group")
	case from == to:
		return nil
	}
	if err := checkNewGroup(to, groups); err != nil {
		return err
	}
	if err := w.renameGroupMembers(from, to, o); err != nil {
		return err
	}
	return w.listGroups(func(list []string) []string {
		return append(slices.DeleteFunc(list, func(g string) bool { return g == from }), to)
	})
}

// DeleteEnvironmentGroup removes a group: each of its environments leaves it (one left in no other group is then in the
// default group), and none is deleted. A group
// that is not there is FLW-REQ-014; the default group cannot be deleted.
func (w *Workspace) DeleteEnvironmentGroup(name string, o SaveOptions) error {
	w.mu.RLock()
	groups := w.environmentGroups()
	w.mu.RUnlock()
	switch {
	case name == "":
		return groupDiagErr(name, "the default group cannot be deleted")
	case !slices.Contains(groups, name):
		return groupDiagErr(name, "there is no such group")
	}
	if err := w.leaveGroupMembers(name, o); err != nil {
		return err
	}
	return w.listGroups(func(list []string) []string {
		return slices.DeleteFunc(list, func(g string) bool { return g == name })
	})
}

// renameGroupMembers swaps the name from for to in every environment of the group from.
func (w *Workspace) renameGroupMembers(from, to string, o SaveOptions) error {
	return w.changeGroups(w.groupFiles(from), func(groups []string) []string {
		return append(slices.DeleteFunc(groups, func(g string) bool { return g == from }), to)
	}, o)
}

// leaveGroupMembers takes every environment of the group out of it.
func (w *Workspace) leaveGroupMembers(group string, o SaveOptions) error {
	return w.changeGroups(w.groupFiles(group), func(groups []string) []string {
		return slices.DeleteFunc(groups, func(g string) bool { return g == group })
	}, o)
}

// groupFiles lists the files of the environments in a named group.
func (w *Workspace) groupFiles(group string) []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	var files []string
	for _, e := range model.EnvironmentsInGroup(w.globalEnvs, group) {
		files = append(files, e.File)
	}
	return files
}

// listGroups changes the list in workspace.yaml and writes it, when the change leaves it different.
func (w *Workspace) listGroups(change func([]string) []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	next := w.model
	next.EnvironmentGroups = change(slices.Clone(w.model.EnvironmentGroups))
	if slices.Equal(next.EnvironmentGroups, w.model.EnvironmentGroups) {
		return nil
	}
	return w.saveModel(next)
}

func checkNewGroup(name string, existing []string) error {
	switch {
	case name == "":
		return groupDiagErr(name, "give the group a name")
	case model.CheckGroup(name) != nil:
		return groupDiagErr(name, model.CheckGroup(name).Error())
	case slices.Contains(existing, name):
		return groupDiagErr(name, "there is already a group with this name")
	}
	return nil
}

func groupDiagErr(group, detail string) error {
	return diagErr(diag.CodeRequestEnvGroupUse, map[string]string{"group": group, "detail": detail})
}
