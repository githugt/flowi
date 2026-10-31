// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"log/slog"

	"github.com/githugt/flowi/app/internal/vm"
)

// SessionService keeps what carries over between sessions: the open tabs,
// and, when ui.keep_history is on, the run log and the response history.
type SessionService struct {
	app    *App
	store  *vm.SessionStore
	writer *vm.SessionWriter
	logger *slog.Logger
}

// newSessionService builds the service for the session. logger gets what could not be kept.
func newSessionService(a *App, logger *slog.Logger) *SessionService {
	s := &SessionService{app: a, store: vm.NewSessionStore(a.backend.SessionDir()), logger: logger}
	s.writer = vm.NewSessionWriter(func(err error) {
		logger.Warn("the session could not be kept", "Event", "SessionSaveProblem", "Error", err)
	})
	return s
}

// Restore brings back the last session's run log and response history (when
// ui.keep_history is on) and returns the tab rows to open (when
// ui.restore_tabs is on). Call it once the workspace tree has loaded.
func (s *SessionService) Restore() []string {
	restoreTabs, keepHistory := s.prefs()
	var tabs []string
	if restoreTabs {
		var err error
		if tabs, err = s.store.LoadTabs(); err != nil {
			s.logger.Warn("the last session's tabs could not be restored", "Event", "SessionRestoreProblem", "Error", err)
		}
	}
	if keepHistory {
		s.restoreHistory()
	} else if err := s.store.DropHistory(); err != nil {
		s.logger.Warn("the stored history could not be removed", "Event", "SessionRestoreProblem", "Error", err)
	}
	return nonNil(tabs)
}

func (s *SessionService) restoreHistory() {
	lines, lerr := s.store.LoadLog()
	items, herr := s.store.LoadHistory()
	for _, err := range []error{lerr, herr} {
		if err != nil {
			s.logger.Warn("the last session's history could not be restored", "Event", "SessionRestoreProblem", "Error", err)
		}
	}
	s.app.actor.Do(func() {
		s.app.log.Restore(lines)
		s.app.history.Restore(items)
		s.app.history.OnAdd(func() {
			// Copied here: the writer runs on its own goroutine.
			saved, exchanges := append([]string(nil), s.app.log.Lines()...), s.app.history.Export()
			s.writer.Queue("history", func() error {
				if err := s.store.SaveLog(saved); err != nil {
					return err
				}
				return s.store.SaveHistory(exchanges)
			})
		})
	})
}

// SaveTabs stores the rows of the open tabs.
func (s *SessionService) SaveTabs(rows []string) {
	s.writer.Queue("tabs", func() error { return s.store.SaveTabs(rows) })
}

func (s *SessionService) prefs() (restoreTabs, keepHistory bool) {
	r := s.app.backend.Settings("")
	if v, ok := r.Get("ui.restore_tabs"); ok {
		restoreTabs, _ = v.Value.(bool)
	}
	if v, ok := r.Get("ui.keep_history"); ok {
		keepHistory, _ = v.Value.(bool)
	}
	return restoreTabs, keepHistory
}
