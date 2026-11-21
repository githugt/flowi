// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/examples"
	"github.com/githugt/flowi/lib/examples/demoserver"
	"github.com/githugt/flowi/lib/model"
)

// ExampleEnvironment is the name of the environment the example runs in. It is a file in the workspace's
// environments folder like any other, so the person can see and edit it; its base_url is rewritten each time the
// example is opened, because the demo server takes a free port.
const ExampleEnvironment = "demo"

// The demo password is public (it is in the demo server's source), so the example keeps it as a plain variable of
// the environment, not as a vault secret: opening the example must not create a vault or ask for a passphrase.
const (
	examplePassword  = "demo"
	exampleSecretRef = "{{secret.password}}"
	examplePlainRef  = "{{env.password}}"
)

// Example is the example collection opened for trying Flowi without an API of one's own.
type Example struct {
	Collection model.ID
	// Env is the environment to select to run it (ExampleEnvironment).
	Env string
	// ServerURL is where its demo API listens, on the loopback address.
	ServerURL string
}

// exampleState is the example this workspace opened: its collection, folder and demo server.
type exampleState struct {
	info   Example
	dir    string
	server *http.Server
}

// OpenExample opens the example collection (a user signs up, verifies and completes onboarding) as a temporary
// collection, starts its demo API on a free loopback port in this process, and makes the environment "demo" that
// points at it, so the first request can be sent at once. Nothing is mounted into the workspace file: the
// collection and the server last until CloseExample or the end of the process. Opening it again returns the one
// already open.
func (w *Workspace) OpenExample() (Example, error) {
	w.exampleMu.Lock()
	defer w.exampleMu.Unlock()
	if w.example != nil && w.hasCollection(w.example.info.Collection) {
		return w.example.info, nil
	}
	url, server, err := startDemoServer()
	if err != nil {
		return Example{}, err
	}
	dir, err := w.writeExample()
	if err != nil {
		stopDemoServer(server)
		return Example{}, err
	}
	if err := w.setExampleEnvironment(url); err != nil {
		stopDemoServer(server)
		_ = os.RemoveAll(dir)
		return Example{}, err
	}
	c, _, err := w.store.Load(dir)
	if err != nil {
		stopDemoServer(server)
		_ = os.RemoveAll(dir)
		return Example{}, fmt.Errorf("loading the example: %w", err)
	}
	c.Manifest.Name += " (example)"
	w.mu.Lock()
	w.collections = append(w.collections, w.withGlobalEnvironments(c))
	w.mu.Unlock()
	w.example = &exampleState{info: Example{Collection: c.Manifest.ID, Env: ExampleEnvironment, ServerURL: url}, dir: dir, server: server}
	return w.example.info, nil
}

// CloseExample removes the example collection and stops its demo server. It does nothing when the example is not
// open. The "demo" environment stays.
func (w *Workspace) CloseExample() {
	w.exampleMu.Lock()
	defer w.exampleMu.Unlock()
	if w.example == nil {
		return
	}
	w.mu.Lock()
	id := w.example.info.Collection
	w.collections = slices.DeleteFunc(w.collections, func(c model.Collection) bool { return c.Manifest.ID == id })
	w.mu.Unlock()
	stopDemoServer(w.example.server)
	_ = os.RemoveAll(w.example.dir)
	w.example = nil
}

func (w *Workspace) hasCollection(id model.ID) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, ok := w.collection(id)
	return ok
}

// writeExample copies the embedded example into a new temporary folder, with the password read from the
// environment rather than the vault (see examplePassword).
func (w *Workspace) writeExample() (string, error) {
	dir, err := os.MkdirTemp("", "flowi-example-")
	if err != nil {
		return "", fmt.Errorf("making a folder for the example: %w", err)
	}
	src := examples.UserCreation()
	err = fs.WalkDir(src, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		target := filepath.Join(dir, filepath.FromSlash(name))
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		if strings.HasPrefix(name, "environments/") {
			return nil // the workspace's environments are global; the example's own two point at other servers
		}
		return os.WriteFile(target, []byte(strings.ReplaceAll(string(data), exampleSecretRef, examplePlainRef)), 0o644)
	})
	if err != nil {
		_ = os.RemoveAll(dir)
		return "", fmt.Errorf("copying the example: %w", err)
	}
	return dir, nil
}

// setExampleEnvironment makes (or updates) the global environment "demo" for the demo server at url.
func (w *Workspace) setExampleEnvironment(url string) error {
	find := func() (model.Environment, bool) {
		i := slices.IndexFunc(w.Environments(), func(e model.Environment) bool { return e.Name == ExampleEnvironment })
		if i < 0 {
			return model.Environment{}, false
		}
		return w.Environments()[i], true
	}
	if _, ok := find(); !ok {
		if _, err := w.NewEnvironment(ExampleEnvironment); err != nil {
			return err
		}
	}
	env, ok := find()
	if !ok {
		return errors.New("the demo environment was not created")
	}
	env.Vars = []model.EnvVar{
		{Name: "base_url", Value: url},
		{Name: "password", Value: examplePassword},
		{Name: "verify_wait", Value: "10ms"},
	}
	return w.SaveEnvironment(env, SaveOptions{})
}

// startDemoServer serves the demo API on a free loopback port.
func startDemoServer() (string, *http.Server, error) {
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, fmt.Errorf("starting the demo server: %w", err)
	}
	srv := &http.Server{Handler: demoserver.Handler(demoserver.Options{Password: examplePassword}), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }() // ends with Shutdown; its error is the listener closing
	return "http://" + ln.Addr().String(), srv, nil
}

func stopDemoServer(srv *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if srv.Shutdown(ctx) != nil {
		_ = srv.Close()
	}
}
