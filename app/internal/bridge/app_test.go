// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

func TestApp_should_hand_out_one_instance_of_each_service(t *testing.T) {
	t.Parallel()
	services := map[string]func(*bridge.App) any{
		"Workspace":    func(a *bridge.App) any { return a.Workspace() },
		"Flows":        func(a *bridge.App) any { return a.Flows() },
		"Requests":     func(a *bridge.App) any { return a.Requests() },
		"Environments": func(a *bridge.App) any { return a.Environments() },
		"Panels":       func(a *bridge.App) any { return a.Panels() },
		"Settings":     func(a *bridge.App) any { return a.Settings() },
		"Secrets":      func(a *bridge.App) any { return a.Secrets() },
		"Manager":      func(a *bridge.App) any { return a.Manager() },
		"System":       func(a *bridge.App) any { return a.System() },
		"Import":       func(a *bridge.App) any { return a.Import() },
		"Schedules":    func(a *bridge.App) any { return a.Schedules() },
		"Replayer":     func(a *bridge.App) any { return a.Replayer() },
		"Blueprints":   func(a *bridge.App) any { return a.Blueprints() },
		"Trash":        func(a *bridge.App) any { return a.Trash() },
		"Checks":       func(a *bridge.App) any { return a.Checks() },
		"Hooks":        func(a *bridge.App) any { return a.Hooks() },
		"Session":      func(a *bridge.App) any { return a.Session() },
		"Welcome":      func(a *bridge.App) any { return a.Welcome() },
		"Passphrase":   func(a *bridge.App) any { return a.Passphrase() },
	}
	for name, service := range services {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			app, _ := fixture(t)

			first, second := service(app), service(app)

			if first == nil || first != second {
				t.Fatalf("%s gave %p then %p", name, first, second)
			}
		})
	}
}

func TestWorkspaceService_Menu_should_leave_out_actions_the_window_cannot_run(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		backend func() *vmtest.Backend
		row     string // the kind of root row whose menu is asked for
		absent  []string
	}{
		{
			name: "draft collection: no move to folder or discard",
			backend: func() *vmtest.Backend {
				c := vmtest.SampleCollection()
				return &vmtest.Backend{Cols: []model.Collection{c}, DraftIDs: map[model.ID]bool{c.Manifest.ID: true}}
			},
			row:    "collection",
			absent: []string{"moveToFolder", "discardDraft"},
		},
		{
			name: "missing collection: no manage",
			backend: func() *vmtest.Backend {
				return &vmtest.Backend{MissingIDs: []model.ID{"C9"}, Paths: map[model.ID]string{"C9": "/gone"}}
			},
			row:    "missing",
			absent: []string{"manage"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			app := bridge.NewApp(tt.backend(), bridge.NewInlineActor(), (&published{}).emit)
			app.Start(t.Context())
			row, ok := findNode(app.Workspace().Tree().Roots, tt.row)
			if !ok {
				t.Fatalf("no %s row", tt.row)
			}

			menu := app.Workspace().Menu([]string{row.ID})

			for _, e := range menu {
				if e.Action == "" || slices.Contains(tt.absent, e.Action) {
					t.Fatalf("menu offers %q (%q), which the window cannot run", e.Action, e.Label)
				}
			}
		})
	}
}

func TestPanelsService_State_should_report_what_opening_the_workspace_found(t *testing.T) {
	t.Parallel()
	problem := func(s diag.Severity) diag.Diagnostic {
		d := diag.New(diag.CodeFileUnreadable, diag.Location{File: "/c/flowi.yaml"}, map[string]string{"name": "/c/flowi.yaml", "detail": "x"})
		d.Severity = s
		return d
	}
	tests := []struct {
		name string
		open diag.List
		want string
	}{
		{name: "nothing found", open: nil, want: ""},
		{name: "one warning", open: diag.List{problem(diag.SeverityWarning)}, want: "1 problem was found while opening the workspace."},
		{name: "one error", open: diag.List{problem(diag.SeverityError)}, want: "1 problem was found while opening the workspace."},
		{
			name: "an error and a warning",
			open: diag.List{problem(diag.SeverityError), problem(diag.SeverityWarning)},
			want: "2 problems were found while opening the workspace.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
			app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit, bridge.WithOpenProblems(tt.open))

			got := app.Panels().State().Opening

			if got != tt.want {
				t.Fatalf("opening notice %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApp_Start_should_fill_the_problems_panel_without_a_refresh(t *testing.T) {
	t.Parallel()
	broken := diag.New(diag.CodeFileUnreadable, diag.Location{File: "/c/flowi.yaml"}, map[string]string{"name": "/c/flowi.yaml", "detail": "x"})
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}, Diags: diag.List{broken}}
	app := bridge.NewApp(b, bridge.NewInlineActor(), (&published{}).emit)

	app.Start(t.Context())

	if got := app.Panels().State().Problems; len(got) != 1 || got[0].Code != string(diag.CodeFileUnreadable) {
		t.Fatalf("problems %+v", got)
	}
}

func TestPassphraseService_Ask_should_wait_for_the_windows_answer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		respond func(s *bridge.PassphraseService)
		want    string
		wantErr error
	}{
		{name: "answered", respond: func(s *bridge.PassphraseService) { s.Answer("correct horse") }, want: "correct horse"},
		{name: "cancelled", respond: (*bridge.PassphraseService).Cancel, wantErr: bridge.ErrPassphraseCancelled},
		{name: "window closed", respond: (*bridge.PassphraseService).Close, wantErr: bridge.ErrPassphraseClosed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			svc, asked := passphraseService()
			type result struct {
				phrase string
				err    error
			}
			done := make(chan result, 1)

			go func() {
				phrase, err := svc.Ask()
				done <- result{phrase, err}
			}()
			<-asked
			tt.respond(svc)
			got := <-done

			if got.phrase != tt.want || !errors.Is(got.err, tt.wantErr) {
				t.Fatalf("got (%q, %v), want (%q, %v)", got.phrase, got.err, tt.want, tt.wantErr)
			}
		})
	}
}

func TestPassphraseService_Answer_Given_no_open_question_Then_the_next_question_waits(t *testing.T) {
	t.Parallel()
	svc, asked := passphraseService()
	done := make(chan string, 1)

	svc.Answer("stale") // nobody asked: dropped
	go func() {
		phrase, _ := svc.Ask()
		done <- phrase
	}()
	<-asked
	svc.Answer("fresh")

	if got := <-done; got != "fresh" {
		t.Fatalf("got %q, want the answer given after the question", got)
	}
}

// passphraseService builds the service with an emit that reports each
// "passphrase" question on the returned channel.
func passphraseService() (*bridge.PassphraseService, <-chan struct{}) {
	asked := make(chan struct{}, 1)
	emit := func(name string, _ any) {
		if name == "passphrase" {
			select {
			case asked <- struct{}{}:
			default: // a repeat of a question the test already saw
			}
		}
	}
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	return bridge.NewApp(b, bridge.NewInlineActor(), emit).Passphrase(), asked
}

func TestPassphraseService_Given_several_waiting_When_cancelled_Then_all_get_the_one_answer(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var asks atomic.Int32
		emit := func(name string, _ any) {
			if name == "passphrase" {
				asks.Add(1)
			}
		}
		b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
		svc := bridge.NewApp(b, bridge.NewInlineActor(), emit).Passphrase()
		errs := make(chan error, 3)

		for range 3 {
			go func() {
				_, err := svc.Ask()
				errs <- err
			}()
		}
		synctest.Wait() // all three are waiting on the window
		svc.Cancel()

		for range 3 {
			if err := <-errs; !errors.Is(err, bridge.ErrPassphraseCancelled) {
				t.Fatalf("got %v, want every waiting caller cancelled", err)
			}
		}
		if n := asks.Load(); n != 1 {
			t.Fatalf("the window was asked %d times, want once", n)
		}
	})
}

// A question published while the window was not listening (a reload, a
// reconnecting event socket) was lost, and the vault waited for good: the
// question is now published again until it is answered.
func TestPassphraseService_Given_no_answer_Then_the_question_is_published_again(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		var asks atomic.Int32
		emit := func(name string, _ any) {
			if name == "passphrase" {
				asks.Add(1)
			}
		}
		b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
		svc := bridge.NewApp(b, bridge.NewInlineActor(), emit).Passphrase()
		done := make(chan struct{})

		go func() {
			_, _ = svc.Ask() // the answer is not what this test is about
			close(done)
		}()
		time.Sleep(5 * time.Second) // the simulated clock: the first ask and two reminders
		svc.Answer("x")
		<-done
		time.Sleep(5 * time.Second) // answered: no more reminders

		if n := asks.Load(); n != 3 {
			t.Fatalf("the window was asked %d times, want 3", n)
		}
	})
}

func TestPassphraseService_Pending_should_say_whether_a_question_waits(t *testing.T) {
	t.Parallel()
	svc, asked := passphraseService()
	before := svc.Pending()
	done := make(chan struct{})

	go func() {
		_, _ = svc.Ask() // the answer is not what this test is about
		close(done)
	}()
	<-asked
	during := svc.Pending()
	svc.Answer("x")
	<-done
	after := svc.Pending()

	if before || !during || after {
		t.Fatalf("pending before %v, during %v, after %v; want false, true, false", before, during, after)
	}
}

func TestPassphraseService_Given_a_cancel_Then_later_asks_fail_at_once_until_Unlock(t *testing.T) {
	t.Parallel()
	svc, asked := passphraseService()
	cancelled := make(chan struct{})
	go func() {
		_, _ = svc.Ask() // cancelled below
		close(cancelled)
	}()
	<-asked
	svc.Cancel()
	<-cancelled
	answered := make(chan string, 1)

	_, whileDeclined := svc.Ask() // must not block: nobody would answer
	svc.Unlock()
	go func() {
		phrase, _ := svc.Ask()
		answered <- phrase
	}()
	<-asked
	svc.Answer("again")
	afterUnlock := <-answered

	if !errors.Is(whileDeclined, bridge.ErrPassphraseCancelled) || afterUnlock != "again" {
		t.Fatalf("while declined %v, after Unlock answered %q", whileDeclined, afterUnlock)
	}
}
