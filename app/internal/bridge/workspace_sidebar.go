// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strings"
	"sync"

	"github.com/githugt/flowi/lib/diag"

	"github.com/githugt/flowi/app/internal/vm"
)

// RecentRow is a recently opened request or flow, for the sidebar's Recent tab.
type RecentRow struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Method   string `json:"method,omitempty"`
	Type     string `json:"type,omitempty"`
	TypeName string `json:"typeName,omitempty"`
	Where    string `json:"where,omitempty"`
}

// SidebarView is what the sidebar remembers for this person: the starred collections (their tree rows) and the
// recently opened requests and flows. The "sidebar" event carries it whenever it changes.
type SidebarView struct {
	Favourites []string    `json:"favourites"`
	Recent     []RecentRow `json:"recent"`
}

// HoverFact is one labelled line of a hover card.
type HoverFact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// HoverProblem is a validation problem shown on a hover card, with its code and what to do about it.
type HoverProblem struct {
	Severity string `json:"severity"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

// HoverView is the card the sidebar shows when the pointer rests on a row.
type HoverView struct {
	Title        string         `json:"title"`
	Kind         string         `json:"kind"`
	Verb         string         `json:"verb,omitempty"`
	Where        string         `json:"where,omitempty"`
	TargetLabel  string         `json:"targetLabel,omitempty"`
	Target       string         `json:"target,omitempty"`
	Description  string         `json:"description,omitempty"`
	Facts        []HoverFact    `json:"facts"`
	Problems     []HoverProblem `json:"problems"`
	MoreProblems int            `json:"moreProblems,omitempty"`
}

// sidebarMemory is the starred collections and the recent list of one window, and how they are kept on disk.
type sidebarMemory struct {
	once   sync.Once
	vm     *vm.Sidebar
	loaded vm.SidebarFile
	err    error
	store  *vm.SessionStore
	writer *vm.SessionWriter
}

// sidebarNotice is the notice that tells the person the sidebar's memory could not be read or kept: the code, what it
// says, and what to do.
func sidebarNotice(code diag.Code, what string, err error) Notice {
	d := diag.New(code, diag.Location{}, map[string]string{"what": what, "detail": err.Error()})
	return Notice{Level: "warn", Text: string(code) + ": " + diag.Text(d)}
}

// loadSidebar reads what the sidebar stored, once, off the UI goroutine (it blocks on the disk). A damaged file is
// reported as a notice and the sidebar starts empty.
func (s *WorkspaceService) loadSidebar() {
	m := &s.memory
	m.once.Do(func() {
		m.store = vm.NewSessionStore(s.app.backend.SessionDir())
		m.writer = vm.NewSessionWriter(func(err error) {
			s.app.emit("notice", sidebarNotice(diag.CodeSidebarSave, "favourites and recent list", err))
		})
		m.loaded, m.err = m.store.LoadSidebar()
		if m.err != nil {
			s.app.emit("notice", sidebarNotice(diag.CodeSidebarRead, "favourites and recent list", m.err))
		}
	})
}

// sidebar is the sidebar's memory; call it on the actor. Before it was loaded (tests, or a window not started yet) it
// starts empty.
func (s *WorkspaceService) sidebar() *vm.Sidebar {
	if s.memory.vm == nil {
		s.memory.vm = vm.NewSidebar(s.app.workspace, s.memory.loaded)
	}
	return s.memory.vm
}

// keepRecent says whether the recent list is kept between sessions: it follows "ui.keep_history", the setting for what
// the app remembers of your work. Starred collections are kept either way.
func (s *WorkspaceService) keepRecent() bool {
	_, keep := s.app.session.prefs()
	return keep
}

// sidebarView builds the view; call it on the actor.
func (s *WorkspaceService) sidebarView() SidebarView {
	sb := s.sidebar()
	view := SidebarView{Favourites: sb.Favourites(), Recent: []RecentRow{}}
	for _, r := range sb.Recent() {
		view.Recent = append(view.Recent, RecentRow{ID: r.ID, Label: r.Label, Kind: kindNames()[r.Kind], Method: r.Method, Type: r.Type, TypeName: r.TypeName, Where: r.Where})
	}
	return view
}

// remember keeps the sidebar's memory on disk and tells the window; call it on the actor after a change.
func (s *WorkspaceService) remember() {
	file := s.sidebar().File()
	if !s.keepRecent() {
		file.Recent = nil
	}
	if m := &s.memory; m.writer != nil {
		m.writer.Queue("sidebar", func() error { return m.store.SaveSidebar(file) })
	}
	s.app.emit("sidebar", s.sidebarView())
}

// Sidebar is what the sidebar remembers now: the starred collections and the recent requests and flows.
func (s *WorkspaceService) Sidebar() SidebarView {
	s.loadSidebar()
	var v SidebarView
	s.app.actor.Do(func() { v = s.sidebarView() })
	return v
}

// ToggleFavourite stars the collection of tree row `node`, or takes its star off.
func (s *WorkspaceService) ToggleFavourite(node string) {
	s.app.actor.Do(func() {
		if s.sidebar().ToggleFavourite(node) {
			s.remember()
		}
	})
}

// NoteOpened records that the window opened request or flow row `node`, for the Recent tab.
func (s *WorkspaceService) NoteOpened(node string) {
	s.app.actor.Do(func() {
		if s.sidebar().NoteOpened(node) {
			s.remember()
		}
	})
}

// ClearRecent empties the Recent tab.
func (s *WorkspaceService) ClearRecent() {
	s.app.actor.Do(func() {
		s.sidebar().ClearRecent()
		s.remember()
	})
}

// Hover is the card for tree row `node`: what it is, where it lives, what it talks to, what it says about itself and
// what is wrong with it, so a person can tell rows apart without opening them.
func (s *WorkspaceService) Hover(node string) HoverView {
	var card vm.HoverCard
	var unresolved []vm.Problem // an environment's: reading the vault is done off the actor
	if strings.HasPrefix(node, "e:") {
		for _, st := range s.app.envService.States() {
			if "e:"+st.File == node {
				for _, i := range st.Issues {
					unresolved = append(unresolved, vm.Problem{Severity: "warning", Brief: i.Brief, Hint: i.Hint})
				}
			}
		}
	}
	s.app.actor.Do(func() {
		ws := s.app.workspace
		problems := unresolved
		if file := ws.Node(node).File; file != "" {
			problems = append(problems, s.app.problems.ForFile(file)...)
		}
		card = ws.Hover(node, problems)
	})
	out := HoverView{Title: card.Title, Kind: card.Kind, Verb: card.Verb, Where: card.Where, TargetLabel: card.TargetLabel, Target: card.Target,
		Description: card.Description, MoreProblems: card.MoreProblems, Facts: []HoverFact{}, Problems: []HoverProblem{}}
	for _, f := range card.Facts {
		out.Facts = append(out.Facts, HoverFact{Label: f.Label, Value: f.Value})
	}
	for _, p := range card.Problems {
		out.Problems = append(out.Problems, HoverProblem{Severity: p.Severity, Code: p.Code, Message: p.Message, Hint: p.Hint})
	}
	return out
}
