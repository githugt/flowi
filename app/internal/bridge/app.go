// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"context"
	"log/slog"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"

	"github.com/githugt/flowi/app/internal/vm"
)

// Emit publishes an event to the front end.
type Emit func(name string, data any)

// App is the view-models behind the window, owned by one Actor, and the
// services the window calls. Every service is built once, in NewApp: the
// window registers them from the main goroutine while the actor already
// reaches them, so none may be created on first use.
type App struct {
	actor     *Actor
	backend   vm.Backend
	worker    *vm.Worker
	workspace *vm.Workspace
	send      *vm.Send
	log       *vm.RunLog
	history   *vm.History
	problems  *vm.Problems
	scheduler vm.Scheduler // autosave timers
	pickers   Pickers
	wsFile    WorkspaceFile       // nil when the app was built without one (tests)
	specs     SpecOverlays        // nil when the app was built without them (tests)
	variables CollectionVariables // nil when the app was built without them (tests)
	quit      func()
	treeReady *firstLoad // the workspace tree has loaded once
	emit      Emit
	logger    *slog.Logger // for what the app cannot report in the window
	changelog string       // CHANGELOG.md as it was when the app was built, for the welcome screen
	// openProblems is what opening the workspace reported, for the
	// Problems panel's notice; set once, in NewApp.
	openProblems diag.List

	tree       *WorkspaceService
	flows      *FlowService
	requests   *RequestService
	envService *EnvironmentService
	panels     *PanelsService
	settings   *SettingsService
	secrets    *SecretsService
	manager    *ManagerService
	system     *SystemService
	importer   *ImportService
	schedules  *SchedulesService
	replayer   *ReplayerService
	blueprints *BlueprintsService
	trash      *TrashService
	checks     *ChecksService
	hooks      *HooksService
	session    *SessionService
	welcome    *WelcomeService
	passphrase *PassphraseService
}

// Option changes how NewApp builds the app.
type Option func(*appOptions)

type appOptions struct {
	logger       *slog.Logger
	openProblems diag.List
	wsFile       WorkspaceFile
	specs        SpecOverlays
	variables    CollectionVariables
	changelog    string
}

// WithLogger sets the logger for what the app cannot report in the window
// (a session that could not be kept). The default is slog.Default().
func WithLogger(l *slog.Logger) Option {
	return func(o *appOptions) { o.logger = l }
}

// WithChangelog gives the text of CHANGELOG.md, which the welcome screen shows the latest entry of.
func WithChangelog(text string) Option {
	return func(o *appOptions) { o.changelog = text }
}

// WithOpenProblems passes on what flowi.OpenWorkspace reported. The Problems
// panel then says so, and lists them, as soon as the window shows.
func WithOpenProblems(l diag.List) Option {
	return func(o *appOptions) { o.openProblems = l }
}

// NewApp builds the app over a backend. Call Start to load it.
func NewApp(b vm.Backend, actor *Actor, emit Emit, opts ...Option) *App {
	o := appOptions{logger: slog.Default()}
	for _, opt := range opts {
		opt(&o)
	}
	w := vm.NewWorker(actor.Post)
	if actor.inline {
		w = vm.NewInlineWorker()
	}
	ws := vm.NewWorkspace(b, w)
	log, history := vm.NewRunLog(), vm.NewHistory()
	a := &App{actor: actor, backend: b, worker: w, workspace: ws, emit: emit, log: log, history: history,
		send: vm.NewSend(b, w, ws, log, history), problems: vm.NewProblems(b, w, ws), openProblems: o.openProblems, logger: o.logger, changelog: o.changelog, wsFile: o.wsFile, specs: o.specs, variables: o.variables}
	a.send.OnResult(a.problems.SetRun)
	a.send.OnPrompt(a.promptPerson)
	a.scheduler = vm.NewTimerScheduler(actor.Post)
	a.treeReady = newFirstLoad()
	a.buildServices(o.logger)
	return a
}

// buildServices creates every service once. Several subscribe to view-model
// changes, so building one twice would publish its events twice.
func (a *App) buildServices(logger *slog.Logger) {
	a.tree = &WorkspaceService{app: a}
	a.flows = &FlowService{app: a, docs: map[string]*openFlow{}}
	a.requests = newRequestService(a)
	a.envService = &EnvironmentService{app: a, envs: map[string]*vm.Environment{}}
	a.panels = newPanelsService(a)
	a.settings = newSettingsService(a)
	a.secrets = newSecretsService(a)
	a.manager = newManagerService(a)
	a.system = &SystemService{app: a}
	a.importer = newImportService(a)
	a.schedules = newSchedulesService(a)
	a.replayer = newReplayerService(a)
	a.blueprints = newBlueprintsService(a)
	a.trash = &TrashService{app: a}
	a.checks = &ChecksService{app: a}
	a.hooks = newHooksService(a)
	a.session = newSessionService(a, logger)
	a.welcome = newWelcomeService(a)
	a.passphrase = newPassphraseService(a)
}

// Start loads the workspace and watches its files until ctx ends. Every
// change publishes the "workspace" event with the new Tree. The Problems
// panel is filled at once, so what is wrong shows without asking.
func (a *App) Start(ctx context.Context) {
	a.actor.Do(func() {
		a.workspace.OnChange(func() {
			a.treeReady.mark()
			a.emit("workspace", a.tree.tree())
		})
		a.workspace.OnLabelChange(func() { a.emit("workspace", a.tree.tree()) })
		a.workspace.Load()
		a.workspace.Watch(ctx)
		a.problems.Refresh()
	})
}

// Workspace is the service for the workspace tree and its context menu.
func (a *App) Workspace() *WorkspaceService { return a.tree }

// Flows is the service for open flow documents.
func (a *App) Flows() *FlowService { return a.flows }

// Requests is the service for open requests.
func (a *App) Requests() *RequestService { return a.requests }

// Environments is the service for open environments.
func (a *App) Environments() *EnvironmentService { return a.envService }

// Panels is the service for the footer panels.
func (a *App) Panels() *PanelsService { return a.panels }

// Settings is the service for the settings screen.
func (a *App) Settings() *SettingsService { return a.settings }

// Secrets is the service for the secrets screen.
func (a *App) Secrets() *SecretsService { return a.secrets }

// Manager is the service for the collections manager.
func (a *App) Manager() *ManagerService { return a.manager }

// System is the service for the operating system's dialogs.
func (a *App) System() *SystemService { return a.system }

// Import is the service for the import wizard.
func (a *App) Import() *ImportService { return a.importer }

// Schedules is the service for the schedules screen.
func (a *App) Schedules() *SchedulesService { return a.schedules }

// Replayer is the service for the replayer screen.
func (a *App) Replayer() *ReplayerService { return a.replayer }

// Blueprints is the service for the blueprints library.
func (a *App) Blueprints() *BlueprintsService { return a.blueprints }

// Trash is the service for the trash.
func (a *App) Trash() *TrashService { return a.trash }

// Checks is the service for the Checks screen.
func (a *App) Checks() *ChecksService { return a.checks }

// Hooks is the service for the hook editor.
func (a *App) Hooks() *HooksService { return a.hooks }

// Session is the service for what carries over between sessions.
func (a *App) Session() *SessionService { return a.session }

// Welcome is the service for the welcome screen and usage reporting.
func (a *App) Welcome() *WelcomeService { return a.welcome }

// Passphrase is the service for the vault's passphrase question.
func (a *App) Passphrase() *PassphraseService { return a.passphrase }

// newRequestService builds the service for open requests.
func newRequestService(a *App) *RequestService {
	s := &RequestService{app: a, reqs: map[string]*openRequest{}}
	// A send starts and ends outside the request's own changes: tell every open request.
	a.send.OnChange(func() {
		for node, o := range s.reqs {
			a.emit("request", s.view(node, o))
		}
	})
	return s
}

// Shutdown stops what the app started (the replayer) and finishes the
// session writes. Call it once, off the actor.
func (a *App) Shutdown() {
	a.passphrase.Close()
	a.replayer.Close()
	a.schedules.Close()
	if err := a.importer.Close(); err != nil {
		a.logger.Warn("the pasted text was not removed", "Event", "PasteFileLeft", "Error", err)
	}
	a.session.writer.Wait()
}

// SetQuit installs what ends the application.
func (a *App) SetQuit(quit func()) { a.quit = quit }

// Dirty reports whether any open document has unsaved edits. Call it off the actor.
func (a *App) Dirty() bool {
	dirty := false
	a.actor.Do(func() {
		for _, o := range a.flows.docs {
			dirty = dirty || o.doc.Dirty()
		}
		for _, o := range a.requests.reqs {
			dirty = dirty || o.req.Dirty()
		}
		for _, e := range a.envService.envs {
			dirty = dirty || e.Dirty()
		}
	})
	return dirty
}

// keepEditsTimeout bounds the wait for edit slots to be written on quit.
const keepEditsTimeout = 5 * time.Second

// KeepEdits is what runs before the app quits. A request's unsaved edits are
// written to its edit slot (the autosave would do so only ten seconds after
// the last change, which a quit does not wait for) and come back when the
// tab is opened again. It reports whether edits remain that would be lost:
// flows and environments have no slot, and a slot can refuse a change (a new
// version). Call it off the actor.
func (a *App) KeepEdits() (lost bool) {
	pending := a.flushRequests()
	a.actor.Do(func() {
		for _, o := range a.flows.docs {
			lost = lost || o.doc.Dirty()
		}
		for _, e := range a.envService.envs {
			lost = lost || e.Dirty()
		}
	})
	return waitKept(pending, keepEditsTimeout) || lost
}

// KeepFlowEdits stores the unsaved edits of every open flow, which have no edit
// slot, for the next start (FlowService.Open brings them back). It returns the
// first failure; the quit then stays cancelled. Call it off the actor.
func (a *App) KeepFlowEdits() (err error) {
	a.actor.Do(func() {
		for _, o := range a.flows.docs {
			if err = o.doc.KeepEdits(); err != nil {
				return
			}
		}
	})
	return err
}

// DirtyEnvironments counts the environments with unsaved edits: they cannot be
// kept for the next start, because their secrets must not be written aside.
// Call it off the actor.
func (a *App) DirtyEnvironments() (n int) {
	a.actor.Do(func() {
		for _, e := range a.envService.envs {
			if e.Dirty() {
				n++
			}
		}
	})
	return n
}

// waitKept reports whether any edit may be lost: one whose slot refused it,
// or one not answered within timeout. timeout bounds the whole wait, not
// each edit's, so a quit never waits longer than it however many tabs are open.
func waitKept(pending []chan bool, timeout time.Duration) (lost bool) {
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for _, p := range pending {
		select {
		case kept := <-p:
			lost = lost || !kept
		case <-deadline.C:
			return true
		}
	}
	return lost
}

// flushRequests starts storing every open request's edit; each channel
// answers whether that request's edit is safe.
func (a *App) flushRequests() []chan bool {
	var pending []chan bool
	a.actor.Do(func() {
		for _, o := range a.requests.reqs {
			done := make(chan bool, 1)
			pending = append(pending, done)
			o.req.FlushResult(func(stored bool) { done <- stored })
		}
	})
	return pending
}

// PromptView is what a request waits for the person to do: the "authprompt" event.
type PromptView struct {
	Kind    string `json:"kind"` // "open": sign in at URL; "device": open URL and enter Code
	URL     string `json:"url"`
	Code    string `json:"code,omitempty"`
	Expires string `json:"expires,omitempty"` // RFC 3339
}

// promptPerson publishes what a waiting request asks of the person, so the window can open the page or show the code.
func (a *App) promptPerson(p engine.AuthPrompted) {
	a.emit("authprompt", PromptView{Kind: p.Kind, URL: p.URL, Code: p.Code, Expires: p.Expires.Format(time.RFC3339)})
}
