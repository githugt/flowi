// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"
)

// NodeKind is what a workspace tree node shows.
type NodeKind int

// Node kinds.
const (
	NodeCollection NodeKind = iota + 1
	NodeMissing             // a mounted collection whose directory is gone
	NodeRequestsGroup
	NodeFolder
	NodeRequest
	NodeEnvironmentsGroup
	NodeEnvironment
	NodeFlowsGroup
	NodeFlow
)

// TreeNode is one row of the workspace tree.
type TreeNode struct {
	ID         string
	Label      string
	Kind       NodeKind
	Collection model.ID
	Request    model.ID // NodeRequest
	Flow       model.ID // NodeFlow
	File       string   // NodeRequest, NodeEnvironment, NodeFlow
	Path       string   // NodeRequest, NodeFlow: the path in the collection; NodeFolder: the folder
	Draft      bool     // NodeCollection: not saved to a folder yet
	Method     string   // NodeRequest: the verb of its default version
	Tree       string   // NodeFolder and the two groups: "requests" or "flows"
}

// snapshot is everything the workspace view-model shows, built off the UI
// goroutine and swapped in on it.
type snapshot struct {
	collections []model.Collection
	drafts      map[model.ID]bool
	nodes       map[string]TreeNode
	children    map[string][]string
	envs        []string
	environment []model.Environment // the workspace's own, shared by every collection
	defaultEnv  string
}

// Workspace is the workspace tree and the selected environment.
//
// State changes happen on the UI goroutine only (through the worker's
// poster); the watch goroutine reads the backend and posts.
type Workspace struct {
	b         Backend
	w         *Worker
	snap      snapshot
	env       string
	onChange  []func()
	onFile    []func(file string)
	onLabel   []func()
	envChosen bool // the user picked an environment: reloads keep it
	touched   map[string]time.Time
	labels    map[string]string // display names that replace a node's label (untitled drafts show their URL)

	watchCtx  context.Context    // the app's lifetime, from Watch
	stopWatch context.CancelFunc // stops the current watch; Rewatch replaces it
}

// NewWorkspace returns an empty workspace view-model; call Load.
func NewWorkspace(b Backend, w *Worker) *Workspace {
	return &Workspace{b: b, w: w, touched: map[string]time.Time{}, labels: map[string]string{}, snap: snapshot{nodes: map[string]TreeNode{}, children: map[string][]string{}}}
}

// OnChange registers f to run (on the UI goroutine) after the tree or the
// environment changes.
func (ws *Workspace) OnChange(f func()) { ws.onChange = append(ws.onChange, f) }

// OnLabelChange registers f to run when a display name set with SetLabel
// changes: the tree redraws, nothing is reloaded or re-validated.
func (ws *Workspace) OnLabelChange(f func()) { ws.onLabel = append(ws.onLabel, f) }

// OnFileChanged registers f to run (on the UI goroutine) with each file
// changed on disk, after the workspace has reloaded it.
func (ws *Workspace) OnFileChanged(f func(file string)) { ws.onFile = append(ws.onFile, f) }

// Load reads the workspace in the background and refreshes the tree.
func (ws *Workspace) Load() {
	Run(ws.w, func() snapshot { return buildSnapshot(ws.b) }, ws.apply)
}

// Watch reloads files changed on disk until ctx ends. Call Rewatch after
// the set of mounted collections changes.
//
// Goroutine ownership: each watch runs one goroutine that ends when its
// context is done (ctx, or Rewatch replacing it) or the backend's change
// channel closes.
func (ws *Workspace) Watch(ctx context.Context) {
	ws.watchCtx = ctx
	ws.startWatch()
}

// Rewatch restarts the watch so it covers the collections mounted now.
func (ws *Workspace) Rewatch() {
	if ws.watchCtx == nil {
		return // not watching (tests, or before startup)
	}
	ws.startWatch()
}

func (ws *Workspace) startWatch() {
	if ws.stopWatch != nil {
		ws.stopWatch()
	}
	ctx, cancel := context.WithCancel(ws.watchCtx)
	ws.stopWatch = cancel
	ws.w.wg.Go(func() { ws.watch(ctx) })
}

func (ws *Workspace) watch(ctx context.Context) {
	changes, err := ws.b.Watch(ctx)
	if err != nil || changes == nil {
		return // no live reload; the tree still refreshes after the GUI's own saves
	}
	for ch := range changes {
		if ch.Err != nil {
			continue // a failing watcher ends the channel; nothing to reload
		}
		if _, err := ws.b.Reload(ch); err != nil {
			continue // the Problems panel shows what is wrong with the file
		}
		snap := buildSnapshot(ws.b)
		ws.w.Post(func() {
			ws.apply(snap)
			for _, f := range ws.onFile {
				f(ch.File)
			}
		})
	}
}

func (ws *Workspace) apply(s snapshot) {
	ws.snap = s
	// "No environment" is a choice, not a name that went missing: a reload (every
	// send causes one) must not put the default back.
	if !ws.envChosen || ws.env != "" {
		ws.env = ws.usableEnv(s)
	}
	ws.notify()
}

// usableEnv is the environment to keep after a reload: the chosen one while
// it still exists, else the default, else the first one (sends need some
// environment).
func (ws *Workspace) usableEnv(s snapshot) string {
	env := ws.env
	if !ws.envChosen || !slices.Contains(s.envs, env) {
		env = s.defaultEnv
	}
	if !slices.Contains(s.envs, env) && len(s.envs) > 0 {
		env = s.envs[0]
	}
	return env
}

func (ws *Workspace) notify() {
	for _, f := range ws.onChange {
		f()
	}
}

// Children lists the ids under id; "" lists the roots.
func (ws *Workspace) Children(id string) []string { return ws.snap.children[id] }

// Node returns the node with id.
func (ws *Workspace) Node(id string) TreeNode { return ws.snap.nodes[id] }

// IsBranch reports whether id can have children.
func (ws *Workspace) IsBranch(id string) bool {
	switch ws.snap.nodes[id].Kind {
	case NodeRequest, NodeEnvironment, NodeFlow, NodeMissing:
		return false
	}
	return true
}

// InScratch reports whether the node belongs to the Drafts scratch space.
func (ws *Workspace) InScratch(nodeID string) bool {
	c, ok := ws.Collection(ws.snap.nodes[nodeID].Collection)
	return ok && ws.snap.drafts[c.Manifest.ID] && c.Manifest.Name == scratchName
}

// WorkspaceName is the open workspace's name.
func (ws *Workspace) WorkspaceName() string { return ws.b.WorkspaceName() }

// IsDraftCollection reports whether collection id is a draft (not saved to a folder yet).
func (ws *Workspace) IsDraftCollection(id model.ID) bool { return ws.snap.drafts[id] }

// InDraft reports whether the node belongs to a draft collection.
func (ws *Workspace) InDraft(nodeID string) bool {
	return ws.snap.drafts[ws.snap.nodes[nodeID].Collection]
}

// Environment returns the workspace's environments.
func (ws *Workspace) Environment() []model.Environment { return ws.snap.environment }

// Environments lists the environment names, sorted.
func (ws *Workspace) Environments() []string { return ws.snap.envs }

// EnvColours maps each environment name to its colour; an environment with the default colour is absent.
func (ws *Workspace) EnvColours() map[string]string {
	out := map[string]string{}
	for _, e := range ws.snap.environment {
		if e.Colour != "" {
			out[e.Name] = e.Colour
		}
	}
	return out
}

// SelectedEnv is the environment sends use.
func (ws *Workspace) SelectedEnv() string { return ws.env }

// SelectEnv picks the environment for sends.
func (ws *Workspace) SelectEnv(name string) {
	ws.env, ws.envChosen = name, true
	ws.notify()
}

// VariableNames are the variables (and, separately, the secrets) the
// selected environment defines: what {{env.x}} and {{secret.x}} can name.
func (ws *Workspace) VariableNames() (vars, secrets []string) {
	for _, e := range ws.snap.environment {
		if e.Name != ws.env {
			continue
		}
		for _, v := range e.Vars {
			if v.Secret {
				secrets = append(secrets, v.Name)
			} else {
				vars = append(vars, v.Name)
			}
		}
	}
	return vars, secrets
}

// Collection returns a loaded collection.
func (ws *Workspace) Collection(id model.ID) (model.Collection, bool) {
	i := slices.IndexFunc(ws.snap.collections, func(c model.Collection) bool { return c.Manifest.ID == id })
	if i < 0 {
		return model.Collection{}, false
	}
	return ws.snap.collections[i], true
}

// Request returns a loaded request and its collection.
func (ws *Workspace) Request(t flowi.RequestTarget) (model.Collection, model.Request, bool) {
	c, ok := ws.Collection(t.Collection)
	if !ok {
		return model.Collection{}, model.Request{}, false
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == t.Request })
	if i < 0 {
		return model.Collection{}, model.Request{}, false
	}
	return c, c.Requests[i], true
}

// Display is the name the tree shows for a node: the label set with
// SetLabel, else its own.
func (ws *Workspace) Display(nodeID string) string {
	if l, ok := ws.labels[nodeID]; ok {
		return l
	}
	return ws.snap.nodes[nodeID].Label
}

// SetLabel shows label for nodeID instead of its own ("" restores it). Only
// the tree redraws, and only when the shown name changes.
func (ws *Workspace) SetLabel(nodeID, label string) {
	if ws.labels[nodeID] == label {
		return
	}
	if label == "" {
		delete(ws.labels, nodeID)
	} else {
		ws.labels[nodeID] = label
	}
	for _, f := range ws.onLabel {
		f()
	}
}

// Touched is when this session last created or edited the node (a request
// or flow), or the zero time when it did not: the tree shows it as an age
// on draft rows.
func (ws *Workspace) Touched(nodeID string) time.Time { return ws.touched[nodeID] }

// Touch records that nodeID was just created or edited.
func (ws *Workspace) Touch(nodeID string) { ws.TouchAt(nodeID, time.Now()) }

// TouchAt is Touch with an explicit time.
func (ws *Workspace) TouchAt(nodeID string, at time.Time) {
	previous := ws.touched[nodeID]
	ws.touched[nodeID] = at
	if at.Sub(previous) > touchRefresh {
		ws.notify() // the displayed age changes; typing does not redraw the tree
	}
}

// touchRefresh is how long a node stays as it is shown before a new touch
// redraws the tree.
const touchRefresh = 30 * time.Second

// Collections are the loaded collections.
func (ws *Workspace) Collections() []model.Collection { return ws.snap.collections }

// buildSnapshot reads the backend; it runs off the UI goroutine.
func buildSnapshot(b Backend) snapshot {
	s := snapshot{collections: b.Collections(), drafts: map[model.ID]bool{}, nodes: map[string]TreeNode{}, children: map[string][]string{}}
	add := func(parent string, n TreeNode) {
		s.nodes[n.ID] = n
		s.children[parent] = append(s.children[parent], n.ID)
	}
	for _, c := range draftsFirst(s.collections, b.IsDraft) {
		s.drafts[c.Manifest.ID] = b.IsDraft(c.Manifest.ID)
		addCollection(add, c, s.drafts[c.Manifest.ID])
	}
	paths := b.MountedPaths()
	for _, id := range b.Missing() {
		add("", TreeNode{ID: "m:" + string(id), Label: paths[id] + " (missing)", Kind: NodeMissing, Collection: id})
	}
	s.environment = b.Environments()
	addEnvironments(add, s.environment) // after the collections: they are what people open first
	for _, e := range s.environment {
		s.envs = append(s.envs, e.Name)
	}
	slices.Sort(s.envs)
	if v, ok := b.Settings("").Get("defaults.environment"); ok {
		s.defaultEnv, _ = v.Value.(string)
	}
	return s
}

// scratchName is the Scratch draft's name (flowi-lib drafts_api.go); the
// tree calls it Drafts.
const scratchName = "Scratch"

// draftsFirst orders collections for the tree: Scratch (shown as Drafts)
// first, then the rest in load order.
func draftsFirst(cs []model.Collection, isDraft func(model.ID) bool) []model.Collection {
	rank := func(c model.Collection) int {
		if isDraft(c.Manifest.ID) && c.Manifest.Name == scratchName {
			return 0
		}
		return 1
	}
	out := slices.Clone(cs)
	slices.SortStableFunc(out, func(a, b model.Collection) int { return rank(a) - rank(b) })
	return out
}

// environmentsGroupID is the tree node that holds the workspace's
// environments, below the collections.
const environmentsGroupID = "environments"

// addEnvironments adds the Environments group: environments belong to the
// workspace, not to a collection.
func addEnvironments(add func(string, TreeNode), envs []model.Environment) {
	add("", TreeNode{ID: environmentsGroupID, Label: "Environments", Kind: NodeEnvironmentsGroup})
	for _, e := range sortedBy(envs, func(e model.Environment) string { return e.Name }) {
		add(environmentsGroupID, TreeNode{ID: "e:" + e.File, Label: e.Name, Kind: NodeEnvironment, File: e.File})
	}
}

func addCollection(add func(string, TreeNode), c model.Collection, draft bool) {
	cid := c.Manifest.ID
	root := "c:" + string(cid)
	label := c.Manifest.Name
	if draft && label == scratchName {
		label = "Drafts" // the scratch space; other collections in Flowi's folder are ordinary collections
	}
	add("", TreeNode{ID: root, Label: label, Kind: NodeCollection, Collection: cid, Draft: draft})
	reqs := root + "/requests"
	add(root, TreeNode{ID: reqs, Label: "Requests", Kind: NodeRequestsGroup, Collection: cid, Tree: "requests"})
	addNested(add, reqs, "requests", c, requestLeaves(c))
	flows := root + "/flows"
	add(root, TreeNode{ID: flows, Label: "Flows", Kind: NodeFlowsGroup, Collection: cid, Tree: "flows"})
	addNested(add, flows, "flows", c, flowLeaves(c))
}

// treeLeaf is a request or flow to place in a nested tree by its path.
type treeLeaf struct {
	path string
	node TreeNode
}

// addNested nests leaves under folder nodes from their paths; folders come
// before leaves, each sorted. The collection's _folder.yaml folders of this
// tree are shown too, so an empty folder exists before anything is in it.
func addNested(add func(string, TreeNode), parent, tree string, c model.Collection, leaves []treeLeaf) {
	type entry struct {
		folder string // set for a folder
		leaf   treeLeaf
	}
	byDir := map[string][]entry{}
	seen := map[string]bool{}
	addFolder := func(dir string) {
		for d := dir; d != "." && !seen[d]; d = path.Dir(d) {
			seen[d] = true
			byDir[path.Dir(d)] = append(byDir[path.Dir(d)], entry{folder: d})
		}
	}
	for _, l := range leaves {
		dir := path.Dir(l.path)
		byDir[dir] = append(byDir[dir], entry{leaf: l})
		addFolder(dir)
	}
	for _, f := range c.Folders {
		if rel, ok := strings.CutPrefix(f.Dir, tree+"/"); ok {
			addFolder(rel)
		}
	}
	var walk func(dir, parentID string)
	walk = func(dir, parentID string) {
		entries := byDir[dir]
		order := orderOf(c, tree, dir)
		slices.SortFunc(entries, func(a, b entry) int {
			if (a.folder == "") != (b.folder == "") {
				return map[bool]int{true: -1, false: 1}[a.folder != ""]
			}
			if byOrder := orderIndex(order, entryName(a.folder, a.leaf.path)) - orderIndex(order, entryName(b.folder, b.leaf.path)); byOrder != 0 {
				return byOrder
			}
			return strings.Compare(a.folder+a.leaf.path, b.folder+b.leaf.path)
		})
		for _, e := range entries {
			if e.folder != "" {
				id := parentID + "/" + path.Base(e.folder)
				add(parentID, TreeNode{ID: id, Label: path.Base(e.folder), Kind: NodeFolder, Collection: c.Manifest.ID, Path: e.folder, Tree: tree})
				walk(e.folder, id)
				continue
			}
			add(parentID, e.leaf.node)
		}
	}
	walk(".", parent)
}

// orderOf is the order the _folder.yaml of directory dir (relative to the tree, "."
// for its top) gives its entries, nil when it gives none.
func orderOf(c model.Collection, tree, dir string) []string {
	rel := tree
	if dir != "." {
		rel = tree + "/" + dir
	}
	for _, f := range c.Folders {
		if f.Dir == rel {
			return f.Order
		}
	}
	return nil
}

// orderIndex is where name stands in order; an entry it does not name follows
// the ones it does.
func orderIndex(order []string, name string) int {
	if i := slices.Index(order, name); i >= 0 {
		return i
	}
	return len(order)
}

// entryName is the name an order refers to: a folder's, or a leaf's file name
// without its suffix.
func entryName(folder, leafPath string) string {
	if folder != "" {
		return path.Base(folder)
	}
	return path.Base(leafPath)
}

func requestLeaves(c model.Collection) []treeLeaf {
	out := make([]treeLeaf, len(c.Requests))
	for i, r := range c.Requests {
		label := r.Name
		if label == "" {
			label = path.Base(r.Path)
		}
		out[i] = treeLeaf{r.Path, TreeNode{ID: "r:" + string(c.Manifest.ID) + ":" + string(r.ID), Label: label, Kind: NodeRequest,
			Collection: c.Manifest.ID, Request: r.ID, File: r.File, Path: r.Path, Method: verbOf(r)}}
	}
	return out
}

func flowLeaves(c model.Collection) []treeLeaf {
	out := make([]treeLeaf, len(c.Flows))
	for i, f := range c.Flows {
		out[i] = treeLeaf{f.Path, TreeNode{ID: "f:" + string(c.Manifest.ID) + ":" + string(f.ID), Label: path.Base(f.Path), Kind: NodeFlow,
			Collection: c.Manifest.ID, Flow: f.ID, File: f.File, Path: f.Path}}
	}
	return out
}

func sortedBy[T any](xs []T, key func(T) string) []T {
	out := slices.Clone(xs)
	slices.SortFunc(out, func(a, b T) int { return strings.Compare(key(a), key(b)) })
	return out
}

// verbOf is the verb (or protocol) of the version a request opens on.
func verbOf(r model.Request) string {
	if len(r.Versions) == 0 {
		return ""
	}
	return Verb(r.Versions[defaultIndex(r)])
}
