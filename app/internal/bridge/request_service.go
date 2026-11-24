// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"errors"
	"fmt"
	"mime"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi/lib/bugreport"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/respdiff"
	"github.com/githugt/flowi/lib/serve/replay"

	"github.com/githugt/flowi/app/internal/vm"
)

// ResponseView is what a request screen shows of its latest send.
type ResponseView struct {
	Title string `json:"title"`
	At    string `json:"at"`
	Took  string `json:"took"` // how long the send took, "" when not measured
	Size  string `json:"size"` // the body's size, "" when unknown
	// Facts is the line about how the response came ("HTTP/2 · gzip 12 KB → 340 KB · 2 redirects"), and Hops the redirects it
	// passed through, one line each.
	Facts        string          `json:"facts"`
	Hops         []string        `json:"hops"`
	Passed       bool            `json:"passed"`
	Status       string          `json:"status"` // the engine's status: passed, failed, error…
	Code         string          `json:"code"`   // what the Response tab shows after its name: the HTTP status, the gRPC code, or the engine's status
	Sent         []PartView      `json:"sent"`
	Received     []PartView      `json:"received"`
	Diags        []string        `json:"diags"`
	CheckResults []CheckEvalView `json:"checkResults"`       // what the send itself said about each check it ran, in order
	Checks       []string        `json:"checks"`             // the checks (assert lines) this response judged; ones added or edited since are untried
	Extracted    []ExtractedView `json:"extracted"`          // what each extraction the send tried took from the response; ones added or renamed since are absent
	BodyJSON     any             `json:"bodyJson,omitempty"` // the parsed JSON body, for the tree; absent for other bodies and large ones
	BodyXML      string          `json:"bodyXml,omitempty"`  // the body text of an XML response (not HTML), for the tree and the multi-status list; absent otherwise and when large
	Phases       []PhaseView     `json:"phases"`
	Messages     []MessageView   `json:"messages"`
	Table        *TableView      `json:"table,omitempty"`    // the rows of a dataset response
	BodyNote     string          `json:"bodyNote,omitempty"` // why no body shows: it was large and went to a file
	// BodyKind says what the body is, for picking a viewer: none, json, xml, html, image, pdf, text or binary; ""
	// for a response with no single body (messages, gRPC, a query). ContentType is the type it came as.
	BodyKind    string `json:"bodyKind"`
	ContentType string `json:"contentType,omitempty"`
	BodyImage   string `json:"bodyImage,omitempty"` // a picture as a data address, when it is small enough to draw
	BodyHTML    string `json:"bodyHtml,omitempty"`  // a page's source, for the sandboxed preview, when it is small enough
	BodyHex     string `json:"bodyHex,omitempty"`   // the first bytes of a binary body as a hex dump
}

// RequestView is an open request as its screen shows it.
type RequestView struct {
	Node           string               `json:"node"`
	Name           string               `json:"name"`
	Path           string               `json:"path"`
	Kind           string               `json:"kind"`  // http, async, grpc or dataset
	Type           string               `json:"type"`  // the id of its type in Types: http, grpc, async:ws, db:postgres, file:csv
	Types          []TypeChoice         `json:"types"` // every type it can be switched to
	Verb           string               `json:"verb"`  // GET, POST …, or the protocol (MQTT, GRPC …): what the tab and pickers colour
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
	Dataset        *DatasetInfo         `json:"dataset,omitempty"`
	Connection     *ConnectionView      `json:"connection,omitempty"` // for a message or database request: the Test connection button and its last result

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
	o := &openRequest{req: req, format: vm.ViewJSON, snip: vm.NewSnippet(s.app.backend, s.app.worker, s.app.workspace, req, s.snippetExcludesSecrets())}
	req.OnChange(func() {
		s.app.workspace.SetLabel(node, shownLabel(s.app.workspace, node, req))
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
	case r.IsDataset():
		kind = "dataset"
	}
	v := RequestView{
		Node: node, Name: d.Name, Path: d.Path, Kind: kind, Type: vm.TypeOf(r.Current()), Types: typeChoices(), Verb: vm.Verb(r.Current()), Versions: r.Labels(), VersionNames: versionNamesOf(d), Selected: r.SelectedLabel(), Default: d.Default,
		BodyKind: string(r.BodyKind()), BodyFormat: string(r.BodyFormat()), Multipart: r.Multipart(), Auth: authViewOf(r, s.app.workspace.NamedAuths()),
		Dirty: r.Dirty(), OnDisk: r.ChangedOnDisk(), NeedsDrop: r.NeedsDropConfirm(), Error: r.Err(), Current: emptyCollections(r.Current()), FullURL: vm.URLWithQuery(r.Current().URL, r.Current().Query), HookChain: nonNil(r.HookChain()),
		InScratch: s.app.workspace.InScratch(node), Snippet: snippetViewOf(o.snip), Pipeline: pipelineOf(r, s.app.send.TraceFor(t)), Live: liveOf(s.app.send.TraceFor(t)), Async: asyncInfoOf(r), GRPC: grpcInfoOf(r), Dataset: datasetInfoOf(r), Connection: connectionOf(r), Sending: s.app.send.RunningFor(t), SendError: s.app.send.ErrFor(t),
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

// ExtractedView is what one extraction took from the response a request got.
type ExtractedView struct {
	Name  string `json:"name"`
	Value string `json:"value"` // cut for a chip; "" when nothing matched
	Found bool   `json:"found"`
}

// maxTreeBody is the largest JSON body sent to the window as a tree: bigger ones are shown as text.
const maxTreeBody = 1 << 20

// isXMLType says whether a Content-Type is XML: text/xml, application/xml or a +xml type (WebDAV answers are
// application/xml). HTML is not: it is rarely well-formed and has no use for a tree.
func isXMLType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return false
	}
	return mediaType == "text/xml" || mediaType == "application/xml" || strings.HasSuffix(mediaType, "+xml")
}

func responseOf(ex vm.Exchange, f vm.ViewFormat) *ResponseView {
	rv := &ResponseView{
		Title: ex.Title, At: ex.At.Local().Format(time.TimeOnly), Passed: ex.Result.Status == engine.StatusPassed,
		Status: string(ex.Result.Status), Sent: []PartView{}, Received: []PartView{}, Diags: []string{}, Checks: append([]string{}, ex.Checks...), CheckResults: []CheckEvalView{}, Phases: []PhaseView{}, Messages: []MessageView{}, Hops: []string{}, Extracted: []ExtractedView{},
	}
	for _, e := range vm.ExtractedOf(ex) {
		rv.Extracted = append(rv.Extracted, ExtractedView{Name: e.Name, Value: e.Value, Found: e.Found})
	}
	rv.Diags = append(rv.Diags, vm.DiagLines(ex.Result.Diags)...)
	for _, c := range ex.Result.Checks {
		rv.CheckResults = append(rv.CheckResults, CheckEvalView{Line: c.Line, Verdict: c.Verdict, Note: c.Note})
	}
	rv.Code = string(ex.Result.Status)
	if rec := ex.Result.Record; rec != nil {
		rv.Code = codeOf(rec, rv.Code)
		detail := vm.NodeDetail{Status: "sent", Record: rec}
		if rec.Body.JSON != nil && rec.Body.Size <= maxTreeBody {
			rv.BodyJSON = rec.Body.JSON
		}
		if isXMLType(rec.Body.ContentType) && rec.Body.Text != "" && rec.Body.Size <= maxTreeBody {
			rv.BodyXML = rec.Body.Text
		}
		rv.Phases = phasesOf(rec.Timings)
		rv.Took = vm.TookText(rec.Timings.Total)
		rv.Size = sizeOf(rec)
		rv.Facts, rv.Hops = vm.ResponseFacts(rec), vm.RedirectLines(rec)
		rv.BodyNote = bodyNoteOf(rec)
		if singleBody(rec) {
			rv.BodyKind, rv.ContentType = string(vm.ResponseKindOf(rec.Body)), rec.Body.ContentType
			rv.BodyImage, rv.BodyHTML = vm.BodyImage(rec.Body), vm.BodyHTML(rec.Body)
			if rv.BodyKind == string(vm.RespBinary) || rv.BodyKind == string(vm.RespPDF) {
				rv.BodyHex = vm.BodyHex(rec.Body)
			}
		}
		rv.Messages = messagesOf(rec.Messages)
		rv.Table = tableOf(rec)
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
		rec := *ex.Result.Record
		s.app.send.Pin(rec, name, func(file string, err error) {
			if note := engine.PinWarning(rec); err == nil && note != "" {
				s.app.emit("notice", Notice{Level: "warn", Text: note})
			}
			ch <- result{file, err}
		})
	})
	r := <-ch
	return r.file, r.err
}

// ChangeView is one difference between two responses: a JSON path, "line 12" or a header name.
type ChangeView struct {
	Path string `json:"path"`
	Kind string `json:"kind"` // added, removed or changed
	Old  string `json:"old"`
	New  string `json:"new"`
}

// ResponseDiffView is how the response on screen differs from the one sent before it.
type ResponseDiffView struct {
	Older     string       `json:"older"` // when and what, "14:02:11 · 200"
	Newer     string       `json:"newer"`
	Same      bool         `json:"same"`
	OldStatus int          `json:"oldStatus"`
	NewStatus int          `json:"newStatus"`
	Headers   []ChangeView `json:"headers"`
	Body      []ChangeView `json:"body"`
	BodyMode  string       `json:"bodyMode"` // json (by path), text (by line), binary or ""
	Truncated int          `json:"truncated"`
	Hidden    int          `json:"hidden"`   // credential-named values left out of the comparison
	Volatile  int          `json:"volatile"` // headers that change on every send, skipped
}

func changeViews(cs []respdiff.Change) []ChangeView {
	out := make([]ChangeView, len(cs))
	for i, c := range cs {
		out[i] = ChangeView{Path: c.Path, Kind: string(c.Kind), Old: c.Old, New: c.New}
	}
	return out
}

// ComparePrevious compares the response the request shows with the one sent before it (the same request and
// version, from this session's history): the status, the headers, and the body by JSON path or by line.
func (s *RequestService) ComparePrevious(node string) (ResponseDiffView, error) {
	type result struct {
		view ResponseDiffView
		err  error
	}
	ch := make(chan result, 1)
	s.with(node, func(o *openRequest) {
		shown, ok := s.shownExchange(o)
		if !ok || shown.Result.Record == nil {
			ch <- result{err: errors.New("send the request first")}
			return
		}
		var prev *vm.Exchange
		for _, ex := range s.app.history.Items() {
			if ex.Target != shown.Target || ex.Version != shown.Version || ex.Result.Record == nil || (shown.ID != 0 && ex.ID >= shown.ID) {
				continue
			}
			if prev == nil || ex.ID > prev.ID {
				p := ex
				prev = &p
			}
		}
		if prev == nil {
			ch <- result{err: errors.New("there is no earlier response of this request to compare with")}
			return
		}
		d := respdiff.Compare(*prev.Result.Record, *shown.Result.Record, respdiff.Options{})
		label := func(ex vm.Exchange) string {
			return ex.At.Format("15:04:05") + " · " + strconv.Itoa(ex.Result.Record.Status)
		}
		ch <- result{view: ResponseDiffView{
			Older: label(*prev), Newer: label(shown), Same: d.Same(), OldStatus: d.OldStatus, NewStatus: d.NewStatus,
			Headers: changeViews(d.Headers), Body: changeViews(d.Body), BodyMode: d.BodyMode, Truncated: d.Truncated, Hidden: d.Hidden, Volatile: d.Volatile,
		}}
	})
	r := <-ch
	return r.view, r.err
}

// BugReportView is the Markdown of the last send for a bug report, and what was masked in it.
type BugReportView struct {
	Text   string   `json:"text"`
	Masked []string `json:"masked"`
}

// BugReport writes the request of the last send and its response as Markdown (a cURL line, the headers, and the
// bodies cut to a few KB) with every credential-named header, query parameter and JSON field masked.
func (s *RequestService) BugReport(node string) (BugReportView, error) {
	type result struct {
		view BugReportView
		err  error
	}
	ch := make(chan result, 1)
	s.with(node, func(o *openRequest) {
		ex, ok := s.app.send.LastFor(o.req.Target())
		if !ok || ex.Result.Record == nil {
			ch <- result{err: errors.New("send the request first")}
			return
		}
		rep := bugreport.Build(*ex.Result.Record, bugreport.Options{})
		ch <- result{view: BugReportView{Text: rep.Markdown, Masked: append([]string{}, rep.Masked...)}}
	})
	r := <-ch
	return r.view, r.err
}

// CheckEvalView is what one check says about the response on screen.
type CheckEvalView struct {
	Line    string `json:"line"`
	Verdict string `json:"verdict"` // passed, failed or unknown (it needs a send: a variable, the environment)
	Note    string `json:"note"`    // for a failed check what it saw; for an unknown one why it cannot be told
}

// CheckEvalsView is a list of checks tried on the response a request shows.
type CheckEvalsView struct {
	At     string          `json:"at"` // when that response was received, "14:02:11"; "" when there is none
	Checks []CheckEvalView `json:"checks"`
}

// EvalChecks tries the given checks on the response the request shows, with no request sent: a check being
// written is judged at once, and a POST that creates data does not create another record to try it.
func (s *RequestService) EvalChecks(node string, lines []string) CheckEvalsView {
	out := CheckEvalsView{Checks: []CheckEvalView{}}
	s.with(node, func(o *openRequest) {
		ex, ok := s.shownExchange(o)
		if !ok || ex.Result.Record == nil {
			return
		}
		out.At = ex.At.Local().Format(time.TimeOnly)
		for _, c := range engine.EvalChecks(*ex.Result.Record, lines) {
			out.Checks = append(out.Checks, CheckEvalView{Line: c.Line, Verdict: c.Verdict, Note: c.Note})
		}
	})
	return out
}

// shownExchange is the exchange whose response the request shows: one opened from the history, else the latest.
func (s *RequestService) shownExchange(o *openRequest) (vm.Exchange, bool) {
	if o.shown != nil {
		return *o.shown, true
	}
	return s.app.send.LastFor(o.req.Target())
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

// snippetExcludesSecrets is ui.snippet_exclude_secrets: what the "Leave secrets out" toggle of a request's code
// snippet starts as. It was always on, whatever the setting said.
func (s *RequestService) snippetExcludesSecrets() bool {
	v, ok := s.app.backend.Settings("").Get("ui.snippet_exclude_secrets")
	exclude, _ := v.Value.(bool)
	return ok && exclude
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
// the place a tree row stands for (a collection or a request folder), under
// the name text, unless it is already there; the
// moved request opens in place of this tab ("open" event, then close it).
func (s *RequestService) SaveTo(node, placeRow, text string) {
	s.with(node, func(o *openRequest) {
		to, folder, ok := placeDestination(s.app.workspace, placeRow, "requests")
		if !ok {
			s.app.emit("notice", Notice{Level: "error", Text: "Choose a collection or a folder."})
			return
		}
		dest := placePath(folder, text)
		target := o.req.Target()
		o.req.SaveThen(false, func() {
			if _, r, found := s.app.workspace.Request(target); found && to == target.Collection && r.Path == dest {
				return // saved where it was
			}
			s.app.workspace.MoveRequest(target, to, dest, func(newRow string) {
				if newRow == node {
					// Kept in its own collection under a new name: the row, and so the tab, are the same one. Opening
					// and closing it raced, and left a tab with nothing in it ("not ported to the new window yet").
					return
				}
				s.app.emit("open", newRow)
				s.app.emit("close", node)
			}, func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
		})
	})
}

// SuggestCopyName is the name offered for a copy of the open request: its own with "-copy".
func (s *RequestService) SuggestCopyName(node string) string {
	return path.Base(s.app.workspace.Node(node).Path) + "-copy"
}

// PlaceOf is the tree row of the folder the request is in (its collection's row at the top): the
// place a copy starts from.
func (s *RequestService) PlaceOf(node string) string {
	n := s.app.workspace.Node(node)
	row := "c:" + string(n.Collection)
	if dir := path.Dir(n.Path); dir != "." {
		row += "/requests/" + dir
	}
	return row
}

// SaveAsNew copies the open request, as last saved, to the place a tree row stands for (a
// collection or a request folder) under the name text, and opens the copy. The original and its
// tab stay as they are, unsaved edits included: they are not in the copy.
func (s *RequestService) SaveAsNew(node, placeRow, text string) {
	s.with(node, func(o *openRequest) {
		to, folder, ok := placeDestination(s.app.workspace, placeRow, "requests")
		if !ok || strings.TrimSpace(text) == "" {
			s.app.emit("notice", Notice{Level: "error", Text: "Choose a collection or a folder, and a name."})
			return
		}
		s.app.workspace.DuplicateRequestTo(o.req.Target(), to, placePath(folder, text),
			func(row string) { s.app.emit("open", row) },
			func(err error) { s.app.emit("notice", Notice{Level: "error", Text: vm.ErrorText(err)}) })
	})
}

// shownLabel is what the tree and the tab call the request while it is open: a try-out request's URL, else the name as
// typed (the file's last part when the name is cleared), so a name shows as it is written and not only once saved.
// "" when that is what the tree already shows.
//
// Before: a name typed in the request bar reached the sidebar and the tab only after Save.
func shownLabel(ws *vm.Workspace, node string, req *vm.Request) string {
	if u := untitledName(ws, node, req); u != "" {
		return u
	}
	d := req.Draft()
	want := d.Name
	if want == "" {
		want = path.Base(d.Path)
	}
	if want == ws.Node(node).Label {
		return ""
	}
	return want
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

// bodyNoteOf says where a large body went. A body over the stream threshold is written to a file and not kept in the
// record, so the Body tab has nothing to draw. Before, it was empty and nothing said why.
func bodyNoteOf(rec *engine.Record) string {
	if rec.Body.File == "" {
		return ""
	}
	return vm.SizeText(rec.Body.Size) + " saved to " + rec.Body.File + ". It is not shown here, and checks and extracts do not see it."
}

// sizeOf is the size of the body a send got back. Messages, gRPC calls and queries have no single body, so they show
// none: a query shows its rows.
func sizeOf(rec *engine.Record) string {
	if !singleBody(rec) {
		return ""
	}
	return vm.SizeText(rec.Body.Size)
}

// singleBody says whether the record has one response body: messages, gRPC calls and queries do not.
func singleBody(rec *engine.Record) bool {
	return !rec.HasMessages() && rec.Kind != engine.KindGRPC && rec.Kind != engine.KindDataset
}

// codeOf is the short status of a record: a gRPC code, an HTTP status, or the fallback (messages: the outcome).
func codeOf(rec *engine.Record, fallback string) string {
	switch {
	case rec.Kind == engine.KindGRPC:
		return rec.GRPCCode
	case rec.HasMessages(), rec.Kind == engine.KindDataset:
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
