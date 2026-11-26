// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"cmp"
	"fmt"
	"path"
	"strings"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// Prompt describes the question an action asks before it runs.
type Prompt struct {
	Title     string   `json:"title"`
	Label     string   `json:"label"`
	Initial   string   `json:"initial"`
	Protocols []Choice `json:"protocols,omitempty"` // a protocol is asked too
}

// MenuEntry is one line of the tree's context menu.
type MenuEntry struct {
	Action  string  `json:"action"`
	Label   string  `json:"label"`
	Prompt  *Prompt `json:"prompt,omitempty"`  // ask this first
	Confirm string  `json:"confirm,omitempty"` // ask yes or no first
	// ConfirmTitle and ConfirmAction name the question and its yes button when the plain "Delete" would hide something:
	// a delete that leaves flow steps pointing at nothing says so on both.
	ConfirmTitle  string `json:"confirmTitle,omitempty"`
	ConfirmAction string `json:"confirmAction,omitempty"`
	Guard         string `json:"guard,omitempty"`  // "dirty": a tab of these rows may have unsaved edits; the window warns that they are lost before it goes on
	Dialog        string `json:"dialog,omitempty"` // "move": the window asks where with its own dialog
}

// Notice is published as the "notice" event: something the user should see.
type Notice struct {
	Level string `json:"level"` // error or info
	Text  string `json:"text"`
}

// actionNames names the actions this window can run (see do). The view-model
// offers more (move a draft to a folder, discard a draft, manage a missing
// collection); until they are ported they have no name here and stay out of
// the menu, rather than showing an entry that does nothing.
func actionNames() map[vm.Action]string {
	return map[vm.Action]string{
		vm.ActOpen: "open", vm.ActDuplicate: "duplicate", vm.ActRename: "rename", vm.ActDelete: "delete", vm.ActCopyMermaid: "mermaid",
		vm.ActNewRequest: "newRequest", vm.ActNewEnvironment: "newEnvironment", vm.ActNewFlow: "newFlow", vm.ActFlowFromRequests: "flowFromRequests",
		vm.ActNewFolder: "newFolder", vm.ActNewAsyncRequest: "newAsync", vm.ActNewGRPCRequest: "newGRPC", vm.ActNewGraphQLRequest: "newGraphQL", vm.ActNewDatasetRequest: "newDataset", vm.ActNewDataFileRequest: "newDataFile",
	}
}

// Menu is the context menu for the given rows (one row, or the marked rows).
func (s *WorkspaceService) Menu(ids []string) []MenuEntry {
	var out []MenuEntry
	s.app.actor.Do(func() {
		ws, names := s.app.workspace, actionNames()
		for _, item := range ws.MenuFor(ids) {
			if _, ported := names[item.Action]; ported {
				out = append(out, s.entry(item, ids))
			}
		}
		out = withMove(out, ws, ids)
		out = withLabelsEntry(out, ws, ids)
	})
	return out
}

func (s *WorkspaceService) entry(item vm.MenuItem, ids []string) MenuEntry {
	ws := s.app.workspace
	n := ws.Node(ids[0])
	e := MenuEntry{Action: actionNames()[item.Action], Label: menuLabel(item)}
	_, where := folderPrefix(n)
	switch item.Action {
	case vm.ActDuplicate:
		e.Prompt = &Prompt{Title: "Duplicate", Label: "Path", Initial: n.Path + "-copy"}
	case vm.ActRename:
		// Rename asks for a name only; Move to… changes the folder or collection.
		title := "Rename request"
		if n.Kind == vm.NodeFolder {
			title = "Rename folder"
		}
		e.Prompt = &Prompt{Title: title, Label: "Name", Initial: path.Base(n.Path)}
		e.Guard = "dirty"
	case vm.ActDelete:
		q := s.deleteQuestion(n, ids)
		e.Confirm, e.ConfirmTitle, e.ConfirmAction, e.Guard = q.Text, q.Title, q.Action, "dirty"
	case vm.ActNewRequest, vm.ActNewGraphQLRequest, vm.ActNewGRPCRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where)}
	case vm.ActNewAsyncRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where), Protocols: protocolChoices()}
	case vm.ActNewDatasetRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where), Protocols: databasePromptChoices()}
	case vm.ActNewDataFileRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where), Protocols: dataFilePromptChoices()}
	case vm.ActNewFolder:
		e.Prompt = &Prompt{Title: "New folder", Label: nameLabel(where)}
	case vm.ActNewEnvironment:
		e.Prompt = &Prompt{Title: "New environment", Label: "Name"}
	}
	return e
}

func menuLabel(item vm.MenuItem) string {
	switch item.Action {
	case vm.ActOpen:
		return "Open"
	case vm.ActDuplicate:
		return "Duplicate…"
	case vm.ActRename:
		return "Rename…"
	case vm.ActDelete:
		if item.Count > 1 {
			return fmt.Sprintf("Delete %d…", item.Count)
		}
		return "Delete…"
	case vm.ActCopyMermaid:
		return "Copy as Mermaid"
	case vm.ActNewRequest:
		return "New request…"
	case vm.ActNewAsyncRequest:
		return "New async request…"
	case vm.ActNewGraphQLRequest:
		return "New GraphQL request…"
	case vm.ActNewGRPCRequest:
		return "New gRPC request…"
	case vm.ActNewDatasetRequest:
		return "New database query…"
	case vm.ActNewDataFileRequest:
		return "New data file query…"
	case vm.ActNewEnvironment:
		return "New environment…"
	case vm.ActNewFolder:
		return "New folder…"
	case vm.ActNewFlow:
		return "New flow"
	case vm.ActFlowFromRequests:
		return fmt.Sprintf("New flow from %d requests", item.Count)
	}
	return ""
}

func protocolLabels() map[string]string {
	return map[string]string{
		model.ProtocolWS: "WebSocket", model.ProtocolSSE: "Server-Sent Event", model.ProtocolMQTT: "MQTT", model.ProtocolKafka: "Kafka",
		model.ProtocolAMQP: "AMQP (RabbitMQ)", model.ProtocolAMQP10: "AMQP 1.0 (Artemis, Qpid…)", model.ProtocolNATS: "NATS", model.ProtocolRedis: "Redis", model.ProtocolSQS: "AWS SQS",
		model.ProtocolSNS: "AWS SNS", model.ProtocolServiceBus: "Azure Service Bus", model.ProtocolPubSub: "Google Pub/Sub",
	}
}

func protocolChoices() []Choice {
	var out []Choice
	for _, p := range model.AsyncProtocols() {
		label := protocolLabels()[p]
		if label == "" {
			label = p
		}
		out = append(out, Choice{ID: p, Path: p, Label: label})
	}
	return out
}

// folderPrefix is where an item made from row n goes: inside the folder
// clicked ("auth/"), else at the top of its tree.
func folderPrefix(n vm.TreeNode) (prefix, where string) {
	if n.Kind == vm.NodeFolder {
		return n.Path + "/", n.Path + "/"
	}
	return "", ""
}

func nameLabel(where string) string {
	if where == "" {
		return "Name"
	}
	return fmt.Sprintf("Name (in %s)", where)
}

// deleteQuestion says what deleting the rows takes with it, and what it leaves broken: the flow steps that run a
// request that goes.
func (s *WorkspaceService) deleteQuestion(n vm.TreeNode, ids []string) DeleteQuestion {
	ws := s.app.workspace
	uses := s.usesOfDeleted(ids)
	if n.Kind == vm.NodeFolder {
		requests, flows := ws.ContentsOf(n.ID)
		var what []string
		if requests > 0 {
			what = append(what, fmt.Sprintf("%d requests", requests))
		}
		if flows > 0 {
			what = append(what, fmt.Sprintf("%d flows", flows))
		}
		if len(what) == 0 {
			return DeleteQuestion{} // an empty folder just goes
		}
		text := fmt.Sprintf("Delete the folder %s and everything in it (%s)? This cannot be undone.", ws.Display(n.ID), strings.Join(what, ", "))
		return withUses(DeleteQuestion{Text: text}, uses, "")
	}
	var envs, effects []string
	others := 0
	for _, id := range ids {
		if ws.Node(id).Kind == vm.NodeEnvironment {
			envs = append(envs, ws.Display(id))
			if impact, err := s.app.backend.EnvironmentImpact(ws.Node(id).File); err == nil {
				if text := envEffects(ws.Display(id), impact); text != "" {
					effects = append(effects, text)
				}
			}
		} else {
			others++
		}
	}
	return withUses(DeleteQuestion{Text: leafDeleteQuestion(envs, others, effects)}, uses, " Undo brings the request back.")
}

// usesOfDeleted lists the flow steps, outside everything being deleted, that run a request among rows ids (or inside
// a folder among them). Steps in a flow that goes with them are not a problem: they go too.
func (s *WorkspaceService) usesOfDeleted(ids []string) []deleteUse {
	ws := s.app.workspace
	names := map[flowi.RequestTarget]string{}
	var targets []flowi.RequestTarget
	going := map[string]bool{}
	var visit func(id string)
	visit = func(id string) {
		switch n := ws.Node(id); n.Kind {
		case vm.NodeRequest:
			t := flowi.RequestTarget{Collection: n.Collection, Request: n.Request}
			names[t] = cmp.Or(n.Label, n.Path)
			targets = append(targets, t)
			going[n.File] = true
		case vm.NodeFlow:
			going[n.File] = true
		default:
			for _, child := range ws.Children(id) {
				visit(child)
			}
		}
	}
	for _, id := range ids {
		visit(id)
	}
	if len(targets) == 0 {
		return nil
	}
	var out []deleteUse
	callers := s.app.backend.Callers(targets)
	for _, t := range targets {
		for _, c := range callers[t] {
			if going[c.File] {
				continue
			}
			out = append(out, deleteUse{Request: names[t], Where: callerPlace(c)})
		}
	}
	return out
}

// Inside lists the request rows at or below a row: the tabs an action on it
// affects.
func (s *WorkspaceService) Inside(id string) []string {
	var out []string
	s.app.actor.Do(func() { out = append([]string{}, s.app.workspace.RequestsUnder(id)...) })
	return out
}

// Do runs a menu action on rows. text is what its prompt asked (a path or
// name) and protocol the async protocol. Results come back as events:
// "open" with a row to open, "notice" with a problem, "clipboard" with text
// to copy.
func (s *WorkspaceService) Do(action string, ids []string, text, protocol string) {
	s.app.actor.Do(func() { s.do(action, ids, strings.TrimSpace(text), protocol) })
}

func (s *WorkspaceService) do(action string, ids []string, text, protocol string) {
	ws, emit := s.app.workspace, s.app.emit
	if len(ids) == 0 {
		return
	}
	n := ws.Node(ids[0])
	open := func(node string) { emit("open", node) }
	fail := func(err error) { emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) }
	prefix, _ := folderPrefix(n)
	switch action {
	case "open":
		open(n.ID)
	case "duplicate":
		if n.Kind == vm.NodeFlow {
			ws.DuplicateFlow(flowi.FlowTarget{Collection: n.Collection, Flow: n.Flow}, text, open, fail)
		} else {
			ws.DuplicateRequest(flowi.RequestTarget{Collection: n.Collection, Request: n.Request}, text, open, fail)
		}
	case "rename":
		if text == "" || text == n.Path {
			return
		}
		to := renamedPath(n.Path, text)
		if to == n.Path {
			return
		}
		if n.Kind == vm.NodeFolder {
			ws.RenameFolder(n, to, fail)
		} else {
			ws.MoveRequest(flowi.RequestTarget{Collection: n.Collection, Request: n.Request}, n.Collection, to, open, fail)
		}
	case "delete":
		if n.Kind == vm.NodeFolder {
			ws.DeleteFolder(n, fail)
		} else {
			ws.Delete(ids, func([]string) {}, func(items []flowi.TrashedItem) {
				if d := deletedNotice(items); len(d.IDs) > 0 {
					emit("deleted", d)
				}
			}, fail)
		}
	case "mermaid":
		ws.FlowMermaid(flowi.FlowTarget{Collection: n.Collection, Flow: n.Flow}, func(chart string) { emit("clipboard", chart) }, fail)
	case "newRequest":
		ws.CreateRequest(n.Collection, prefix+text, open, fail)
	case "newAsync":
		ws.CreateAsyncRequest(n.Collection, prefix+text, protocol, open, fail)
	case "newGraphQL":
		ws.CreateGraphQLRequest(n.Collection, prefix+text, open, fail)
	case "newGRPC":
		ws.CreateGRPCRequest(n.Collection, prefix+text, open, fail)
	case "newDataset", "newDataFile":
		ws.CreateDatasetRequest(n.Collection, prefix+text, protocol, open, fail)
	case "newFolder":
		tree := n.Tree
		if tree == "" {
			tree = "requests"
		}
		ws.CreateFolder(n.Collection, tree, prefix+text, fail)
	case "newEnvironment":
		if text != "" {
			ws.CreateEnvironment(text, open, fail)
		}
	case "newFlow":
		ws.CreateUntitledFlowIn(n.Collection, strings.TrimSuffix(prefix, "/"), open, fail)
	case "flowFromRequests":
		requests := ids
		if len(ids) == 1 {
			requests = ws.RequestsUnder(n.ID)
		}
		ws.FlowFromRequests(n.Collection, "", requests, open, fail)
	default:
		emit("notice", Notice{Level: "info", Text: fmt.Sprintf("%s is not available in the new window yet.", action)})
	}
}

// NewScratchRequest creates an "untitled" request in the Scratch draft and
// asks the window to open it: the quickest way to try an API.
func (s *WorkspaceService) NewScratchRequest() {
	s.app.actor.Do(func() {
		s.app.workspace.NewScratchRequest(
			func(node string) { s.app.emit("open", node) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// NewScratchFlow creates an "untitled" flow in the Scratch draft and asks the
// window to open it.
func (s *WorkspaceService) NewScratchFlow() {
	s.app.actor.Do(func() {
		s.app.workspace.CreateUntitledFlow("",
			func(node string) { s.app.emit("open", node) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// NewIn creates a request, async request, GraphQL request, gRPC request, dataset query or flow at path in a
// collection (by its tree row), then asks the window to open it. kind is
// request, async, graphql, grpc, dataset or flow; protocol is for async, and the database for a dataset query.
func (s *WorkspaceService) NewIn(kind, collectionRow, path, protocol string) {
	s.app.actor.Do(func() {
		n := s.app.workspace.Node(collectionRow)
		open := func(node string) { s.app.emit("open", node) }
		fail := func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) }
		ws := s.app.workspace
		switch kind {
		case "request":
			ws.CreateRequest(n.Collection, path, open, fail)
		case "async":
			ws.CreateAsyncRequest(n.Collection, path, protocol, open, fail)
		case "graphql":
			ws.CreateGraphQLRequest(n.Collection, path, open, fail)
		case "grpc":
			ws.CreateGRPCRequest(n.Collection, path, open, fail)
		case "dataset":
			ws.CreateDatasetRequest(n.Collection, path, protocol, open, fail)
		case "flow":
			ws.CreateFlow(n.Collection, path, open, fail)
		default:
			fail(fmt.Errorf("unknown kind %q", kind))
		}
	})
}
