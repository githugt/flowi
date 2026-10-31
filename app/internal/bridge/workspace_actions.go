// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"fmt"
	"path"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
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
	Guard   string  `json:"guard,omitempty"`   // "dirty": refuse while a tab of these rows has unsaved edits
	Dialog  string  `json:"dialog,omitempty"`  // "move": the window asks where with its own dialog
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
		vm.ActNewFolder: "newFolder", vm.ActNewAsyncRequest: "newAsync", vm.ActNewGRPCRequest: "newGRPC", vm.ActNewDatasetRequest: "newDataset",
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
		e.Confirm, e.Guard = s.deleteQuestion(n, ids), "dirty"
	case vm.ActNewRequest, vm.ActNewGRPCRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where)}
	case vm.ActNewAsyncRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where), Protocols: protocolChoices()}
	case vm.ActNewDatasetRequest:
		e.Prompt = &Prompt{Title: strings.TrimSuffix(e.Label, "…"), Label: nameLabel(where), Protocols: driverPromptChoices()}
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
	case vm.ActNewGRPCRequest:
		return "New gRPC request…"
	case vm.ActNewDatasetRequest:
		return "New dataset query…"
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

// deleteQuestion says what deleting the rows takes with it.
func (s *WorkspaceService) deleteQuestion(n vm.TreeNode, ids []string) string {
	ws := s.app.workspace
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
			return "" // an empty folder just goes
		}
		return fmt.Sprintf("Delete the folder %s and everything in it (%s)? This cannot be undone.", ws.Display(n.ID), strings.Join(what, ", "))
	}
	const maxNamed = 5
	var names []string
	for _, id := range ids {
		names = append(names, ws.Display(id))
	}
	list := strings.Join(names[:min(len(names), maxNamed)], ", ")
	if more := len(names) - maxNamed; more > 0 {
		list += fmt.Sprintf(" and %d more", more)
	}
	return fmt.Sprintf("Delete %s? This cannot be undone.", list)
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
			ws.Delete(ids, func([]string) {}, fail)
		}
	case "mermaid":
		ws.FlowMermaid(flowi.FlowTarget{Collection: n.Collection, Flow: n.Flow}, func(chart string) { emit("clipboard", chart) }, fail)
	case "newRequest":
		ws.CreateRequest(n.Collection, prefix+text, open, fail)
	case "newAsync":
		ws.CreateAsyncRequest(n.Collection, prefix+text, protocol, open, fail)
	case "newGRPC":
		ws.CreateGRPCRequest(n.Collection, prefix+text, open, fail)
	case "newDataset":
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
		folder, _ := folderPrefix(n)
		ws.CreateUntitledFlowIn(n.Collection, strings.TrimSuffix(folder, "/"), open, fail)
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

// NewIn creates a request, async request, gRPC request, dataset query or flow at path in a
// collection (by its tree row), then asks the window to open it. kind is
// request, async, grpc, dataset or flow; protocol is for async, and the database for a dataset query.
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
