// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"runtime"
	"strings"

	"github.com/githugt/flowi/lib/changelog"
	"github.com/githugt/flowi/lib/telemetry"

	"github.com/githugt/flowi/app/internal/buildinfo"
	"github.com/githugt/flowi/app/internal/vm"
)

// AnnouncementView is a notice from the project's feed.
type AnnouncementView struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
	URL   string `json:"url"`
	Level string `json:"level"` // info or warning
}

// WelcomeView is the welcome screen and the notices over it.
type WelcomeView struct {
	Version       string             `json:"version"`
	WhatsNew      string             `json:"whatsNew"` // Markdown: the newest release of the changelog
	Upgraded      string             `json:"upgraded"` // Markdown: releases since the last one seen, "" when none
	Announcements []AnnouncementView `json:"announcements"`
}

// TelemetryView is the usage-reporting choice.
type TelemetryView struct {
	Explanation string            `json:"explanation"`
	Level       string            `json:"level"` // off, minimal or full
	Problem     string            `json:"problem,omitempty"`
	Previews    map[string]string `json:"previews"` // the report each level would send
}

// WelcomeService serves the welcome screen, the announcements and the
// telemetry choice.
type WelcomeService struct {
	app       *App
	news      *vm.Announcements
	telemetry *vm.Telemetry
	firstTele *firstLoad
	seen      *vm.SessionStore
}

// newWelcomeService builds the service for the welcome screen.
func newWelcomeService(a *App) *WelcomeService {
	client := telemetry.Client{App: "gui", Version: buildinfo.Version(), OS: runtime.GOOS, Arch: runtime.GOARCH}
	s := &WelcomeService{app: a, news: vm.NewAnnouncements(a.backend, a.worker), telemetry: vm.NewTelemetry(a.backend, a.worker, client), firstTele: newFirstLoad(), seen: vm.NewSessionStore(a.backend.SessionDir())}
	s.news.OnChange(func() { a.emit("welcome", s.view()) })
	s.news.OnError(func(err error) { a.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	s.telemetry.OnChange(func() {
		s.firstTele.mark()
		a.emit("telemetry", s.telemetryView())
	})
	return s
}

// State is the welcome screen. The announcements load in the background and
// arrive as the "welcome" event.
func (s *WelcomeService) State() WelcomeView {
	var v WelcomeView
	s.app.actor.Do(func() {
		s.news.Load()
		v = s.view()
	})
	return v
}

// Dismiss remembers that an announcement was seen; it is not shown again.
func (s *WelcomeService) Dismiss(id string) { s.app.actor.Do(func() { s.news.Dismiss(id) }) }

// DismissUpgrade remembers that the release notes of this build were read; the Home strip goes away.
func (s *WelcomeService) DismissUpgrade() {
	s.app.actor.Do(func() {
		if err := s.seen.MarkVersionSeen(buildinfo.Version()); err != nil {
			s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)})
			return
		}
		s.app.emit("welcome", s.view())
	})
}

// upgraded is the notes for an upgrade: the releases since the last version that showed its notes. The first
// launch has none (the welcome page has its own), and records the version so the next upgrade has a starting point.
func (s *WelcomeService) upgraded() string {
	current := buildinfo.Version()
	last, err := s.seen.SeenVersion()
	if err != nil {
		return "" // a damaged marker is not evidence of an upgrade
	}
	if last == "" {
		_ = s.seen.MarkVersionSeen(current) // best effort: without it there is just no note next time either
		return ""
	}
	if last == current {
		return ""
	}
	var b strings.Builder
	for _, r := range changelog.Upgrade(changelog.Parse(s.app.changelog), last) {
		b.WriteString(r.Markdown())
		b.WriteString("\n")
	}
	return b.String()
}

// TurnOffAnnouncements switches announcements off.
func (s *WelcomeService) TurnOffAnnouncements() { s.app.actor.Do(s.news.TurnOff) }

// Telemetry loads the current choice and returns it.
func (s *WelcomeService) Telemetry() TelemetryView {
	var v TelemetryView
	s.app.actor.Do(s.telemetry.Load)
	s.firstTele.wait()
	s.app.actor.Do(func() { v = s.telemetryView() })
	return v
}

// ChooseTelemetry stores the choice: off, minimal or full.
func (s *WelcomeService) ChooseTelemetry(level string) {
	s.app.actor.Do(func() { s.telemetry.Choose(level) })
}

func (s *WelcomeService) view() WelcomeView {
	v := WelcomeView{Version: buildinfo.Version(), Announcements: []AnnouncementView{}}
	if latest := changelog.Latest(changelog.Parse(s.app.changelog), 1); len(latest) > 0 {
		v.WhatsNew = latest[0].Markdown()
	}
	v.Upgraded = s.upgraded()
	for _, it := range s.news.Items() {
		v.Announcements = append(v.Announcements, AnnouncementView{ID: it.ID, Title: it.Title, Body: it.Body, URL: it.URL, Level: it.Level})
	}
	return v
}

func (s *WelcomeService) telemetryView() TelemetryView {
	v := TelemetryView{Explanation: s.telemetry.Explanation(), Level: s.telemetry.Level(), Problem: s.telemetry.Problem(), Previews: map[string]string{}}
	for _, l := range []string{"off", "minimal", "full"} {
		v.Previews[l] = s.telemetry.Preview(l)
	}
	return v
}
