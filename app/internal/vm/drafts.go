// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"fmt"
	"strings"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
)

// Drafts are collections kept in Flowi's own directory until the user
// saves them to a folder (flowi-lib drafts_api.go). Requests and flows made
// without choosing a collection go to the "Scratch" draft.

// NewScratchRequest creates the next "untitled" request in Scratch, then
// calls open with its tree node id once the tree shows it. fail, if set,
// gets any error.
func (ws *Workspace) NewScratchRequest(open func(nodeID string), fail func(error)) {
	var seedErr error // written by the worker before create's callbacks run
	ws.create(func() (string, error) {
		t, err := ws.b.NewScratchRequest()
		if err == nil {
			seedErr = ws.b.SeedRequestHooks(t)
		}
		return requestNodeID(t), err
	}, ws.openThenReport(open, fail, &seedErr), fail)
}

// CreateRequest creates a request at path in collection; "" means Scratch.
// The request is seeded with its starting hooks (requests.seed_blueprint); if
// only that fails the request still opens and fail gets the error.
func (ws *Workspace) CreateRequest(collection model.ID, path string, open func(nodeID string), fail func(error)) {
	var seedErr error
	path = ws.untitledIfBlank(collection, path)
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewRequest(id, path)
		if err == nil {
			seedErr = ws.b.SeedRequestHooks(t)
		}
		return requestNodeID(t), err
	}, ws.openThenReport(open, fail, &seedErr), fail)
}

// CreateAsyncRequest is CreateRequest for an async request of the given
// protocol (ws, sse, mqtt, kafka, amqp or nats).
func (ws *Workspace) CreateAsyncRequest(collection model.ID, path, protocol string, open func(nodeID string), fail func(error)) {
	var seedErr error
	path = ws.untitledIfBlank(collection, path)
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewAsyncRequest(id, path, protocol)
		if err == nil {
			seedErr = ws.b.SeedRequestHooks(t) // the library seeds an async request with async-aware hooks
		}
		return requestNodeID(t), err
	}, ws.openThenReport(open, fail, &seedErr), fail)
}

// CreateGRPCRequest is CreateRequest for a gRPC request.
func (ws *Workspace) CreateGRPCRequest(collection model.ID, path string, open func(nodeID string), fail func(error)) {
	var seedErr error
	path = ws.untitledIfBlank(collection, path)
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewGRPCRequest(id, path)
		if err == nil {
			seedErr = ws.b.SeedRequestHooks(t) // the library seeds a gRPC request with hooks that read the status code
		}
		return requestNodeID(t), err
	}, ws.openThenReport(open, fail, &seedErr), fail)
}

// CreateDatasetRequest is CreateRequest for a dataset request of the given driver.
func (ws *Workspace) CreateDatasetRequest(collection model.ID, path, driver string, open func(nodeID string), fail func(error)) {
	var seedErr error
	path = ws.untitledIfBlank(collection, path)
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewDatasetRequest(id, path, driver)
		if err == nil {
			seedErr = ws.b.SeedRequestHooks(t) // the library seeds a dataset request with hooks that read the outcome
		}
		return requestNodeID(t), err
	}, ws.openThenReport(open, fail, &seedErr), fail)
}

// GenerateGRPCRequests creates a gRPC request for each of methods in
// collection (see flowi.Workspace.GenerateGRPCRequests), shows the tree with
// them, then calls done with what was created and skipped.
func (ws *Workspace) GenerateGRPCRequests(collection model.ID, base model.GRPC, methods []grpcx.Method, done func(flowi.GeneratedGRPC), fail func(error)) {
	var result flowi.GeneratedGRPC
	ws.create(func() (string, error) {
		var err error
		result, err = ws.b.GenerateGRPCRequests(collection, base, methods)
		return "", err
	}, func(string) {
		if done != nil {
			done(result)
		}
	}, fail)
}

// CreateFolder creates the folder path under tree ("requests" or "flows") of
// collection and shows it in the tree.
func (ws *Workspace) CreateFolder(collection model.ID, tree, path string, fail func(error)) {
	trimmed := strings.TrimSpace(path)
	path = strings.Trim(trimmed, "/")
	if path == "" || strings.HasSuffix(trimmed, "/") {
		path = ws.freeFolderName(collection, tree, strings.Trim(trimmed, "/"))
	}
	ws.create(func() (string, error) {
		return "", ws.b.NewFolder(collection, tree, path)
	}, nil, fail)
}

// CreateDraftCollection creates a collection named name in Flowi's own folder and
// calls open with its tree row: the place a request can be put without choosing a
// folder on disk first.
func (ws *Workspace) CreateDraftCollection(name string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		m, err := ws.b.NewDraft(strings.TrimSpace(name))
		return "c:" + string(m.ID), err
	}, open, fail)
}

// CreateFolderAt is CreateFolder that calls open with the new folder's tree row.
func (ws *Workspace) CreateFolderAt(collection model.ID, tree, path string, open func(nodeID string), fail func(error)) {
	path = strings.Trim(strings.TrimSpace(path), "/")
	ws.create(func() (string, error) {
		if err := ws.b.NewFolder(collection, tree, path); err != nil {
			return "", err
		}
		return "c:" + string(collection) + "/" + tree + "/" + path, nil
	}, open, fail)
}

// SetFolderOrder sets the order the entries of folder dir ("" is the top of the
// tree) are shown in, then shows the tree again. names are the entries' names:
// a request or flow without its file suffix, or a sub-folder's name. fail gets
// any error (FLW-WRITE-001 when the folder file has comments, which a retry with
// dropComments allows).
func (ws *Workspace) SetFolderOrder(collection model.ID, tree, dir string, names []string, dropComments bool, fail func(error)) {
	ws.create(func() (string, error) {
		return "", ws.b.SetFolderOrder(collection, tree, dir, names, dropComments)
	}, nil, fail)
}

// freeFolderName is "<parent>/new-folder", numbered when that exists: the
// name a folder gets when none was typed. parent may be empty.
func (ws *Workspace) freeFolderName(collection model.ID, tree, parent string) string {
	taken := map[string]bool{}
	if c, ok := ws.Collection(collection); ok {
		for _, f := range c.Folders {
			taken[f.Dir] = true
		}
	}
	join := func(name string) string {
		if parent == "" {
			return name
		}
		return parent + "/" + name
	}
	name := "new-folder"
	for n := 2; taken[tree+"/"+join(name)]; n++ {
		name = fmt.Sprintf("new-folder-%d", n)
	}
	return join(name)
}

// untitledIfBlank gives a request without a name (or only a folder prefix,
// "auth/") the name "untitled", numbered when the collection has one.
func (ws *Workspace) untitledIfBlank(collection model.ID, path string) string {
	path = strings.TrimSpace(path)
	if path != "" && !strings.HasSuffix(path, "/") {
		return path
	}
	taken := map[string]bool{}
	if c, ok := ws.Collection(collection); ok {
		for _, r := range c.Requests {
			taken[r.Path] = true
		}
	}
	name := path + "untitled"
	for n := 2; taken[name]; n++ {
		name = fmt.Sprintf("%suntitled-%d", path, n)
	}
	return name
}

// openThenReport opens the new node, then reports a seeding error, which does
// not undo the creation.
func (ws *Workspace) openThenReport(open func(string), fail func(error), seedErr *error) func(string) {
	return func(node string) {
		if open != nil {
			open(node)
		}
		if *seedErr != nil && fail != nil {
			fail(*seedErr)
		}
	}
}

// CreateFlow creates a flow at path in collection; "" means Scratch.
func (ws *Workspace) CreateFlow(collection model.ID, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewFlow(id, path)
		return "f:" + string(t.Collection) + ":" + string(t.Flow), err
	}, open, fail)
}

// CreateUntitledFlow creates the next "untitled" flow in collection ("" means
// Scratch), then calls open with its tree node id.
func (ws *Workspace) CreateUntitledFlow(collection model.ID, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		id, err := ws.orScratch(collection)
		if err != nil {
			return "", err
		}
		t, err := ws.b.NewUntitledFlow(id)
		return "f:" + string(t.Collection) + ":" + string(t.Flow), err
	}, open, fail)
}

// CreateUntitledFlowIn creates the next "untitled" flow inside folder of
// collection's flows ("" is the top, as CreateUntitledFlow). The folder a
// person right-clicked is where they expect the flow to be.
func (ws *Workspace) CreateUntitledFlowIn(collection model.ID, folder string, open func(nodeID string), fail func(error)) {
	if folder == "" {
		ws.CreateUntitledFlow(collection, open, fail)
		return
	}
	taken := map[string]bool{}
	if c, ok := ws.Collection(collection); ok {
		for _, f := range c.Flows {
			taken[f.Path] = true
		}
	}
	path := strings.Trim(folder, "/") + "/untitled"
	for n := 2; taken[path]; n++ {
		path = fmt.Sprintf("%s/untitled-%d", strings.Trim(folder, "/"), n)
	}
	ws.CreateFlow(collection, path, open, fail)
}

// CreateEnvironment creates the environment name in the workspace and opens
// its node.
func (ws *Workspace) CreateEnvironment(name string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		file, err := ws.b.NewEnvironment(name)
		return "e:" + file, err
	}, open, fail)
}

func requestNodeID(t flowi.RequestTarget) string {
	return "r:" + string(t.Collection) + ":" + string(t.Request)
}

// orScratch runs off the UI goroutine.
func (ws *Workspace) orScratch(collection model.ID) (model.ID, error) {
	if collection != "" {
		return collection, nil
	}
	m, err := ws.b.Scratch()
	return m.ID, err
}

type created struct {
	node string
	snap snapshot
	err  error
}

// create runs build in the background, then shows the new tree and opens
// the node. A new Scratch is a new root, so the watch restarts too.
func (ws *Workspace) create(build func() (string, error), open func(string), fail func(error)) {
	Run(ws.w, func() created {
		node, err := build()
		if err != nil {
			return created{err: err}
		}
		return created{node: node, snap: buildSnapshot(ws.b)}
	}, func(c created) {
		if c.err != nil {
			if fail != nil {
				fail(c.err)
			}
			return
		}
		ws.touched[c.node] = time.Now() // new items show as "now" in Drafts
		ws.apply(c.snap)
		ws.Rewatch()
		if open != nil {
			open(c.node)
		}
	})
}

// MoveRequest moves request t to path in collection to (its own collection:
// a rename), then calls open with the request's new tree node id. fail, if
// set, gets any error.
func (ws *Workspace) MoveRequest(t flowi.RequestTarget, to model.ID, path string, open func(nodeID string), fail func(error)) {
	ws.create(func() (string, error) {
		moved, err := ws.b.MoveRequest(t, to, path)
		return requestNodeID(moved), err
	}, open, fail)
}
