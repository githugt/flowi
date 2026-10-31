// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestRename_should_save_the_name_and_survive_reopening(t *testing.T) {
	tests := []struct {
		name  string
		given string
		want  string
	}{
		{name: "a plain name", given: "Payments", want: "Payments"},
		{name: "spaces around it are dropped", given: "  Team API  ", want: "Team API"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			w := e.open(t)

			err := w.Rename(tt.given)

			if err != nil {
				t.Fatal(err)
			}
			if got := e.open(t).Model().Name; got != tt.want {
				t.Fatalf("name after reopening %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRename_Given_an_empty_name_Then_refused_and_the_name_kept(t *testing.T) {
	e := newEnv(t)
	w := e.open(t)
	before := w.Model().Name

	err := w.Rename("   ")

	if !errors.Is(err, ErrWorkspaceName) {
		t.Fatalf("err %v, want ErrWorkspaceName", err)
	}
	if got := e.open(t).Model().Name; got != before {
		t.Fatalf("name %q, want %q kept", got, before)
	}
}

func TestFolder_should_be_where_workspace_yaml_lives(t *testing.T) {
	e := newEnv(t)

	got := e.open(t).Folder()

	if want := filepath.Dir(workspaceFile(e)); got != want {
		t.Fatalf("folder %q, want %q", got, want)
	}
}
