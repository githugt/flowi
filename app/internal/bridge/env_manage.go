// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// ManagedEnvView is one environment on the Manage environments screen.
type ManagedEnvView struct {
	Name      string   `json:"name"`
	File      string   `json:"file"`
	Groups    []string `json:"groups"` // as the file holds them: named groups, plus "default" when it is in the default group too; none means the default group alone
	Colour    string   `json:"colour"` // "" is the default colour
	Base      string   `json:"base"`   // the environment it builds on, or ""
	Protected bool     `json:"protected"`
	Vars      int      `json:"vars"`
	Warning   string   `json:"warning"` // a problem to show beside it, or ""
}

// EnvManageView is everything the Manage environments screen shows.
type EnvManageView struct {
	Envs    []ManagedEnvView `json:"envs"`
	Groups  []EnvGroupView   `json:"groups"`
	Colours []string         `json:"colours"` // the colours an environment can be marked with, for the picker
}

// EnvMoveView is how a change to environment files ended: a move between groups or a new colour.
type EnvMoveView struct {
	// Refused names the environment whose file has comments that the change would delete. Nothing changed: ask, then
	// call again with dropComments set.
	Refused string `json:"refused"`
}

// Manage lists every environment of the workspace with its group, colour and base, and the groups.
func (s *EnvironmentService) Manage() EnvManageView {
	m := vm.ManageEnvironments(s.app.backend.Environments(), s.app.backend.EnvironmentGroups())
	v := EnvManageView{Envs: make([]ManagedEnvView, 0, len(m.Envs)), Groups: groupViews(m.Groups), Colours: model.EnvColours()}
	for _, e := range m.Envs {
		v.Envs = append(v.Envs, ManagedEnvView{Name: e.Name, File: e.File, Groups: nonNil(e.Groups), Colour: e.Colour, Base: e.Base, Protected: e.Protected, Vars: e.Vars, Warning: e.Warning})
	}
	return v
}

// AddGroup makes a group with no environment in it, to move environments into afterwards.
func (s *EnvironmentService) AddGroup(name string) error {
	return vm.AddGroup(s.app.backend, name)
}

// MoveToGroups makes groups the groups of the environments in files; none is the default group. A name that is new
// makes the group first, so it stays when its environments leave.
func (s *EnvironmentService) MoveToGroups(files []string, groups []string, dropComments bool) (EnvMoveView, error) {
	envs := s.app.backend.Environments()
	return s.moved(vm.MoveEnvironments(s.app.backend, envs, files, groups, dropComments))
}

// ColourEnvironments marks the environments in files with a colour; "" is the default colour.
func (s *EnvironmentService) ColourEnvironments(files []string, colour string, dropComments bool) (EnvMoveView, error) {
	envs := s.app.backend.Environments()
	return s.moved(vm.ColourEnvironments(s.app.backend, envs, files, colour, dropComments))
}

// RenameGroup renames a group in each of its environments, which keep their other groups.
func (s *EnvironmentService) RenameGroup(from, to string, dropComments bool) (EnvMoveView, error) {
	envs := s.app.backend.Environments()
	return s.moved(vm.RenameGroup(s.app.backend, envs, from, to, dropComments))
}

// DeleteGroup takes the environments out of a group; one left in no group is in the default group. No environment
// is deleted.
func (s *EnvironmentService) DeleteGroup(group string, dropComments bool) (EnvMoveView, error) {
	envs := s.app.backend.Environments()
	return s.moved(vm.DeleteGroup(s.app.backend, envs, group, dropComments))
}

// moved reloads the workspace when something moved, so the open editors and the other screens see the new groups.
func (s *EnvironmentService) moved(res vm.GroupMove, err error) (EnvMoveView, error) {
	if err != nil {
		return EnvMoveView{}, err
	}
	if res.Refused == "" {
		s.app.actor.Do(s.app.workspace.Load)
	}
	return EnvMoveView{Refused: res.Refused}, nil
}
