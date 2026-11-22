// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// StatusMark is a development status as the window draws it: next to a row of the tree, and in an editor.
type StatusMark struct {
	Name      string `json:"name"`
	Color     string `json:"color"` // a name from StatusColors, drawn from the theme
	Icon      string `json:"icon"`
	From      string `json:"from,omitempty"`      // a status the item inherits: the folder it comes from, as its path in the collection
	Inherited bool   `json:"inherited,omitempty"` // the item sets none of its own
	Unknown   bool   `json:"unknown,omitempty"`   // the collection has no status of this name (FLW-STATUS-001)
}

// TagChoice is a tag already in use and how many items carry it.
type TagChoice struct {
	Tag   string `json:"tag"`
	Count int    `json:"count"`
}

// LabelsView is what the Tags and status editor shows for one request, flow or folder.
type LabelsView struct {
	Node       string       `json:"node"`
	Kind       string       `json:"kind"` // request, flow or folder
	Tags       []string     `json:"tags"` // its own
	Status     string       `json:"status"`
	Inherited  *StatusMark  `json:"inherited,omitempty"` // what applies when Status is "": a folder's
	FolderTags []string     `json:"folderTags"`          // those the folders above add
	Statuses   []StatusMark `json:"statuses"`            // the choices of its collection
	Suggest    []TagChoice  `json:"suggest"`             // tags the collection uses, most used first
	Colors     []string     `json:"colors"`              // what a new status may be coloured
	Icons      []string     `json:"icons"`               // what a new status may be drawn with
}

func markOf(n vm.TreeNode) *StatusMark {
	if n.Status.Name == "" {
		return nil
	}
	return &StatusMark{Name: n.Status.Name, Color: n.Status.Color, Icon: n.Status.Icon, From: n.StatusFrom, Inherited: n.StatusFrom != "", Unknown: n.StatusUnknown}
}

func markOfStatus(s model.Status) StatusMark {
	return StatusMark{Name: s.Name, Color: s.Color, Icon: s.Icon}
}

// Labels is the tags and status of a request, flow or folder row, with the choices to pick from.
func (s *WorkspaceService) Labels(node string) (LabelsView, error) {
	var (
		l   vm.Labels
		err error
	)
	s.app.actor.Do(func() { l, err = s.app.workspace.Labels(node) })
	if err != nil {
		return LabelsView{}, err
	}
	out := LabelsView{
		Node: node, Kind: kindNames()[l.Kind], Tags: nonNil(l.Tags), Status: l.Status, FolderTags: nonNil(l.FolderTags),
		Statuses: []StatusMark{}, Suggest: []TagChoice{}, Colors: model.StatusColors(), Icons: model.StatusIcons(),
	}
	for _, st := range l.Statuses {
		out.Statuses = append(out.Statuses, markOfStatus(st))
	}
	for _, t := range l.Suggest {
		out.Suggest = append(out.Suggest, TagChoice{Tag: t.Tag, Count: t.Count})
	}
	if l.Inherited.Name != "" {
		m := markOfStatus(l.Inherited)
		m.From, m.Inherited = l.InheritedFrom, true
		out.Inherited = &m
	}
	return out, nil
}

// SetLabels sets the tags and the status ("" for none of its own) of a request, flow or folder row, in its file. It
// returns true, with nothing written, when the file has comments that writing would drop: the window asks, then calls
// again with dropComments.
func (s *WorkspaceService) SetLabels(node string, tags []string, status string, dropComments bool) (bool, error) {
	return s.written(func(done func(error)) { s.app.workspace.SetLabels(node, tags, status, dropComments, done) })
}

// AddStatus adds a status to the collection of the row node, or changes the colour and icon of the one of that name.
// It returns true, with nothing written, when the collection file has comments that writing would drop.
func (s *WorkspaceService) AddStatus(node, name, color, icon string, dropComments bool) (bool, error) {
	st := model.Status{Name: name, Color: color, Icon: icon}
	return s.written(func(done func(error)) { s.app.workspace.AddStatus(node, st, dropComments, done) })
}

// written runs a write on the actor and waits for its outcome, outside the actor, which the write needs to finish.
func (s *WorkspaceService) written(start func(done func(error))) (bool, error) {
	outcome := make(chan error, 1)
	s.app.actor.Do(func() { start(func(err error) { outcome <- err }) })
	err := <-outcome
	switch {
	case err == nil:
		return false, nil
	case vm.IsDropsComments(err):
		return true, nil
	}
	return false, errors.New(vm.ErrorText(err))
}
