// SPDX-License-Identifier: AGPL-3.0-or-later

//go:build server

// Command uidev serves Flowi's front end in a plain browser over a made-up workspace (internal/uidev): no web view,
// no real workspace, no network, no vault. Build the front end first (`just uidev` does), then open the address it
// prints. Nothing is saved: edits live in memory until the process ends.
//
// It is a development tool. It listens on localhost only, and its sends and runs answer with canned results.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/uidev"
)

func main() {
	dist := flag.String("dist", "frontend/dist", "the built front end (npm run build in frontend/)")
	port := flag.Int("port", 5173, "the port to listen on, on localhost")
	flag.Parse()
	if _, err := os.Stat(*dist + "/index.html"); err != nil {
		log.Fatalf("no built front end in %s: run `npm run build` in frontend/ first (or `just uidev`)", *dist)
	}
	if err := run(*dist, *port); err != nil {
		log.Fatal(err)
	}
}

func run(dist string, port int) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wails *application.App
	emit := func(name string, data any) { wails.Event.Emit(name, data) }
	app := bridge.NewApp(uidev.New(), bridge.NewActor(ctx), emit)
	wails = application.New(application.Options{
		Name:        "Flowi UI development",
		Description: "Flowi front end over a made-up workspace",
		Services:    services(app),
		Assets:      application.AssetOptions{Handler: application.AssetFileServerFS(os.DirFS(dist))},
		Server:      application.ServerOptions{Host: "localhost", Port: port},
	})
	wails.Window.NewWithOptions(application.WebviewWindowOptions{Title: "Flowi", URL: "/"})
	app.SetQuit(func() { wails.Quit() })
	app.Start(ctx)
	fmt.Printf("Flowi UI development: http://localhost:%d (a made-up workspace; nothing is saved)\n", port)
	return wails.Run()
}

// services is every bridge service the real app (main.go) registers: a screen that calls one that is missing here
// would fail only in this server. A test keeps the two lists the same.
func services(app *bridge.App) []application.Service {
	return []application.Service{
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
		application.NewService(app.Auths()),
		application.NewService(app.Trash()),
		application.NewService(app.Checks()),
		application.NewService(app.Hooks()),
		application.NewService(app.Session()),
		application.NewService(app.Passphrase()),
		application.NewService(app.Welcome()),
		application.NewService(app.Tracing()),
	}
}
