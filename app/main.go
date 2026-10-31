// SPDX-License-Identifier: AGPL-3.0-or-later

// Command flowi is the web-view front end of Flowi.
package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"sync/atomic"
	"syscall"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/vault"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"

	"github.com/githugt/flowi/internal/backend"
	"github.com/githugt/flowi/internal/bridge"
	"github.com/githugt/flowi/internal/buildinfo"
	"github.com/githugt/flowi/internal/uithread"
)

//go:embed all:frontend/dist
var assets embed.FS

// appIcon is the window icon on Linux. macOS and Windows take theirs from the app bundle and the executable.
//
//go:embed build/appicon.png
var appIcon []byte

// changelog is what the welcome screen shows the latest entry of.
//
//go:embed CHANGELOG.md
var changelog string

func main() {
	// WebKitGTK kills the program with a signal when its helpers are missing; say what to install instead.
	if !systemReady() {
		os.Exit(1)
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var quitting atomic.Bool // the user confirmed quitting with unsaved edits
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wails *application.App
	var app *bridge.App
	var mayQuit func() bool
	// The vault asks for its passphrase only when a secret is first used,
	// by which time app is set and the window shows.
	ask := func() (string, error) { return app.Passphrase().Ask() }
	ws, openProblems, err := openWorkspace(os.Environ(), userHome(), ask)
	if err != nil {
		return err
	}
	if len(openProblems) > 0 {
		slog.Warn("the workspace opened with problems", "Event", "WorkspaceOpenProblems", "Count", len(openProblems))
	}
	emit := func(name string, data any) { wails.Event.Emit(name, data) }
	app = bridge.NewApp(backend.New(ws, uithread.NewGuard(false)), bridge.NewActor(ctx), emit,
		bridge.WithLogger(slog.Default()), bridge.WithChangelog(changelog), bridge.WithOpenProblems(openProblems), bridge.WithWorkspaceFile(ws), bridge.WithSpecOverlays(ws), bridge.WithCollectionVariables(ws))

	wails = application.New(application.Options{
		Name:        "Flowi",
		Description: "Flowi",
		OnShutdown:  app.Shutdown,
		// Cmd+Q and the Quit menu item do not close the window first, so the
		// guard sits here too.
		ShouldQuit: func() bool { return mayQuit() },
		Services: []application.Service{
			application.NewService(app.Workspace()),
			application.NewService(app.Flows()),
			application.NewService(app.Requests()),
			application.NewService(app.Environments()),
			application.NewService(app.Panels()),
			application.NewService(app.Settings()),
			application.NewService(app.Secrets()),
			application.NewService(app.Manager()),
			application.NewService(app.System()),
			application.NewService(app.Import()),
			application.NewService(app.Schedules()),
			application.NewService(app.Replayer()),
			application.NewService(app.Blueprints()),
			application.NewService(app.Trash()),
			application.NewService(app.Checks()),
			application.NewService(app.Hooks()),
			application.NewService(app.Session()),
			application.NewService(app.Passphrase()),
			application.NewService(app.Welcome()),
		},
		Assets: application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Mac:    application.MacOptions{ApplicationShouldTerminateAfterLastWindowClosed: true},
	})
	wails.Menu.Set(buildMenu(wails, emit))
	app.SetPickers(bridge.Pickers{
		Folder: func(title string) (string, error) {
			return wails.Dialog.OpenFile().SetTitle(title).CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).PromptForSingleSelection()
		},
		File: func(title string) (string, error) {
			return wails.Dialog.OpenFile().SetTitle(title).PromptForSingleSelection()
		},
		Save: func(title, name string) (string, error) {
			return wails.Dialog.SaveFile().SetMessage(title).SetFilename(name).CanCreateDirectories(true).PromptForSingleSelection()
		},
		Reveal: func(folder string) error { return wails.Env.OpenFileManager(folder, false) },
	})
	window := wails.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:  "Flowi",
		Width:  1360,
		Height: 860,
		// The page says where the window can be dragged from (--wails-draggable: the sidebar's
		// head and the tab bar, but not its tabs). An InvisibleTitleBarHeight was dropped: AppKit
		// dragged the window from anywhere in that strip, tabs included, before the page saw the
		// press, so a tab could not be dragged to a new place.
		Mac: application.MacWindow{
			TitleBar: application.MacTitleBarHiddenInset,
		},
		// On Linux the menus sit behind a button in the title bar instead of taking a row of their own.
		Linux:            application.LinuxWindow{MenuStyle: application.LinuxMenuStylePrimaryMenu, Icon: appIcon},
		BackgroundColour: application.NewRGB(15, 17, 22),
		URL:              "/",
	})

	if runtime.GOOS == "windows" {
		hideMenuBarUntilAlt(wails, window)
	}

	app.SetQuit(func() {
		quitting.Store(true)
		wails.Quit()
	})
	// Quitting keeps what a request's edit slot can hold and asks only about
	// the rest; the window answers through System.Quit.
	mayQuit = func() bool {
		if quitting.Load() || !app.KeepEdits() {
			return true
		}
		emit("quit-requested", map[string]int{"environments": app.DirtyEnvironments()})
		return false
	}
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		if !mayQuit() {
			e.Cancel()
		}
	})
	app.Start(ctx)
	return wails.Run()
}

// userHome is the folder "~" stands for in mount paths: HOME on Unix, USERPROFILE
// on Windows. Without one, "~" paths stay unexpanded.
func userHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		slog.Warn("the home folder is unknown; ~ in paths will not expand", "Event", "HomeUnknown", "Error", err)
		return ""
	}
	return home
}

// openWorkspace opens the user's workspace; FLOWI_CONFIG_DIR points it at
// another one. The problems it returns are what the files got wrong: the
// window lists them in the Problems panel. passphrase asks for the vault's
// passphrase where no OS keychain holds its key.
func openWorkspace(environ []string, home string, passphrase func() (string, error)) (*flowi.Workspace, diag.List, error) {
	paths, err := settings.DiscoverPaths(environ)
	if err != nil {
		return nil, nil, fmt.Errorf("finding the config folder: %w", err)
	}
	ws, problems, err := flowi.OpenWorkspace(context.Background(), flowi.WorkspaceOptions{
		Paths: paths, Environ: environ, Home: home, Keys: vault.OSKeyStore(), Passphrase: passphrase,
		UserAgent: "Flowi-UI/" + buildinfo.Version(),
	})
	return ws, problems, err
}
