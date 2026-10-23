// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
)

// The sidebar's workspace name was a label that did nothing. It is now the
// workspace's menu: rename it, show its folder, open its settings. The
// view-model's backend knows only the name, so the workspace's own file is
// handed in apart (WithWorkspaceFile).

// WorkspaceFile is workspace.yaml: its name and the folder holding it.
type WorkspaceFile interface {
	Rename(name string) error
	Folder() string
}

// WithWorkspaceFile lets the window rename the workspace and show its folder.
func WithWorkspaceFile(f WorkspaceFile) Option {
	return func(o *appOptions) { o.wsFile = f }
}

var (
	errNoWorkspaceFile = errors.New("the workspace's file is not available here")
	errNoReveal        = errors.New("showing a folder is not available here")
)

// RenameWorkspace changes the name the sidebar shows; the folder keeps its
// own. The new name is published with the tree.
func (s *WorkspaceService) RenameWorkspace(name string) error {
	if s.app.wsFile == nil {
		return errNoWorkspaceFile
	}
	if err := s.app.wsFile.Rename(name); err != nil {
		return fmt.Errorf("renaming the workspace: %w", err)
	}
	s.app.actor.Do(func() { s.app.emit("workspace", s.tree()) })
	return nil
}

// RevealWorkspace shows the workspace's folder in the file manager.
func (s *WorkspaceService) RevealWorkspace() error {
	folder := s.app.workspaceFolder()
	if folder == "" {
		return errNoWorkspaceFile
	}
	if s.app.pickers.Reveal == nil {
		return errNoReveal
	}
	if err := s.app.pickers.Reveal(folder); err != nil {
		return fmt.Errorf("showing the workspace folder: %w", err)
	}
	return nil
}

func (a *App) workspaceFolder() string {
	if a.wsFile == nil {
		return ""
	}
	return a.wsFile.Folder()
}
