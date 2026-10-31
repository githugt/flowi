// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"strconv"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// TrashItemView is a request or flow in the trash.
type TrashItemView struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // request or flow
	Name string `json:"name"`
	Path string `json:"path"` // where it was in the Drafts
	At   string `json:"at"`   // when it was moved, local time
}

// TrashView is the trash screen.
type TrashView struct {
	Items []TrashItemView `json:"items"`
	After string          `json:"after"` // how long a draft may sit unused before it is moved here, "" when never
	Error string          `json:"error,omitempty"`
}

// TrashService is where old, unused try-out requests and flows of the Drafts go,
// until the user restores them or deletes them for good.
type TrashService struct{ app *App }

// State lists what the trash holds.
func (s *TrashService) State() TrashView { return s.view("") }

func (s *TrashService) view(problem string) TrashView {
	v := TrashView{Items: []TrashItemView{}, After: afterText(s.after()), Error: problem}
	list, err := s.app.backend.ListTrash()
	if err != nil && v.Error == "" {
		v.Error = vm.ErrorText(err)
	}
	for _, it := range list {
		v.Items = append(v.Items, TrashItemView{ID: it.ID, Kind: it.Kind, Name: it.Name, Path: it.Path, At: it.TrashedAt.Local().Format("2006-01-02 15:04")})
	}
	return v
}

// after is how long a draft may sit unused: ui.drafts_trash_after.
func (s *TrashService) after() time.Duration {
	if v, ok := s.app.backend.Settings("").Get("ui.drafts_trash_after"); ok {
		d, _ := v.Value.(time.Duration)
		return d
	}
	return 0
}

func afterText(d time.Duration) string {
	switch {
	case d <= 0:
		return ""
	case d == 24*time.Hour:
		return "1 day"
	case d%(24*time.Hour) == 0:
		return strconv.Itoa(int(d/(24*time.Hour))) + " days"
	}
	return d.String()
}

// Sweep moves the try-out requests and flows of the Drafts that were not changed for
// ui.drafts_trash_after to the trash, except those open in a tab (open lists the tree
// rows of the tabs). It returns how many it moved; the tree and the trash update.
func (s *TrashService) Sweep(open []string) int {
	inUse := map[string]bool{}
	for _, row := range open {
		parts := strings.Split(row, ":")
		if len(parts) == 3 {
			inUse[parts[0]+":"+parts[2]] = true
		}
	}
	moved, err := s.app.backend.TrashStaleDrafts(s.after(), func(kind string, id model.ID) bool {
		prefix := map[string]string{flowi.TrashRequest: "r", flowi.TrashFlow: "f"}[kind]
		return inUse[prefix+":"+string(id)]
	})
	if err != nil {
		s.app.emit("trash", s.view(vm.ErrorText(err)))
		return 0
	}
	if len(moved) > 0 {
		s.app.actor.Do(s.app.workspace.Load)
		s.app.emit("trash", s.view(""))
	}
	return len(moved)
}

// Restore puts an item back into the Drafts and returns the path it got.
func (s *TrashService) Restore(id string) (string, error) {
	path, err := s.app.backend.RestoreFromTrash(id)
	s.changed()
	return path, err
}

// Delete removes one item for good.
func (s *TrashService) Delete(id string) string {
	return s.act(s.app.backend.DeleteFromTrash(id))
}

// Empty removes everything in the trash for good.
func (s *TrashService) Empty() string {
	return s.act(s.app.backend.EmptyTrash())
}

func (s *TrashService) act(err error) string {
	s.changed()
	if err != nil {
		return vm.ErrorText(err)
	}
	return ""
}

// changed shows the trash and the tree again.
func (s *TrashService) changed() {
	s.app.actor.Do(s.app.workspace.Load)
	s.app.emit("trash", s.view(""))
}
