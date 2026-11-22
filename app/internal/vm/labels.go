// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"path"
	"slices"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
)

// unknownStatusIcon is what a status that names nothing of the collection is drawn with, so a typo in a file is seen in
// the tree and not shown as no status.
const unknownStatusIcon = "alert"

// withLabels puts the resolved status and the tags onto a tree node.
func withLabels(n *TreeNode, c model.Collection, at model.StatusOf, tags []string) {
	n.Tags = tags
	if at.Name == "" {
		return
	}
	n.StatusFrom = at.From
	if def, ok := c.StatusNamed(at.Name); ok {
		n.Status = def
		return
	}
	n.Status = model.Status{Name: at.Name, Color: "gray", Icon: unknownStatusIcon}
	n.StatusUnknown = true
}

// Labels is what a Tags and status editor shows for one request, flow or folder.
type Labels struct {
	Node string
	Kind NodeKind
	// Tags and Status are the item's own, as written in its file. Status is "" when it sets none.
	Tags   []string
	Status string
	// Inherited is the status that applies when Status is "": the nearest folder's, and the folder it is from.
	Inherited     model.Status
	InheritedFrom string
	// FolderTags are the tags the folders above add; they are shown, and cannot be removed here.
	FolderTags []string
	// Statuses are the choices of the collection, and Suggest the tags it already uses, most used first.
	Statuses []model.Status
	Suggest  []model.TagUse
	// Collection names the collection, for adding a status to it.
	Collection model.ID
}

// ErrNoLabels is what Labels and SetLabels report for a row that carries neither tags nor a status.
var ErrNoLabels = errors.New("only a request, a flow or a folder has tags and a status")

// Labels reads the tags and status of the row nodeID from the loaded collection.
func (ws *Workspace) Labels(nodeID string) (Labels, error) {
	n := ws.Node(nodeID)
	c, ok := ws.Collection(n.Collection)
	if !ok {
		return Labels{}, ErrNoLabels
	}
	out := Labels{Node: nodeID, Kind: n.Kind, Statuses: c.Manifest.StatusList(), Suggest: c.TagsInUse(), Collection: n.Collection}
	var dir string
	switch n.Kind {
	case NodeRequest:
		i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == n.Request })
		if i < 0 {
			return Labels{}, ErrNoLabels
		}
		r := c.Requests[i]
		out.Tags, out.Status, dir = r.Tags, r.Status, path.Join("requests", path.Dir(r.Path))
	case NodeFlow:
		i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.ID == n.Flow })
		if i < 0 {
			return Labels{}, ErrNoLabels
		}
		f := c.Flows[i]
		out.Tags, out.Status, dir = f.Tags, f.Status, path.Join("flows", path.Dir(f.Path))
	case NodeFolder:
		rel := n.Tree + "/" + n.Path
		j := slices.IndexFunc(c.Folders, func(f model.Folder) bool { return f.Dir == rel })
		if j >= 0 {
			out.Tags, out.Status = c.Folders[j].Tags, c.Folders[j].Status
		}
		dir = path.Dir(rel)
	default:
		return Labels{}, ErrNoLabels
	}
	out.FolderTags = folderTagsAbove(c, dir, out.Tags)
	if out.Status == "" {
		out.Inherited, out.InheritedFrom = inheritedStatus(c, dir)
	}
	return out, nil
}

// folderTagsAbove are the tags the folders from dir upward give, without the item's own.
func folderTagsAbove(c model.Collection, dir string, own []string) []string {
	var out []string
	for _, t := range c.FolderTags(dir) {
		if !slices.Contains(own, t) {
			out = append(out, t)
		}
	}
	return out
}

// inheritedStatus is the status an item with none of its own gets from the folder dir or above, and where from.
func inheritedStatus(c model.Collection, dir string) (model.Status, string) {
	at := c.FolderStatus(dir)
	if at.Name == "" {
		return model.Status{}, ""
	}
	from := at.From
	if from == "" {
		from = dir
	}
	if def, ok := c.StatusNamed(at.Name); ok {
		return def, from
	}
	return model.Status{Name: at.Name, Color: "gray", Icon: unknownStatusIcon}, from
}

// SetLabels sets the tags and status of the row nodeID in its file, then shows the tree again. status "" leaves the
// item to inherit. done is called on the UI goroutine with nil, or with the error: FLW-WRITE-001 (a file with comments,
// which a call with dropComments allows) among them.
func (ws *Workspace) SetLabels(nodeID string, tags []string, status string, dropComments bool, done func(error)) {
	n := ws.Node(nodeID)
	ws.create(func() (string, error) {
		switch n.Kind {
		case NodeRequest:
			return "", ws.b.SetRequestLabels(flowi.RequestTarget{Collection: n.Collection, Request: n.Request}, tags, status, dropComments)
		case NodeFlow:
			return "", ws.b.SetFlowLabels(flowi.FlowTarget{Collection: n.Collection, Flow: n.Flow}, tags, status, dropComments)
		case NodeFolder:
			return "", ws.b.SetFolderLabels(n.Collection, n.Tree, n.Path, tags, status, dropComments)
		}
		return "", ErrNoLabels
	}, func(string) { done(nil) }, done)
}

// AddStatus adds a status to the collection of the row nodeID (or changes the colour and icon of one of that name).
func (ws *Workspace) AddStatus(nodeID string, s model.Status, dropComments bool, done func(error)) {
	n := ws.Node(nodeID)
	c, ok := ws.Collection(n.Collection) // read here: the build below runs off the UI goroutine
	list := slices.Clone(c.Manifest.Statuses)
	if i := slices.IndexFunc(list, func(o model.Status) bool { return o.Name == s.Name }); i >= 0 {
		list[i] = s
	} else {
		list = append(list, s)
	}
	ws.create(func() (string, error) {
		if !ok {
			return "", ErrNoLabels
		}
		return "", ws.b.SaveStatuses(n.Collection, list, dropComments)
	}, func(string) { done(nil) }, done)
}
