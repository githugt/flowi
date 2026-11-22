// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"path"
	"slices"
	"strings"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"
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
	NodeBroken // a request or flow file that could not be read: unmerged, or not valid YAML
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
	Scratch    bool     // NodeCollection: the Drafts try-out space, which is not a collection the person made
	Method     string   // NodeRequest: the verb of its default version
	Tree       string   // NodeFolder and the two groups: "requests" or "flows"
	Detail     string   // NodeBroken: why it could not be read, with the line
	// Status is the development status of a NodeRequest, NodeFlow or NodeFolder as it resolves (its own, else the nearest
	// folder's); the zero value for none. A name the collection does not have is kept with the alert icon.
	Status model.Status
	// StatusFrom is the folder (as its Dir, "requests/auth") an inherited status comes from; "" for the item's own.
	StatusFrom string
	// StatusUnknown is set when Status names no status of the collection (FLW-STATUS-001).
	StatusUnknown bool
	// Tags are the tags that apply to a NodeRequest, NodeFlow or NodeFolder: its own and those of the folders above.
	Tags []string
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
	b              Backend
	w              *Worker
	snap           snapshot
	env            string
	onChange       []func()
	onFile         []func(file string)
	onWatchProblem []func(error)
	watchRetry     time.Duration // the first wait before a failed watch is started again
	onLabel        []func()
	envChosen      bool // the user picked an environment: reloads keep it
	touched        map[string]time.Time
	labels         map[string]string // display names that replace a node's label (untitled drafts show their URL)

	watchCtx  context.Context    // the app's lifetime, from Watch
	stopWatch context.CancelFunc // stops the current watch; Rewatch replaces it
}

// NewWorkspace returns an empty workspace view-model; call Load.
func NewWorkspace(b Backend, w *Worker) *Workspace {
	return &Workspace{b: b, w: w, watchRetry: 2 * time.Second, touched: map[string]time.Time{}, labels: map[string]string{}, snap: snapshot{nodes: map[string]TreeNode{}, children: map[string][]string{}}}
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
	// Before: a watcher that failed to start (an inotify limit) or ended with an error just returned, so live reload
	// stopped and nothing told the person. Now the problem is reported once and the watch is retried with a back-off.
	delay := ws.watchRetry
	for attempt := 0; ; attempt++ {
		failed := ws.watchOnce(ctx, attempt == 0)
		if !failed || ctx.Err() != nil || attempt >= watchRetries {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(delay):
		}
		delay *= 2
	}
}

// watchRetries is how often a failed watch is started again before the workspace gives up.
const watchRetries = 4

// watchOnce runs one watch until its channel ends. It reports true when the watch failed (it did not start, or it
// ended with an error) so the caller can try again; first says if the problem is new to the person.
func (ws *Workspace) watchOnce(ctx context.Context, first bool) (failed bool) {
	changes, err := ws.b.Watch(ctx)
	if err != nil {
		if first {
			ws.reportWatchProblem(err)
		}
		return true
	}
	if changes == nil {
		return false
	}
	for ch := range changes {
		if ch.Err != nil {
			if first {
				ws.reportWatchProblem(ch.Err)
			}
			failed = true
			continue // a failing watcher ends the channel
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
	return failed
}

// reportWatchProblem tells the registered listeners, on the UI goroutine, that live reload is not working.
func (ws *Workspace) reportWatchProblem(err error) {
	ws.w.Post(func() {
		for _, f := range ws.onWatchProblem {
			f(err)
		}
	})
}

// OnWatchProblem registers f to run (on the UI goroutine) when the file watcher fails, so the app can say that
// changes made outside it will not show up until it recovers.
func (ws *Workspace) OnWatchProblem(f func(error)) { ws.onWatchProblem = append(ws.onWatchProblem, f) }

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

// IsScratch reports whether c is the Scratch try-out space (not any other unsaved collection).
func (ws *Workspace) IsScratch(c model.Collection) bool {
	return ws.snap.drafts[c.Manifest.ID] && c.Manifest.Name == scratchName
}

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

// ProtectedEnvs lists the names of the environments that ask for confirmation before a request that changes
// data is sent, sorted.
func (ws *Workspace) ProtectedEnvs() []string {
	out := []string{}
	for _, e := range ws.snap.environment {
		if e.Protected {
			out = append(out, e.Name)
		}
	}
	slices.Sort(out)
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
	// Before: only the selected environment's own rows counted, so a variable it inherits from the environment it
	// "builds on" showed as not set. The nearest environment decides whether a name is a secret.
	chain := ws.SelectedChain()
	seen := map[string]bool{}
	for i := len(chain) - 1; i >= 0; i-- {
		for _, e := range ws.snap.environment {
			if e.Name != chain[i] {
				continue
			}
			for _, v := range e.Vars {
				if seen[v.Name] {
					continue
				}
				seen[v.Name] = true
				if v.Secret {
					secrets = append(secrets, v.Name)
				} else {
					vars = append(vars, v.Name)
				}
			}
		}
	}
	return vars, secrets
}

// SelectedChain is the names of the selected environment and the ones it builds on, the base first and the
// selected one last. It is just the selected one when it builds on nothing or its chain is broken (the Problems
// panel reports that), and empty when none is selected.
func (ws *Workspace) SelectedChain() []string {
	if ws.env == "" {
		return nil
	}
	chain, err := model.EnvironmentChain(ws.snap.environment, ws.env)
	if err != nil {
		return []string{ws.env}
	}
	names := make([]string, len(chain))
	for i, e := range chain {
		names[i] = e.Name
	}
	return names
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

// Flow returns a loaded flow and its collection.
func (ws *Workspace) Flow(t flowi.FlowTarget) (model.Collection, model.Flow, bool) {
	c, ok := ws.Collection(t.Collection)
	if !ok {
		return model.Collection{}, model.Flow{}, false
	}
	i := slices.IndexFunc(c.Flows, func(f model.Flow) bool { return f.ID == t.Flow })
	if i < 0 {
		return model.Collection{}, model.Flow{}, false
	}
	return c, c.Flows[i], true
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
	add("", TreeNode{ID: root, Label: label, Kind: NodeCollection, Collection: cid, Draft: draft, Scratch: draft && c.Manifest.Name == scratchName})
	reqs := root + "/requests"
	add(root, TreeNode{ID: reqs, Label: "Requests", Kind: NodeRequestsGroup, Collection: cid, Tree: "requests"})
	addNested(add, reqs, "requests", c, append(requestLeaves(c), brokenLeaves(c, "request")...))
	flows := root + "/flows"
	add(root, TreeNode{ID: flows, Label: "Flows", Kind: NodeFlowsGroup, Collection: cid, Tree: "flows"})
	addNested(add, flows, "flows", c, append(flowLeaves(c), brokenLeaves(c, "flow")...))
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
				folder := TreeNode{ID: id, Label: path.Base(e.folder), Kind: NodeFolder, Collection: c.Manifest.ID, Path: e.folder, Tree: tree}
				withLabels(&folder, c, c.FolderStatus(tree+"/"+e.folder), c.FolderTags(tree+"/"+e.folder))
				add(parentID, folder)
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
		node := TreeNode{ID: "r:" + string(c.Manifest.ID) + ":" + string(r.ID), Label: label, Kind: NodeRequest,
			Collection: c.Manifest.ID, Request: r.ID, File: r.File, Path: r.Path, Method: verbOf(r)}
		withLabels(&node, c, c.RequestStatus(r), c.RequestTags(r))
		out[i] = treeLeaf{r.Path, node}
	}
	return out
}

// brokenLeaves are the files of c that could not be read, placed where the request or flow would have been, so a file
// with merge conflict markers stays in the tree (marked) instead of vanishing as if deleted.
func brokenLeaves(c model.Collection, kind string) []treeLeaf {
	var out []treeLeaf
	for _, b := range c.Broken {
		if b.Kind != kind {
			continue
		}
		out = append(out, treeLeaf{b.Path, TreeNode{ID: "x:" + string(c.Manifest.ID) + ":" + b.File, Label: path.Base(b.Path), Kind: NodeBroken,
			Collection: c.Manifest.ID, File: b.File, Path: b.Path, Detail: b.Message, Tree: kind + "s"}})
	}
	return out
}

func flowLeaves(c model.Collection) []treeLeaf {
	out := make([]treeLeaf, len(c.Flows))
	for i, f := range c.Flows {
		node := TreeNode{ID: "f:" + string(c.Manifest.ID) + ":" + string(f.ID), Label: path.Base(f.Path), Kind: NodeFlow,
			Collection: c.Manifest.ID, Flow: f.ID, File: f.File, Path: f.Path}
		withLabels(&node, c, c.FlowStatus(f), c.FlowTags(f))
		out[i] = treeLeaf{f.Path, node}
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
