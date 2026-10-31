// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"

	"github.com/githugt/flowi/internal/vm"
)

// LogLine is a line of the run log.
type LogLine struct {
	At      string `json:"at"`
	Level   string `json:"level"` // INFO, WARN, ERROR
	Step    string `json:"step"`
	Message string `json:"message"`
	Tone    string `json:"tone"` // none, pending, success, redirect, client, server, cancelled
}

// LogSection is the lines of one send.
type LogSection struct {
	Title string    `json:"title"`
	Lines []LogLine `json:"lines"`
}

// ProblemRow is a row of the Problems panel.
type ProblemRow struct {
	Severity string `json:"severity"` // error or warning
	Code     string `json:"code"`
	Message  string `json:"message"`
	Where    string `json:"where"`
	Node     string `json:"node,omitempty"` // the row to open on click
}

// HistoryItem is an exchange of this session.
type HistoryItem struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	At    string `json:"at"`
	Node  string `json:"node"` // the request it belongs to
}

// Panels is the state of the footer panels: run log, problems, history.
type Panels struct {
	Log      []LogSection  `json:"log"`
	Problems []ProblemRow  `json:"problems"`
	Summary  string        `json:"summary"`
	History  []HistoryItem `json:"history"`
	// Opening says what went wrong when the workspace opened ("" when
	// nothing did); the rows are among Problems.
	Opening string `json:"opening"`
}

func toneNames() map[vm.Tone]string {
	return map[vm.Tone]string{
		vm.ToneNone: "none", vm.TonePending: "pending", vm.ToneSuccess: "success", vm.ToneRedirect: "redirect",
		vm.ToneClientError: "client", vm.ToneServerError: "server", vm.ToneCancelled: "cancelled",
	}
}

// PanelsService is the run log, the problems and the response history.
type PanelsService struct{ app *App }

// newPanelsService builds the service for the footer panels.
func newPanelsService(a *App) *PanelsService {
	s := &PanelsService{app: a}
	// The events carry the whole panel state: the panels are small, and a
	// run log of 5000 lines is cut to the newest sections.
	changed := func() { a.emit("panels", s.state()) }
	a.log.OnChange(changed)
	a.history.OnChange(changed)
	a.problems.OnChange(changed)
	return s
}

// State is what the footer panels show now.
func (s *PanelsService) State() Panels {
	var p Panels
	s.app.actor.Do(func() { p = s.state() })
	return p
}

// RefreshProblems validates the workspace again.
func (s *PanelsService) RefreshProblems() { s.app.actor.Do(s.app.problems.Refresh) }

// ClearLog empties the run log.
func (s *PanelsService) ClearLog() { s.app.actor.Do(s.app.log.Clear) }

// ClearHistory forgets the responses of this session.
func (s *PanelsService) ClearHistory() { s.app.actor.Do(s.app.history.Clear) }

// DeleteHistory forgets one response of this session.
func (s *PanelsService) DeleteHistory(id int) { s.app.actor.Do(func() { s.app.history.Remove(id) }) }

// ClearProblems forgets what sends reported. Problems found in the files stay
// until the files are fixed.
func (s *PanelsService) ClearProblems() { s.app.actor.Do(s.app.problems.ClearRun) }

const maxLogSections = 30

func (s *PanelsService) state() Panels {
	p := Panels{Log: []LogSection{}, Problems: []ProblemRow{}, History: []HistoryItem{}}
	sections := s.app.log.Sections()
	if len(sections) > maxLogSections {
		sections = sections[len(sections)-maxLogSections:]
	}
	for _, sec := range sections {
		out := LogSection{Title: sec.Title, Lines: []LogLine{}}
		for _, e := range sec.Entries {
			at := ""
			if !e.At.IsZero() {
				at = e.At.Format(time.TimeOnly)
			}
			out.Lines = append(out.Lines, LogLine{At: at, Level: e.Level, Step: e.Step, Message: e.Message, Tone: toneNames()[e.Tone]})
		}
		p.Log = append(p.Log, out)
	}
	for _, it := range s.app.problems.Items() {
		p.Problems = append(p.Problems, ProblemRow{Severity: it.Severity, Code: it.Code, Message: it.Message, Where: it.Where, Node: it.Node})
	}
	p.Summary = s.app.problems.Summary()
	p.Opening = openingNotice(s.app.openProblems)
	for _, ex := range s.app.history.Items() {
		p.History = append(p.History, HistoryItem{ID: ex.ID, Title: ex.Title, At: ex.At.Format(time.TimeOnly), Node: requestNode(ex.Target)})
	}
	return p
}

// openingNotice tells how many problems opening the workspace found.
func openingNotice(l diag.List) string {
	switch n := len(l); n {
	case 0:
		return ""
	case 1:
		return "1 problem was found while opening the workspace."
	default:
		return fmt.Sprintf("%d problems were found while opening the workspace.", n)
	}
}

func requestNode(t flowi.RequestTarget) string {
	return "r:" + string(t.Collection) + ":" + string(t.Request)
}
