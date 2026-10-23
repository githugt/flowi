// SPDX-License-Identifier: Apache-2.0

// Command flowi-demo-server serves the demo API that the user-creation
// example collection talks to.
//
//	go run ./examples/demoserver/cmd/flowi-demo-server -addr 127.0.0.1:8080 -password demo
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/githugt/flowi-lib/examples/demoserver"
)

const (
	shutdownTimeout   = 5 * time.Second
	readHeaderTimeout = 10 * time.Second
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8080", "address to listen on")
	password := flag.String("password", "demo", "password the login endpoints accept")
	flag.Parse()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(*addr, *password, logger); err != nil {
		logger.Error("demo server stopped", "Event", "DemoServerFailed", "Error", err)
		os.Exit(1)
	}
}

// run serves until SIGINT/SIGTERM, then shuts down within
// shutdownTimeout; connections still open after that are dropped.
func run(addr, password string, logger *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Addr: addr, Handler: demoserver.Handler(demoserver.Options{Password: password}), ReadHeaderTimeout: readHeaderTimeout}
	errs := make(chan error, 1)
	go func() { errs <- srv.ListenAndServe() }()
	logger.Info("demo API listening", "Event", "DemoServerStarted", "Addr", "http://"+addr)
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	logger.Info("demo API stopped", "Event", "DemoServerStopped")
	return nil
}
