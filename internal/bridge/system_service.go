// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"

	"github.com/githugt/flowi/internal/vm"
)

// Pickers open the operating system's file dialogs. They are set by the
// shell; a headless build has none.
type Pickers struct {
	// Folder asks for a folder; "" means the user cancelled.
	Folder func(title string) (string, error)
	// File asks for a file; "" means the user cancelled.
	File func(title string) (string, error)
	// Reveal shows a folder in the system's file manager (Finder).
	Reveal func(folder string) error
}

// SystemService is what the window needs from the operating system.
type SystemService struct{ app *App }

// SetPickers installs the file dialogs.
func (a *App) SetPickers(p Pickers) { a.pickers = p }

var errNoDialogs = errors.New("file dialogs are not available here: type the path")

// PickFolder asks for a folder. It returns "" when cancelled.
func (s *SystemService) PickFolder(title string) (string, error) {
	if s.app.pickers.Folder == nil {
		return "", errNoDialogs
	}
	return s.app.pickers.Folder(title)
}

// PickFile asks for a file. It returns "" when cancelled.
func (s *SystemService) PickFile(title string) (string, error) {
	if s.app.pickers.File == nil {
		return "", errNoDialogs
	}
	return s.app.pickers.File(title)
}

// QuitKeeping stores what can be kept of the unsaved edits (the flows'), then quits.
// When that fails the app stays open and the error says why.
func (s *SystemService) QuitKeeping() error {
	if err := s.app.KeepFlowEdits(); err != nil {
		return fmt.Errorf("keeping the unsaved changes: %w", err)
	}
	s.Quit()
	return nil
}

// Quit ends the application. The window asks first when documents have
// unsaved edits (the "quit-requested" event); this is its answer.
func (s *SystemService) Quit() {
	if s.app.quit != nil {
		s.app.quit()
	}
}

// HelpItemView is one name, function or operator of the expression help.
type HelpItemView struct {
	Name    string `json:"name"`
	What    string `json:"what"`
	Example string `json:"example"`
}

// ExprHelpView is the pop-up that says what an expression field can read.
type ExprHelpView struct {
	Title      string         `json:"title"`
	Intro      string         `json:"intro"`
	Names      []HelpItemView `json:"names"`
	Tools      []HelpItemView `json:"tools"`
	Generators []HelpItemView `json:"generators"`
	Examples   []string       `json:"examples"`
}

// ExpressionHelp says what can be written in the expression field of the
// given context (see vm.ExprContext: flow.when, request.check …).
func (s *SystemService) ExpressionHelp(context string) ExprHelpView {
	h := vm.ExpressionHelp(vm.ExprContext(context))
	return ExprHelpView{
		Title: h.Title, Intro: h.Intro, Names: helpItems(h.Names), Tools: helpItems(h.Tools),
		Generators: helpItems(vm.GeneratorHelp()), Examples: nonNil(h.Examples),
	}
}

func helpItems(in []vm.HelpItem) []HelpItemView {
	out := make([]HelpItemView, 0, len(in))
	for _, i := range in {
		out = append(out, HelpItemView{Name: i.Name, What: i.What, Example: i.Example})
	}
	return out
}
