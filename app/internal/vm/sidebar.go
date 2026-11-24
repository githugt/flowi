// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"

	"github.com/githugt/flowi/lib/model"
)

// MaxRecent is how many recently opened requests and flows the sidebar's Recent tab keeps.
const MaxRecent = 15

// SidebarFile is what the sidebar keeps between sessions, in the session folder (so it is the person's own and this
// machine's, not the collection's: starring a collection does not touch the files a team shares).
type SidebarFile struct {
	// Favourites are the collections the person starred, by collection id (which survives a rename or a move of the
	// folder), in the order they were starred.
	Favourites []model.ID `json:"favourites"`
	// Recent are the tree rows of the requests and flows opened last, newest first.
	Recent []string `json:"recent"`
}

// SaveSidebar stores the sidebar's favourites and recent list.
func (s *SessionStore) SaveSidebar(f SidebarFile) error { return s.write("sidebar.json", f) }

// LoadSidebar reads the stored sidebar; a missing file is an empty one, a damaged one an error and an empty one.
func (s *SessionStore) LoadSidebar() (SidebarFile, error) {
	var out SidebarFile
	return out, s.read("sidebar.json", &out)
}

// RecentRow is a recently opened request or flow, as the Recent tab lists it.
type RecentRow struct {
	ID       string
	Label    string
	Kind     NodeKind
	Method   string
	Type     string
	TypeName string
	Where    string
}

// Sidebar is what the sidebar remembers: the starred collections and the recent list. It follows the workspace's rows,
// so a row that is gone (a deleted request, a collection that was unmounted) drops out of what it shows. It changes
// on the UI goroutine only.
type Sidebar struct {
	ws   *Workspace
	file SidebarFile
}

// NewSidebar returns the memory over ws, starting from what was stored.
func NewSidebar(ws *Workspace, stored SidebarFile) *Sidebar {
	return &Sidebar{ws: ws, file: SidebarFile{Favourites: slices.Clone(stored.Favourites), Recent: slices.Clone(stored.Recent)}}
}

// File is what to store, copied so a writer on another goroutine does not share it.
func (s *Sidebar) File() SidebarFile {
	return SidebarFile{Favourites: slices.Clone(s.file.Favourites), Recent: slices.Clone(s.file.Recent)}
}

// ToggleFavourite stars the collection of tree row `node` or takes the star off, and reports whether the list changed.
// It does nothing (false) for a row that is not a collection, and for Drafts: that is a place to try things in, not a
// collection to keep at hand.
func (s *Sidebar) ToggleFavourite(node string) bool {
	n := s.ws.Node(node)
	if n.Kind != NodeCollection || n.Scratch {
		return false
	}
	if i := slices.Index(s.file.Favourites, n.Collection); i >= 0 {
		s.file.Favourites = slices.Delete(s.file.Favourites, i, i+1)
	} else {
		s.file.Favourites = append(s.file.Favourites, n.Collection)
	}
	return true
}

// Favourites are the tree rows of the starred collections that exist, in the order they were starred.
func (s *Sidebar) Favourites() []string {
	out := []string{}
	for _, id := range s.file.Favourites {
		if _, ok := s.ws.Collection(id); ok {
			out = append(out, "c:"+string(id))
		}
	}
	return out
}

// NoteOpened puts a request or flow at the front of the recent list, and reports whether the list changed. Another
// kind of row is not remembered: Recent is for what you work on, not for screens.
func (s *Sidebar) NoteOpened(node string) bool {
	switch s.ws.Node(node).Kind {
	case NodeRequest, NodeFlow:
	default:
		return false
	}
	if len(s.file.Recent) > 0 && s.file.Recent[0] == node {
		return false
	}
	s.file.Recent = slices.DeleteFunc(s.file.Recent, func(id string) bool { return id == node })
	s.file.Recent = append([]string{node}, s.file.Recent...)
	if len(s.file.Recent) > MaxRecent {
		s.file.Recent = s.file.Recent[:MaxRecent]
	}
	return true
}

// ClearRecent empties the recent list.
func (s *Sidebar) ClearRecent() { s.file.Recent = nil }

// Recent lists the recent rows that still exist, newest first.
func (s *Sidebar) Recent() []RecentRow {
	out := []RecentRow{}
	for _, id := range s.file.Recent {
		n, ok := s.ws.snap.nodes[id]
		if !ok {
			continue
		}
		out = append(out, RecentRow{ID: id, Label: s.ws.Display(id), Kind: n.Kind, Method: n.Method, Type: n.Type, TypeName: n.TypeName, Where: n.Where})
	}
	return out
}
