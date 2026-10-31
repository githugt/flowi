// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
	"github.com/githugt/flowi/lib/model"
)

// fakeWorkspaceFile renames the fake backend, as the real file renames the
// workspace the backend reads its name from.
type fakeWorkspaceFile struct {
	b      *vmtest.Backend
	folder string
	err    error
}

func (f *fakeWorkspaceFile) Rename(name string) error {
	if f.err != nil {
		return f.err
	}
	f.b.Name = strings.TrimSpace(name)
	return nil
}

func (f *fakeWorkspaceFile) Folder() string { return f.folder }

func newWorkspaceApp(t *testing.T, file *fakeWorkspaceFile, reveal func(string) error) (*bridge.App, *published) {
	t.Helper()
	b := &vmtest.Backend{Cols: []model.Collection{vmtest.SampleCollection()}}
	pub := &published{}
	var opts []bridge.Option
	if file != nil {
		file.b = b
		opts = append(opts, bridge.WithWorkspaceFile(file))
	}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit, opts...)
	app.SetPickers(bridge.Pickers{Reveal: reveal})
	app.Start(t.Context())
	return app, pub
}

func TestWorkspaceService_RenameWorkspace_should_publish_the_new_name(t *testing.T) {
	app, pub := newWorkspaceApp(t, &fakeWorkspaceFile{folder: "/ws"}, nil)

	err := app.Workspace().RenameWorkspace("Payments")

	if err != nil {
		t.Fatal(err)
	}
	tree, ok := pub.last["workspace"].(bridge.Tree)
	if !ok || tree.Name != "Payments" || tree.Folder != "/ws" {
		t.Fatalf("published tree %+v, want Payments in /ws", pub.last["workspace"])
	}
}

func TestWorkspaceService_RenameWorkspace_Given_a_failure_Then_an_error(t *testing.T) {
	refused := errors.New("refused")
	tests := []struct {
		name string
		file *fakeWorkspaceFile
		want error
	}{
		{name: "the file refuses", file: &fakeWorkspaceFile{err: refused}, want: refused},
		{name: "no file", file: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := newWorkspaceApp(t, tt.file, nil)

			err := app.Workspace().RenameWorkspace("x")

			if err == nil || (tt.want != nil && !errors.Is(err, tt.want)) {
				t.Fatalf("err %v, want one wrapping %v", err, tt.want)
			}
		})
	}
}

func TestWorkspaceService_RevealWorkspace_should_show_the_folder(t *testing.T) {
	var shown []string
	reveal := func(dir string) error { shown = append(shown, dir); return nil }
	tests := []struct {
		name    string
		file    *fakeWorkspaceFile
		reveal  func(string) error
		wantErr bool
		want    []string
	}{
		{name: "a folder and a file manager", file: &fakeWorkspaceFile{folder: "/ws"}, reveal: reveal, want: []string{"/ws"}},
		{name: "no file manager", file: &fakeWorkspaceFile{folder: "/ws"}, wantErr: true},
		{name: "no workspace file", reveal: reveal, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shown = nil
			app, _ := newWorkspaceApp(t, tt.file, tt.reveal)

			err := app.Workspace().RevealWorkspace()

			if (err != nil) != tt.wantErr || strings.Join(shown, ",") != strings.Join(tt.want, ",") {
				t.Fatalf("err %v shown %v, want error %v shown %v", err, shown, tt.wantErr, tt.want)
			}
		})
	}
}
