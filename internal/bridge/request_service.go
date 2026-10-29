// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/serve/replay"

	"github.com/githugt/flowi/internal/vm"
)

// ResponseView is what a request screen shows of its latest send.
type ResponseView struct {
	Title    string        `json:"title"`
	At       string        `json:"at"`
	Took     string        `json:"took"` // how long the send took, "" when not measured
	Size     string        `json:"size"` // the body's size, "" when unknown
	Passed   bool          `json:"passed"`
	Status   string        `json:"status"` // the engine's status: passed, failed, error…
	Code     string        `json:"code"`   // what the Response tab shows after its name: the HTTP status, the gRPC code, or the engine's status
	Sent     []PartView    `json:"sent"`
	Received []PartView    `json:"received"`
	Diags    []string      `json:"diags"`
	BodyJSON any           `json:"bodyJson,omitempty"` // the parsed JSON body, for the tree; absent for other bodies and large ones
	Phases   []PhaseView   `json:"phases"`
	Messages []MessageView `json:"messages"`
}

// RequestView is an open request as its screen shows it.
type RequestView struct {
	Node           string               `json:"node"`
	Name           string               `json:"name"`
	Path           string               `json:"path"`
	Kind           string               `json:"kind"` // http, async or grpc
	Verb           string               `json:"verb"` // GET, POST …, or the protocol (MQTT, GRPC …): what the tab and pickers colour
	Versions       []string             `json:"versions"`
	VersionNames   []string             `json:"versionNames"` // each version's title, "" when it has none; in the order of Versions
	Selected       string               `json:"selected"`
	Default        string               `json:"default"`
	Dirty          bool                 `json:"dirty"`
	OnDisk         bool                 `json:"onDisk"` // the file changed on disk
	NeedsDrop      bool                 `json:"needsDrop"`
	Error          string               `json:"error,omitempty"`
	Current        model.RequestVersion `json:"current"`
	FullURL        string               `json:"fullUrl"`    // the URL with its query rows, as typed in the URL bar
	ReplayPath     string               `json:"replayPath"` // the path the replayer answers this request on (HTTP only)
	HookChain      []string             `json:"hookChain"`
	BodyKind       string               `json:"bodyKind"`   // none, text, form or file
	BodyFormat     string               `json:"bodyFormat"` // json, xml, yaml or raw
	Multipart      bool                 `json:"multipart"`
	Auth           AuthView             `json:"auth"`
	Sending        bool                 `json:"sending"`
	SendError      string               `json:"sendError,omitempty"`
	Response       *ResponseView        `json:"response,omitempty"`
	Snippet        SnippetView          `json:"snippet"`
	Pipeline       []PipelineRowView    `json:"pipeline"`
	Live           []MessageView        `json:"live"`           // messages received so far by a running request
	ShowingHistory bool                 `json:"showingHistory"` // the response is an earlier exchange, not the latest send
	InScratch      bool                 `json:"inScratch"`      // a try-out request in the Scratch draft: saving it asks where to keep it
	Async          *AsyncInfo           `json:"async,omitempty"`
	GRPC           *GRPCInfo            `json:"grpc,omitempty"`

	// Parts are the parts of a multipart form body, for its editor.
	Parts []MultipartPartView `json:"parts"`

	// JSONRPC is set when the JSON body is a JSON-RPC 2.0 call: the editor then offers its method and id.
	JSONRPC *JSONRPCView `json:"jsonrpc,omitempty"`

	FlowiOverlay      string             `json:"flowiOverlay"`      // the flowi overlay laid over the version ("" for none)
	FlowiOverlays     []FlowiOverlayView `json:"flowiOverlays"`     // those of the collection
	FlowiOverlayNotes []string           `json:"flowiOverlayNotes"` // what the chosen one does not fit or has drifted from
	FlowiOverlayMarks []FlowiOverlayMark `json:"flowiOverlayMarks"` // the header and query rows the chosen overlays set
}

type openRequest struct {
	req    *vm.Request
	snip   *vm.Snippet
	format vm.ViewFormat
	shown  *vm.Exchange // an exchange of the history the user opened, instead of the latest send
}

// RequestService is the set of requests open in the window.
type RequestService struct {
	app  *App
	reqs map[string]*openRequest // by tree node id; used on the actor only
}

// Open opens the request of a tree node, or returns the one already open.
// Every change to it, and every send of it, publishes the "request" event.
func (s *RequestService) Open(node string) (RequestView, error) {
	var v RequestView
	var err error
	s.app.actor.Do(func() { v, err = s.open(node) })
	return v, err
}

func (s *RequestService) open(node string) (RequestView, error) {
	if o, ok := s.reqs[node]; ok {
		return s.view(node, o), nil
	}
	n := s.app.workspace.Node(node)
	if n.Kind != vm.NodeRequest {
		return RequestView{}, fmt.Errorf("%q is not a request", node)
	}
	c, ok := s.app.workspace.Collection(n.Collection)
	if !ok {
		return RequestView{}, fmt.Errorf("collection of %q is not loaded", node)
	}
	i := slices.IndexFunc(c.Requests, func(r model.Request) bool { return r.ID == n.Request })
	if i < 0 {
		return RequestView{}, fmt.Errorf("request of %q not found", node)
	}
	req := vm.NewRequest(s.app.backend, s.app.worker, c, c.Requests[i])
	req.SetScheduler(s.app.scheduler)
	o := &openRequest{req: req, format: vm.ViewJSON, snip: vm.NewSnippet(s.app.backend, s.app.worker, s.app.workspace, req, true)}
	req.OnChange(func() {
		s.app.workspace.SetLabel(node, untitledName(s.app.workspace, node, req))
		s.app.emit("request", s.view(node, o))
	})
	req.OnEdit(o.snip.Refresh)
	o.snip.OnChange(func() { s.app.emit("request", s.view(node, o)) })
	o.snip.Refresh()
	s.reqs[node] = o
	// The tab's first view lists the overlays: an event sent after it could be lost while the window is still starting.
	if list, err := s.app.backend.Overlays(c.Manifest.ID); err == nil {
		req.SetOverlays(list)
	}
	s.app.send.LoadSaved(req)
	return s.view(node, o), nil
}

// Close forgets the open request of a tree node; unsaved edits stay in its
// autosaved working copy.
func (s *RequestService) Close(node string) {
	s.app.actor.Do(func() {
		if o, ok := s.reqs[node]; ok {
			o.req.Abandon()
			o.snip.Close()
			delete(s.reqs, node)
		}
	})
}

func (s *RequestService) with(node string, f func(o *openRequest)) {
	s.app.actor.Do(func() {
		if o, ok := s.reqs[node]; ok {
			f(o)
		}
	})
}

// Edit replaces the selected version's content (its label stays). The
// request autosaves its working copy.
func (s *RequestService) Edit(node string, v model.RequestVersion) {
	s.with(node, func(o *openRequest) {
		o.req.Edit(func(cur *model.RequestVersion) {
			v.Label = cur.Label
			*cur = v
		})
	})
}

// SelectVersion shows another version.
func (s *RequestService) SelectVersion(node, label string) {
	s.with(node, func(o *openRequest) { o.req.SelectVersion(label) })
}

// SetDefault makes the selected version the default.
func (s *RequestService) SetDefault(node string) {
	s.with(node, func(o *openRequest) { o.req.SetDefault() })
}

// NewVersion adds a version copied from the selected one.
func (s *RequestService) NewVersion(node, label string) error {
	var err error
	s.with(node, func(o *openRequest) { err = o.req.NewVersionFromCurrent(label) })
	return err
}

// DeleteVersion removes the selected version from the request file. It reports
// true, deleting nothing, when the file has comments the rewrite would lose:
// ask, then call it again with dropComments.
func (s *RequestService) DeleteVersion(node string, dropComments bool) (needsConfirm bool, err error) {
	done := make(chan error, 1)
	s.with(node, func(o *openRequest) { o.req.DeleteVersion(dropComments, func(err error) { done <- err }) })
	err = <-done // the vm answers on the actor, after the save worker is done; this call is not on the actor
	if vm.IsDropsComments(err) {
		return true, nil
	}
	return false, err
}

// Save writes the request to its file.
func (s *RequestService) Save(node string) {
	s.with(node, func(o *openRequest) { o.req.Save(false) })
}

// Revert reloads the request from disk, dropping edits.
func (s *RequestService) Revert(node string) { s.with(node, func(o *openRequest) { o.req.Revert() }) }

// DiscardEdit throws away the working copy of the selected version.
func (s *RequestService) DiscardEdit(node string) {
	s.with(node, func(o *openRequest) { o.req.DiscardEdit() })
}

// Send sends the request in the selected environment. hooks is all, none,
// skip-pre or skip-post.
func (s *RequestService) Send(node, hooks string) {
	s.with(node, func(o *openRequest) {
		if hooks == "" {
			hooks = vm.HooksAll
		}
		s.app.send.Send(o.req, vm.SendOptions{Hooks: hooks})
	})
}

// Cancel stops a send in progress.
func (s *RequestService) Cancel(node string) {
	s.with(node, func(*openRequest) { s.app.send.Cancel() })
}

// SetFormat chooses how response bodies are shown: json, text or raw.
func (s *RequestService) SetFormat(node, format string) {
	s.with(node, func(o *openRequest) {
		o.format = map[string]vm.ViewFormat{"json": vm.ViewJSON, "text": vm.ViewText, "raw": vm.ViewRaw}[format]
		s.app.emit("request", s.view(node, o))
	})
}

// view builds the RequestView; call it on the actor.
func (s *RequestService) view(node string, o *openRequest) RequestView {
	r := o.req
	d := r.Draft()
	t := r.Target()
	kind := "http"
	switch {
	case r.IsAsync():
		kind = "async"
	case r.IsGRPC():
		kind = "grpc"
	}
	v := RequestView{
		Node: node, Name: d.Name, Path: d.Path, Kind: kind, Verb: vm.Verb(r.Current()), Versions: r.Labels(), VersionNames: versionNamesOf(d), Selected: r.SelectedLabel(), Default: d.Default,
		BodyKind: string(r.BodyKind()), BodyFormat: string(r.BodyFormat()), Multipart: r.Multipart(), Auth: authViewOf(r),
		Dirty: r.Dirty(), OnDisk: r.ChangedOnDisk(), NeedsDrop: r.NeedsDropConfirm(), Error: r.Err(), Current: r.Current(), FullURL: vm.URLWithQuery(r.Current().URL, r.Current().Query), HookChain: nonNil(r.HookChain()),
		InScratch: s.app.workspace.InScratch(node), Snippet: snippetViewOf(o.snip), Pipeline: pipelineOf(r, s.app.send.TraceFor(t)), Live: liveOf(s.app.send.TraceFor(t)), Async: asyncInfoOf(r), GRPC: grpcInfoOf(r), Sending: s.app.send.RunningFor(t), SendError: s.app.send.ErrFor(t),
	}
	v.FlowiOverlay, v.FlowiOverlays, v.FlowiOverlayNotes = r.OverlayName(), flowiOverlaysOf(r), nonNil(r.OverlayNotes())
	v.FlowiOverlayMarks = flowiOverlayMarksOf(r)
	v.Parts = multipartPartsOf(r)
	if call, ok := r.JSONRPC(); ok {
		v.JSONRPC = &JSONRPCView{Method: call.Method, ID: call.ID}
	}
	if kind == "http" {
		v.ReplayPath = replay.PathOf(r.Current().URL)
	}
	var last *vm.Exchange
	if ex, ok := s.app.send.LastFor(t); ok {
		last = &ex
	}
	hasPost := slices.ContainsFunc(v.Pipeline, func(p PipelineRowView) bool { return p.Kind == "section" && strings.EqualFold(p.Title, postHooksTitle) })
	v.Pipeline = append(v.Pipeline, builtInRows(schemasOn(s.app.backend, t.Collection, r.Current()), last, hasPost)...)
	switch ex, ok := s.app.send.LastFor(t); {
	case o.shown != nil:
		v.Response = responseOf(*o.shown, o.format)
		v.Response.Title = "History · " + v.Response.Title
		v.ShowingHistory = true
	case ok:
		v.Response = responseOf(ex, o.format)
	}
	return v
}

// maxTreeBody is the largest JSON body sent to the window as a tree: bigger ones are shown as text.
const maxTreeBody = 1 << 20

func responseOf(ex vm.Exchange, f vm.ViewFormat) *ResponseView {
	rv := &ResponseView{
		Title: ex.Title, At: ex.At.Local().Format(time.TimeOnly), Passed: ex.Result.Status == engine.StatusPassed,
		Status: string(ex.Result.Status), Sent: []PartView{}, Received: []PartView{}, Diags: []string{}, Phases: []PhaseView{}, Messages: []MessageView{},
	}
	rv.Diags = append(rv.Diags, vm.DiagLines(ex.Result.Diags)...)
	rv.Code = string(ex.Result.Status)
	if rec := ex.Result.Record; rec != nil {
		rv.Code = codeOf(rec, rv.Code)
		detail := vm.NodeDetail{Status: "sent", Record: rec}
		if rec.Body.JSON != nil && rec.Body.Size <= maxTreeBody {
			rv.BodyJSON = rec.Body.JSON
		}
		rv.Phases = phasesOf(rec.Timings)
		rv.Took = vm.TookText(rec.Timings.Total)
		rv.Size = sizeOf(rec)
		rv.Messages = messagesOf(rec.Messages)
		rv.Sent = partsOf(vm.InputParts(model.Node{}, detail))
		rv.Received = partsOf(vm.OutputParts(model.Node{}, detail, f))
	}
	return rv
}

// SaveDroppingComments saves although the file's comments are lost.
func (s *RequestService) SaveDroppingComments(node string) {
	s.with(node, func(o *openRequest) { o.req.ConfirmDrop() })
}

// DeclineDrop gives up a save that would drop comments.
func (s *RequestService) DeclineDrop(node string) {
	s.with(node, func(o *openRequest) { o.req.DeclineDrop() })
}

// ExtractAsVariable adds a value of the response body (by JSONPath) to the
// version's extractions, and saves the request.
func (s *RequestService) ExtractAsVariable(node, name, path string) error {
	var err error
	s.with(node, func(o *openRequest) { err = o.req.ExtractAsVariable(name, path) })
	return err
}

// PinExample saves the response shown as a pinned example named name
// (empty: its status code) and returns the file written.
func (s *RequestService) PinExample(node, name string) (string, error) {
	type result struct {
		file string
		err  error
	}
	ch := make(chan result, 1)
	s.with(node, func(o *openRequest) {
		ex, ok := s.app.send.LastFor(o.req.Target())
		if !ok || ex.Result.Record == nil {
			ch <- result{err: errors.New("send the request first")}
			return
		}
		s.app.send.Pin(*ex.Result.Record, name, func(file string, err error) { ch <- result{file, err} })
	})
	r := <-ch
	return r.file, r.err
}

// ShowExchange shows an exchange of the response history in the request
// (by its id in the history); ShowLatest goes back to the latest send.
func (s *RequestService) ShowExchange(node string, id int) {
	s.with(node, func(o *openRequest) {
		for _, ex := range s.app.history.Items() {
			if ex.ID == id {
				o.shown = &ex
			}
		}
		s.app.emit("request", s.view(node, o))
	})
}

// ShowLatest goes back to the latest send after ShowExchange.
func (s *RequestService) ShowLatest(node string) {
	s.with(node, func(o *openRequest) {
		o.shown = nil
		s.app.emit("request", s.view(node, o))
	})
}

// SuggestSavePath is the path offered when a try-out request is kept: its
// name, or for an untitled one its URL as a single dashed name.
func (s *RequestService) SuggestSavePath(node string) string {
	var path string
	s.with(node, func(o *openRequest) {
		path = vm.DefaultSavePath(s.app.workspace.Node(node).Label, o.req.Current().URL)
	})
	return path
}

// SaveTo saves the open request (its pending edits too), then moves it to
// path in a collection (by its tree row) unless it is already there; the
// moved request opens in place of this tab ("open" event, then close it).
func (s *RequestService) SaveTo(node, collectionRow, path string) {
	s.with(node, func(o *openRequest) {
		to := s.app.workspace.Node(collectionRow).Collection
		target := o.req.Target()
		o.req.SaveThen(false, func() {
			if _, r, found := s.app.workspace.Request(target); found && to == target.Collection && r.Path == path {
				return // saved where it was
			}
			s.app.workspace.MoveRequest(target, to, path, func(newRow string) {
				s.app.emit("open", newRow)
				s.app.emit("close", node)
			}, func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
		})
	})
}

// untitledName is what names a try-out request in the tree and its tab: its
// URL without the scheme, as httpie does. "" when the request keeps its own name.
func untitledName(ws *vm.Workspace, node string, req *vm.Request) string {
	n := ws.Node(node)
	url := req.Current().URL
	if !ws.InDraft(node) || !strings.HasPrefix(n.Label, "untitled") || url == "" {
		return ""
	}
	for _, p := range []string{"https://", "http://"} {
		if rest, ok := strings.CutPrefix(url, p); ok {
			return rest
		}
	}
	return url
}

// sizeOf is the size of the body a send got back. Messages and gRPC calls have no single body, so they show none.
func sizeOf(rec *engine.Record) string {
	if rec.HasMessages() || rec.Kind == engine.KindGRPC {
		return ""
	}
	return vm.SizeText(rec.Body.Size)
}

// codeOf is the short status of a record: a gRPC code, an HTTP status, or the fallback (messages: the outcome).
func codeOf(rec *engine.Record, fallback string) string {
	switch {
	case rec.Kind == engine.KindGRPC:
		return rec.GRPCCode
	case rec.HasMessages():
		return rec.Outcome
	case rec.Status != 0:
		return strconv.Itoa(rec.Status)
	}
	return fallback
}

// versionNamesOf are the titles of the request's versions, in order.
func versionNamesOf(r model.Request) []string {
	out := make([]string, len(r.Versions))
	for i, v := range r.Versions {
		out[i] = v.Name
	}
	return out
}

// JSONRPCView is the method and id of the JSON-RPC call a JSON body holds.
type JSONRPCView struct {
	Method string `json:"method"`
	ID     string `json:"id"` // "" for a notification
}

// MultipartPartView is one part of a multipart body: text, or a file by its path.
type MultipartPartView struct {
	Name        string `json:"name"`
	File        bool   `json:"file"`
	Value       string `json:"value"` // the text, or the file's path
	ContentType string `json:"contentType"`
}

func multipartPartsOf(r *vm.Request) []MultipartPartView {
	out := []MultipartPartView{}
	if !r.Multipart() {
		return out
	}
	for _, p := range r.Parts() {
		out = append(out, MultipartPartView{Name: p.Name, File: p.File, Value: p.Value, ContentType: p.ContentType})
	}
	return out
}
