// SPDX-License-Identifier: AGPL-3.0-or-later

// Package vmtest has test doubles for view-model and screen tests.
package vmtest

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/analysis"
	"github.com/githugt/flowi-lib/announce"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/flowioverlay"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/schemainfer"
	"github.com/githugt/flowi-lib/serve/replay"
	"github.com/githugt/flowi-lib/serve/scheduler"
	"github.com/githugt/flowi-lib/settings"

	"github.com/githugt/flowi/internal/vm"
)

// StoredEdit is one StoreEdit call.
type StoredEdit struct {
	Target  flowi.RequestTarget
	Base    string
	Overlay string
	Version model.RequestVersion
}

// Backend is an in-memory vm.Backend. Tests set its fields; Calls are
// recorded in Calls. mu guards every field: the worker Calls it from other
// goroutines.
type Backend struct {
	Mu          sync.Mutex         // guards every field; tests lock it to change state mid-test
	FieldList   []flowi.Completion // what ResponseFields returns
	Name        string             // the workspace name; "default-workspace" when empty
	Cols        []model.Collection
	MissingIDs  []model.ID
	Paths       map[model.ID]string
	Diags       diag.List
	Values      map[string]any // user settings layer
	ResetErr    error          // ResetUserSetting fails with it
	SetErr      error          // SetUserSetting fails with it
	Session     string         // SessionDir
	SecretList  []flowi.SecretInfo
	SaveErr     error
	NextSend    *Sending
	SendErr     error
	Changes     chan flowi.Change
	Calls       []string
	Saved       []model.Request
	SavedRecord *engine.Record             // what LastResponse and StoredRecord return
	Stored      []flowi.StoredResponse     // what StoredResponses returns
	Trash       []flowi.TrashedItem        // what ListTrash returns
	StaleDrafts []flowi.TrashedItem        // what the next TrashStaleDrafts moves (an item's Name is the id open is asked about)
	RunList     []flowi.RunSummary         // what FlowRuns returns
	RunDetails  map[string]flowi.RunDetail // what FlowRunDetail returns, by run id
	Compat      diag.List                  // what FlowCompatibility returns
	SchemaStore Schemas                    // what the schema calls work on
	SavedEnvs   []model.Environment
	CollEnvs    map[model.ID][]model.Environment // what CollectionEnvironments returns, by collection
	SendOpts    []flowi.SendOptions
	// Flowi overlays: OverlayList is what Overlays returns, SavedOverlays what SaveAsOverlay was
	// given, OverlayFiles the text OverlayText returns and WriteOverlayText keeps, and OverlayErr
	// makes the writes fail.
	OverlayList    []model.Overlay
	SavedOverlays  []flowi.SaveOverlayInput
	OverlayFiles   map[string]string
	OverlayErr     error
	OverlayExports map[string]flowi.OverlayExport // what ExportOverlayAsSpec returns, by overlay name
	// SecretsGate, when set, makes Secrets wait for a receive: tests use it
	// to change state while a call is in flight.
	SecretsGate    chan struct{}
	SavedFlows     []model.Flow
	Layout         model.Layout
	CompletionList []flowi.Completion
	NextRun        *Running
	Check          Checks // canned answers for coverage, spec impact, access and drift
	// RunFor, when set, answers each RunFlow from its options (a matrix needs a different outcome per cell).
	RunFor         func(flowi.RunOptions) (*Running, error)
	LastRunOptions flowi.RunOptions   // the options of the latest RunFlow
	RunOptionsLog  []flowi.RunOptions // the options of every RunFlow, in order
	ImpactReport   analysis.Report
	// SaveGate, when set, makes SaveFlow wait for it to close.
	SaveGate chan struct{}
	// Plan is what PreviewImport returns (with PreviewErr); ApplyErr is
	// ApplyImport's result and ApplyOpts records its options.
	Plan       flowi.ImportPlan
	PreviewErr error
	ApplyErr   error
	ApplyOpts  []flowi.ApplyOptions
	Previewed  [][]byte
	// PreviewedSources records every source PreviewImportSource saw.
	PreviewedSources []flowi.ImportSource

	ServeLog   *replay.Log // the fake server's traffic; a new log when nil
	ServeErr   error
	News       announce.Result // what Announcements returns
	NewsErr    error
	ServedWith []flowi.ServeOptions
	ServeWait  error // what the fake server's Wait returns after its context ends

	// StoreEdit and PreviewRequest: Edits records the slot writes, EditErr
	// fails them; PreviewResult is what previews return.
	Edits           []StoredEdit
	Moves           [][2]string // MoveRequest calls: \"collection/request\" to \"collection/path\"
	EditErr         error
	PreviewResult   engine.Preview
	PreviewOpts     []flowi.SendOptions
	PreviewVersions []model.RequestVersion
	// PreviewRequestErr fails PreviewRequest.
	PreviewRequestErr error
	// PreviewRealErr fails only the previews that ask for real secret values.
	PreviewRealErr error

	DraftIDs map[model.ID]bool
	NewErr   error // NewRequest, NewFlow and NewScratchRequest fail with it
	SeedErr  error // SeedRequestHooks fails with it

	GRPCMethodList []grpcx.Method      // what GRPCMethods returns
	GRPCMethodsErr error               // GRPCMethods fails with it
	GeneratedGRPC  flowi.GeneratedGRPC // what GenerateGRPCRequests returns

	ScheduleList []model.Schedule        // Schedules lists these; the schedule calls change them
	ScheduleErr  error                   // AddSchedule, SaveSchedule and RemoveSchedule fail with it
	TriggerSet   bool                    // a trigger token exists
	TokenErr     error                   // NewTriggerToken fails with it
	ArazzoNotes  diag.List               // ExportArazzo lists these as what had no Arazzo equivalent
	ArazzoErr    error                   // ExportArazzo fails with it
	Kept         map[model.ID]model.Flow // KeepFlowEdits stores here, by flow
	ParsedFlow   *model.Flow             // ParseFlowSource returns this flow (else the stored flow, unchanged)
	SourceDiags  diag.List               // ParseFlowSource reports these
	DraftDiags   diag.List               // CheckFlowDraft reports these
	SecretErr    error                   // SetSecret and UnsetSecret fail with it
	SecretValues map[string]string       // RevealSecret answers from these, by "env/name"; others are FLW-SEC-001
	CookieJar    []flowi.CookieInfo      // Cookies lists these; DeleteCookie removes from them
	CookiesErr   error                   // Cookies fails with it
	// FailDeleteAt makes the nth (1-based) Delete* call fail with it, the others succeed.
	FailDeleteAt int
	// OnDelete, when set, runs inside every Delete* call (before it changes anything).
	OnDelete func()
	deletes  int
}

func (f *Backend) record(call string) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Calls = append(f.Calls, call)
}

// Called reports whether call was made.
func (f *Backend) Called(call string) bool {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Contains(f.Calls, call)
}

// SessionDir is empty unless a test sets Session.
func (f *Backend) SessionDir() string { return f.Session }

// WorkspaceName is "default-workspace" unless the test sets Name.
func (f *Backend) WorkspaceName() string {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Name == "" {
		return "default-workspace"
	}
	return f.Name
}

func (f *Backend) Collections() []model.Collection {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.Cols)
}

func (f *Backend) Missing() []model.ID {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.MissingIDs
}

func (f *Backend) MountedPaths() map[model.ID]string {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.Paths
}

func (f *Backend) Validate() diag.List {
	f.record("Validate")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.Diags
}

func (f *Backend) Settings(model.ID) settings.Resolved {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return settings.Resolve(settings.Schema(), settings.LayerValues{Layer: settings.LayerUser, Source: "settings.yaml", Values: f.Values})
}

func (f *Backend) SetUserSetting(key, raw string) error {
	f.record("SetUserSetting " + key + "=" + raw)
	if f.SetErr != nil {
		return f.SetErr
	}
	k, _ := settings.Lookup(settings.Schema(), key)
	v, err := settings.ParseValue(k, raw)
	if err != nil {
		return err
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Values == nil {
		f.Values = map[string]any{}
	}
	f.Values[key] = v
	return nil
}

func (f *Backend) ResetUserSetting(key string) error {
	f.record("ResetUserSetting " + key)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.ResetErr != nil {
		return f.ResetErr
	}
	delete(f.Values, key)
	return nil
}

func (f *Backend) SaveRequest(t flowi.RequestTarget, r model.Request, drop bool) error {
	f.record("SaveRequest")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SaveErr != nil && !drop {
		return f.SaveErr
	}
	f.Saved = append(f.Saved, r)
	for ci := range f.Cols {
		for ri := range f.Cols[ci].Requests {
			if f.Cols[ci].Requests[ri].ID == t.Request {
				f.Cols[ci].Requests[ri] = r
			}
		}
	}
	return nil
}

func (f *Backend) StoreEdit(t flowi.RequestTarget, base, overlay string, v model.RequestVersion) error {
	f.record("StoreEdit")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.EditErr != nil {
		return f.EditErr
	}
	f.Edits = append(f.Edits, StoredEdit{Target: t, Base: base, Overlay: overlay, Version: v})
	f.setEdit(t, &v, base)
	f.setEditOverlay(t, overlay)
	return nil
}

func (f *Backend) DiscardEdit(t flowi.RequestTarget) error {
	f.record("DiscardEdit")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.setEdit(t, nil, "")
	f.setEditOverlay(t, "")
	return nil
}

// setEditOverlay sets the overlay name of the edit slot of the request t; callers hold Mu.
func (f *Backend) setEditOverlay(t flowi.RequestTarget, overlay string) {
	for ci := range f.Cols {
		for ri := range f.Cols[ci].Requests {
			if f.Cols[ci].Requests[ri].ID == t.Request {
				f.Cols[ci].Requests[ri].EditOverlay = overlay
			}
		}
	}
}

// setEdit sets the edit slot of the request t; callers hold Mu.
func (f *Backend) setEdit(t flowi.RequestTarget, v *model.RequestVersion, base string) {
	for ci := range f.Cols {
		for ri := range f.Cols[ci].Requests {
			if f.Cols[ci].Requests[ri].ID == t.Request {
				f.Cols[ci].Requests[ri].Edit, f.Cols[ci].Requests[ri].EditBase = v, base
			}
		}
	}
}

func (f *Backend) PreviewRequest(_ flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions, include bool) (engine.Preview, error) {
	f.record("PreviewRequest")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.PreviewOpts = append(f.PreviewOpts, o)
	f.PreviewVersions = append(f.PreviewVersions, v)
	if f.PreviewRequestErr != nil {
		return engine.Preview{}, f.PreviewRequestErr
	}
	if include && f.PreviewRealErr != nil {
		return engine.Preview{}, f.PreviewRealErr
	}
	out := f.PreviewResult
	if !include {
		out.Real = nil
	}
	return out, nil
}

// MoveRequest moves the request between the fake's collections.
func (f *Backend) MoveRequest(t flowi.RequestTarget, to model.ID, path string) (flowi.RequestTarget, error) {
	f.record("MoveRequest")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	f.Moves = append(f.Moves, [2]string{string(t.Collection) + "/" + string(t.Request), string(to) + "/" + path})
	var moved model.Request
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID != t.Collection {
			continue
		}
		for ri, r := range f.Cols[ci].Requests {
			if r.ID == t.Request {
				moved = r
				f.Cols[ci].Requests = slices.Delete(slices.Clone(f.Cols[ci].Requests), ri, ri+1)
				break
			}
		}
	}
	moved.Path = path
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID == to {
			f.Cols[ci].Requests = append(slices.Clone(f.Cols[ci].Requests), moved)
		}
	}
	return flowi.RequestTarget{Collection: to, Request: t.Request}, nil
}

// DuplicateRequest adds a copy of the request to the fake's collection.
func (f *Backend) DuplicateRequest(t flowi.RequestTarget, path string) (flowi.RequestTarget, error) {
	f.record("DuplicateRequest " + path)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	copyID := t.Request + "-copy"
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID != t.Collection {
			continue
		}
		for _, r := range f.Cols[ci].Requests {
			if r.ID == t.Request {
				r.ID, r.Path, r.Name = copyID, path, path
				f.Cols[ci].Requests = append(slices.Clone(f.Cols[ci].Requests), r)
				break
			}
		}
	}
	return flowi.RequestTarget{Collection: t.Collection, Request: copyID}, nil
}

// DuplicateRequestTo records the call and adds the copy to the destination collection.
func (f *Backend) DuplicateRequestTo(t flowi.RequestTarget, to model.ID, path string) (flowi.RequestTarget, error) {
	f.record("DuplicateRequestTo " + string(to) + " " + path)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	copyID := t.Request + "-copy"
	var source model.Request
	for _, c := range f.Cols {
		if c.Manifest.ID != t.Collection {
			continue
		}
		for _, r := range c.Requests {
			if r.ID == t.Request {
				source = r
			}
		}
	}
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID == to {
			source.ID, source.Path, source.Name = copyID, path, path
			f.Cols[ci].Requests = append(slices.Clone(f.Cols[ci].Requests), source)
		}
	}
	return flowi.RequestTarget{Collection: to, Request: copyID}, nil
}

// deleteFailure counts a Delete* call and fails it when NewErr is set or it is
// the FailDeleteAt-th. Callers hold Mu.
func (f *Backend) deleteFailure() error {
	f.deletes++
	if f.OnDelete != nil {
		f.OnDelete()
	}
	if f.NewErr != nil || (f.FailDeleteAt > 0 && f.deletes == f.FailDeleteAt) {
		return errors.New("delete failed")
	}
	return nil
}

// DeleteRequest removes the request from the fake's collection.
func (f *Backend) DeleteRequest(t flowi.RequestTarget) error {
	f.record("DeleteRequest " + string(t.Request))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if err := f.deleteFailure(); err != nil {
		return err
	}
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID == t.Collection {
			f.Cols[ci].Requests = slices.DeleteFunc(slices.Clone(f.Cols[ci].Requests), func(r model.Request) bool { return r.ID == t.Request })
		}
	}
	return nil
}

// DuplicateFlow adds a copy of the flow to the fake's collection.
func (f *Backend) DuplicateFlow(t flowi.FlowTarget, path string) (flowi.FlowTarget, error) {
	f.record("DuplicateFlow " + path)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return flowi.FlowTarget{}, f.NewErr
	}
	copyID := t.Flow + "-copy"
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID != t.Collection {
			continue
		}
		for _, fl := range f.Cols[ci].Flows {
			if fl.ID == t.Flow {
				fl.ID, fl.Path = copyID, path
				f.Cols[ci].Flows = append(slices.Clone(f.Cols[ci].Flows), fl)
				break
			}
		}
	}
	return flowi.FlowTarget{Collection: t.Collection, Flow: copyID}, nil
}

// DeleteFlow removes the flow from the fake's collection.
func (f *Backend) DeleteFlow(t flowi.FlowTarget) error {
	f.record("DeleteFlow " + string(t.Flow))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if err := f.deleteFailure(); err != nil {
		return err
	}
	for ci := range f.Cols {
		if f.Cols[ci].Manifest.ID == t.Collection {
			f.Cols[ci].Flows = slices.DeleteFunc(slices.Clone(f.Cols[ci].Flows), func(fl model.Flow) bool { return fl.ID == t.Flow })
		}
	}
	return nil
}

// NewFolder records the call and adds the folder to the fake's collection.
func (f *Backend) NewFolder(collection model.ID, tree, path string) error {
	f.record("NewFolder " + tree + "/" + path)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return f.NewErr
	}
	for i := range f.Cols {
		if f.Cols[i].Manifest.ID == collection {
			f.Cols[i].Folders = append(f.Cols[i].Folders, model.Folder{Dir: tree + "/" + path})
		}
	}
	return nil
}

// SetFolderOrder records the call and keeps the order in the fake's folder.
func (f *Backend) SetFolderOrder(collection model.ID, tree, dir string, order []string, _ bool) error {
	f.record("SetFolderOrder " + strings.Trim(tree+"/"+dir, "/") + " " + strings.Join(order, ","))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	rel := strings.Trim(tree+"/"+dir, "/")
	for i := range f.Cols {
		if f.Cols[i].Manifest.ID != collection {
			continue
		}
		at := slices.IndexFunc(f.Cols[i].Folders, func(fo model.Folder) bool { return fo.Dir == rel })
		if at < 0 {
			f.Cols[i].Folders = append(f.Cols[i].Folders, model.Folder{Dir: rel})
			at = len(f.Cols[i].Folders) - 1
		}
		f.Cols[i].Folders[at].Order = slices.Clone(order)
	}
	return nil
}

// MoveFolder records the call.
func (f *Backend) MoveFolder(_ model.ID, tree, from, to string) error {
	f.record("MoveFolder " + tree + "/" + from + " -> " + to)
	return f.NewErr
}

// DeleteFolder records the call.
func (f *Backend) DeleteFolder(_ model.ID, tree, path string) error {
	f.record("DeleteFolder " + tree + "/" + path)
	return f.NewErr
}

// Schemas is what ListSchemas, SchemaFor and DeleteSchema work on, by part.
type Schemas = map[flowi.SchemaPart]schemainfer.Schema

// ListSchemas lists the parts of SchemaStore in a fixed order (request first).
func (f *Backend) ListSchemas(flowi.RequestTarget, string) []flowi.SchemaPart {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	var parts []flowi.SchemaPart
	for p := range f.SchemaStore {
		parts = append(parts, p)
	}
	slices.SortFunc(parts, func(a, b flowi.SchemaPart) int {
		if (a.Kind == "request") != (b.Kind == "request") {
			if a.Kind == "request" {
				return -1
			}
			return 1
		}
		return a.Status - b.Status
	})
	return parts
}

// SchemaFor returns a part of SchemaStore as an inferred schema.
func (f *Backend) SchemaFor(_ flowi.RequestTarget, _ string, part flowi.SchemaPart) (schemainfer.Schema, flowi.SchemaSource, bool) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	s, ok := f.SchemaStore[part]
	return s, flowi.SchemaInferred, ok
}

// DeleteSchema records the call and removes the part.
func (f *Backend) DeleteSchema(_ flowi.RequestTarget, _ string, part flowi.SchemaPart) error {
	f.record("DeleteSchema " + part.Kind + " " + strconv.Itoa(part.Status))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	delete(f.SchemaStore, part)
	return f.NewErr
}

// FlowCompatibility returns Compat.
func (f *Backend) FlowCompatibility(flowi.FlowTarget) diag.List {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.Compat
}

// FlowRunDetail returns the entry of RunDetails for the run id.
func (f *Backend) FlowRunDetail(_ flowi.FlowTarget, id string) (flowi.RunDetail, error) {
	f.record("FlowRunDetail " + id)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	d, ok := f.RunDetails[id]
	if !ok {
		return flowi.RunDetail{}, errors.New("no run " + id)
	}
	return d, nil
}

// FlowRuns returns RunList.
func (f *Backend) FlowRuns(flowi.FlowTarget, int) []flowi.RunSummary {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.RunList)
}

// TrashStaleDrafts records the call and moves Trash-able drafts: those of the fake's Stale list.
func (f *Backend) TrashStaleDrafts(olderThan time.Duration, open func(string, model.ID) bool) ([]flowi.TrashedItem, error) {
	f.record("TrashStaleDrafts " + olderThan.String())
	f.Mu.Lock()
	defer f.Mu.Unlock()
	var moved []flowi.TrashedItem
	for _, item := range f.StaleDrafts {
		if open != nil && open(item.Kind, model.ID(item.Name)) {
			continue
		}
		moved = append(moved, item)
		f.Trash = append(f.Trash, item)
	}
	f.StaleDrafts = nil
	return moved, nil
}

// ListTrash returns Trash.
func (f *Backend) ListTrash() ([]flowi.TrashedItem, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return append([]flowi.TrashedItem(nil), f.Trash...), nil
}

// RestoreFromTrash removes the item from Trash and returns its path.
func (f *Backend) RestoreFromTrash(id string) (string, error) {
	f.record("RestoreFromTrash " + id)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	for i, item := range f.Trash {
		if item.ID == id {
			f.Trash = slices.Delete(f.Trash, i, i+1)
			return item.Path, nil
		}
	}
	return "", errors.New("not in the trash")
}

// DeleteFromTrash removes the item from Trash.
func (f *Backend) DeleteFromTrash(id string) error {
	f.record("DeleteFromTrash " + id)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Trash = slices.DeleteFunc(f.Trash, func(i flowi.TrashedItem) bool { return i.ID == id })
	return nil
}

// EmptyTrash empties Trash.
func (f *Backend) EmptyTrash() error {
	f.record("EmptyTrash")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Trash = nil
	return nil
}

// StoredResponses returns Stored.
func (f *Backend) StoredResponses(flowi.RequestTarget) ([]flowi.StoredResponse, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return append([]flowi.StoredResponse(nil), f.Stored...), nil
}

// DeleteStoredResponse removes the response from Stored and remembers the call.
func (f *Backend) DeleteStoredResponse(_ flowi.RequestTarget, kind, version, name string) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Calls = append(f.Calls, "DeleteStoredResponse "+kind+" "+version+" "+name)
	f.Stored = slices.DeleteFunc(f.Stored, func(s flowi.StoredResponse) bool { return s.Kind == kind && s.Version == version && s.Name == name })
	return nil
}

// StoredRecord returns SavedRecord, when set.
func (f *Backend) StoredRecord(flowi.RequestTarget, string, string) (engine.Record, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SavedRecord == nil {
		return engine.Record{}, errors.New("no such saved response")
	}
	return *f.SavedRecord, nil
}

// LastResponse returns SavedRecord, when set.
func (f *Backend) LastResponse(flowi.RequestTarget, string) (engine.Record, bool) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SavedRecord == nil {
		return engine.Record{}, false
	}
	return *f.SavedRecord, true
}

// DeleteEnvironment removes the environment file from the fake's workspace.
func (f *Backend) DeleteEnvironment(file string) error {
	f.record("DeleteEnvironment " + file)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if err := f.deleteFailure(); err != nil {
		return err
	}
	for ci := range f.Cols {
		f.Cols[ci].Environments = slices.DeleteFunc(slices.Clone(f.Cols[ci].Environments), func(e model.Environment) bool { return e.File == file })
	}
	return nil
}

// Environments are the environments of the fake's collections, each file
// once (the real workspace shares one set between all collections).
func (f *Backend) Environments() []model.Environment {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	var out []model.Environment
	for _, c := range f.Cols {
		for _, e := range c.Environments {
			if !slices.ContainsFunc(out, func(x model.Environment) bool { return x.File == e.File }) {
				out = append(out, e)
			}
		}
	}
	return out
}

// ExportMermaid returns a fixed chart for any flow.
func (f *Backend) ExportMermaid(t flowi.FlowTarget) (string, error) {
	f.record("ExportMermaid " + string(t.Flow))
	return "flowchart TD\n", nil
}

// ExportArazzo returns a fixed document, with ArazzoNotes as what had no equivalent.
func (f *Backend) ExportArazzo(t flowi.FlowTarget) ([]byte, diag.List, error) {
	f.record("ExportArazzo " + string(t.Flow))
	return []byte("arazzo: 1.0.1\n"), f.ArazzoNotes, f.ArazzoErr
}

// FlowSource shows a flow as "nodes: <count>"; its Arazzo file is a fixed document.
func (f *Backend) FlowSource(t flowi.FlowTarget, fl model.Flow) (flowi.FlowText, error) {
	f.record("FlowSource " + string(t.Flow))
	return flowi.FlowText{Flow: fmt.Sprintf("nodes: %d\n", len(fl.Nodes)), File: "arazzo: 1.1.0\n"}, nil
}

// CheckFlowDraft reports DraftDiags.
func (f *Backend) CheckFlowDraft(t flowi.FlowTarget, _ model.Flow) diag.List {
	f.record("CheckFlowDraft " + string(t.Flow))
	return f.DraftDiags
}

// ParseFlowSource returns ParsedFlow, with SourceDiags as what is wrong with the text.
func (f *Backend) ParseFlowSource(t flowi.FlowTarget, _ string) (model.Flow, diag.List, error) {
	f.record("ParseFlowSource " + string(t.Flow))
	if f.ParsedFlow != nil {
		return *f.ParsedFlow, f.SourceDiags, nil
	}
	return model.Flow{}, f.SourceDiags, nil
}

func (f *Backend) NewRequest(collection model.ID, path string) (flowi.RequestTarget, error) {
	f.record("NewRequest " + path)
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	return flowi.RequestTarget{Collection: collection, Request: "new"}, nil
}

// NewAsyncRequest records the call and fails like NewRequest.
func (f *Backend) NewAsyncRequest(collection model.ID, path, protocol string) (flowi.RequestTarget, error) {
	f.record("NewAsyncRequest " + path + " " + protocol)
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	return flowi.RequestTarget{Collection: collection, Request: "new"}, nil
}

// NewGRPCRequest records the call and fails like NewRequest.
func (f *Backend) NewGRPCRequest(collection model.ID, path string) (flowi.RequestTarget, error) {
	f.record("NewGRPCRequest " + path)
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	return flowi.RequestTarget{Collection: collection, Request: "new"}, nil
}

// NewDatasetRequest records the call and fails like NewRequest.
func (f *Backend) NewDatasetRequest(collection model.ID, path, driver string) (flowi.RequestTarget, error) {
	f.record("NewDatasetRequest " + path + " " + driver)
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	return flowi.RequestTarget{Collection: collection, Request: "new"}, nil
}

// GenerateGRPCRequests records the call and returns GeneratedGRPC.
func (f *Backend) GenerateGRPCRequests(_ model.ID, base model.GRPC, methods []grpcx.Method) (flowi.GeneratedGRPC, error) {
	f.record("GenerateGRPCRequests " + base.Server + " " + strconv.Itoa(len(methods)))
	return f.GeneratedGRPC, f.NewErr
}

// GRPCMethods returns GRPCMethods, or GRPCMethodsErr when it is set.
func (f *Backend) GRPCMethods(_ context.Context, _ flowi.RequestTarget, _ model.RequestVersion, _ flowi.SendOptions) ([]grpcx.Method, error) {
	f.record("GRPCMethods")
	if f.GRPCMethodsErr != nil {
		return nil, f.GRPCMethodsErr
	}
	return f.GRPCMethodList, nil
}

// SeedRequestHooks records the call and fails with SeedErr when it is set.
func (f *Backend) SeedRequestHooks(t flowi.RequestTarget) error {
	f.record("SeedRequestHooks " + string(t.Request))
	return f.SeedErr
}

func (f *Backend) NewEnvironment(name string) (string, error) {
	f.record("NewEnvironment " + name)
	if f.NewErr != nil {
		return "", f.NewErr
	}
	return "/d/" + name + ".yaml", nil
}

func (f *Backend) SaveEnvironment(e model.Environment, _ bool) error {
	f.record("SaveEnvironment")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.SavedEnvs = append(f.SavedEnvs, e)
	return f.SaveErr
}

func (f *Backend) CollectionEnvironments(collection model.ID) ([]model.Environment, diag.List, error) {
	f.record("CollectionEnvironments " + string(collection))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.CollEnvs[collection]), nil, nil
}

func (f *Backend) Send(_ context.Context, _ flowi.RequestTarget, o flowi.SendOptions) (vm.Sending, error) {
	f.record("Send")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.SendOpts = append(f.SendOpts, o)
	if f.SendErr != nil {
		return nil, f.SendErr
	}
	return f.NextSend, nil
}

func (f *Backend) PinExample(_ engine.Record, name string) (string, error) {
	f.record("PinExample " + name)
	return "/c/requests/x.examples/" + name + ".json", nil
}

// Overlays returns the flowi overlays the test set up (OverlayList).
func (f *Backend) Overlays(model.ID) ([]model.Overlay, error) {
	f.record("Overlays")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.OverlayList), nil
}

// ApplyOverlay lays an overlay of OverlayList over v with the real rules.
func (f *Backend) ApplyOverlay(t flowi.RequestTarget, name string, v model.RequestVersion) (model.RequestVersion, []flowioverlay.Problem, error) {
	f.record("ApplyOverlay " + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	ops, missing := flowioverlay.Resolve(f.OverlayList, name, t.Request)
	if missing != "" {
		return v, nil, errors.New("no flowi overlay named " + missing)
	}
	out, problems := flowioverlay.Apply(v, ops)
	return out, problems, nil
}

// DiffForOverlay diffs against the saved version of the request the test set up.
func (f *Backend) DiffForOverlay(_ flowi.RequestTarget, base string, edited model.RequestVersion) (flowi.OverlayDiff, error) {
	f.record("DiffForOverlay " + base)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	for _, c := range f.Cols {
		for _, r := range c.Requests {
			for _, v := range r.Versions {
				if v.Label == base {
					ops, unsupported := flowioverlay.Diff(v, edited)
					return flowi.OverlayDiff{Ops: ops, Unsupported: unsupported}, nil
				}
			}
		}
	}
	return flowi.OverlayDiff{}, errors.New("no version " + base)
}

// SaveAsOverlay records the call.
func (f *Backend) SaveAsOverlay(_ flowi.RequestTarget, in flowi.SaveOverlayInput, _ bool) error {
	f.record("SaveAsOverlay " + in.Name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.SavedOverlays = append(f.SavedOverlays, in)
	return f.OverlayErr
}

// DeleteOverlay records the call.
func (f *Backend) DeleteOverlay(_ model.ID, name string) error {
	f.record("DeleteOverlay " + name)
	return nil
}

// OverlayText returns the overlay's file as OverlayFiles holds it.
func (f *Backend) OverlayText(_ model.ID, name string) (string, error) {
	f.record("OverlayText " + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.OverlayFiles[name], nil
}

// ExportOverlayAsSpec returns the export in OverlayExports under the overlay's name.
func (f *Backend) ExportOverlayAsSpec(_ model.ID, name string) (flowi.OverlayExport, error) {
	f.record("ExportOverlayAsSpec " + name)
	return f.OverlayExports[name], f.OverlayErr
}

// WriteOverlayText keeps the text in OverlayFiles.
func (f *Backend) WriteOverlayText(_ model.ID, name, text string) error {
	f.record("WriteOverlayText " + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.OverlayErr != nil {
		return f.OverlayErr
	}
	if f.OverlayFiles == nil {
		f.OverlayFiles = map[string]string{}
	}
	f.OverlayFiles[name] = text
	return nil
}

func (f *Backend) ClearCookies(collection string) error {
	f.record("ClearCookies " + collection)
	return nil
}

func (f *Backend) Cookies(collection string) ([]flowi.CookieInfo, error) {
	f.record("Cookies " + collection)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.CookieJar), f.CookiesErr
}

func (f *Backend) DeleteCookie(_, key string) error {
	f.record("DeleteCookie " + key)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.CookieJar = slices.DeleteFunc(f.CookieJar, func(c flowi.CookieInfo) bool { return c.Key == key })
	return nil
}

func (f *Backend) Schedules() []model.Schedule {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.ScheduleList)
}

func (f *Backend) AddSchedule(s model.Schedule) error {
	f.record("AddSchedule " + s.Name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.ScheduleErr != nil {
		return f.ScheduleErr
	}
	f.ScheduleList = append(f.ScheduleList, s)
	return nil
}

func (f *Backend) SaveSchedule(old string, s model.Schedule) error {
	f.record("SaveSchedule " + old + " " + s.Name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.ScheduleErr != nil {
		return f.ScheduleErr
	}
	if i := slices.IndexFunc(f.ScheduleList, func(x model.Schedule) bool { return x.Name == old }); i >= 0 {
		f.ScheduleList[i] = s
	}
	return nil
}

func (f *Backend) RemoveSchedule(name string) error {
	f.record("RemoveSchedule " + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.ScheduleErr != nil {
		return f.ScheduleErr
	}
	f.ScheduleList = slices.DeleteFunc(f.ScheduleList, func(x model.Schedule) bool { return x.Name == name })
	return nil
}

func (f *Backend) HasTriggerToken() (bool, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.TriggerSet, nil
}

func (f *Backend) NewTriggerToken() (string, error) {
	f.record("NewTriggerToken")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.TokenErr != nil {
		return "", f.TokenErr
	}
	f.TriggerSet = true
	return "tok-123", nil
}

func (f *Backend) Impact(flowi.RequestTarget, string) (analysis.Report, diag.List, error) {
	f.record("Impact")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.ImpactReport, nil, nil
}

func (f *Backend) Secrets() ([]flowi.SecretInfo, error) {
	if f.SecretsGate != nil {
		<-f.SecretsGate
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.SecretList), nil
}

func (f *Backend) SetSecret(env, name, _ string) error {
	f.record("SetSecret " + env + "/" + name)
	return f.markSecret(env, name, true)
}

func (f *Backend) RevealSecret(env, name string) (string, error) {
	f.record("RevealSecret " + env + "/" + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SecretErr != nil {
		return "", f.SecretErr
	}
	value, ok := f.SecretValues[env+"/"+name]
	if !ok {
		return "", diag.List{diag.New(diag.CodeSecretMissing, diag.Location{}, map[string]string{"name": name, "env": env})}.Err()
	}
	return value, nil
}

func (f *Backend) DiscardSecret(env, name string) error {
	f.record("DiscardSecret " + env + "/" + name)
	return f.markSecret(env, name, false)
}

func (f *Backend) UnsetSecret(env, name string) error {
	f.record("UnsetSecret " + env + "/" + name)
	return f.markSecret(env, name, false)
}

// markSecret stores or clears the listed secret, and fails with SecretErr
// when that is set.
func (f *Backend) markSecret(env, name string, set bool) error {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SecretErr != nil {
		return f.SecretErr
	}
	for i, s := range f.SecretList {
		if s.Environment == env && s.Name == name {
			f.SecretList[i].Set = set
		}
	}
	return nil
}

func (f *Backend) Mount(dir string) (model.Manifest, error) {
	f.record("Mount " + dir)
	return model.Manifest{}, nil
}

// Checks are the canned answers the fake gives for coverage, spec impact, access and drift.
type Checks struct {
	Coverage analysis.Coverage
	Impact   flowi.SpecImpact
	Specs    []string
	Access   flowi.AccessReport
	Drift    flowi.DriftReport
	Err      error
}

func (f *Backend) OpenArazzo(_ context.Context, ref string) error {
	f.record("OpenArazzo " + ref)
	return f.Check.Err
}

func (f *Backend) Coverage(collection model.ID) (analysis.Coverage, error) {
	f.record("Coverage " + string(collection))
	return f.Check.Coverage, f.Check.Err
}

func (f *Backend) ImpactAgainst(collection model.ID, name string, spec []byte) (flowi.SpecImpact, error) {
	f.record("ImpactAgainst " + string(collection) + " " + name + " " + string(spec))
	return f.Check.Impact, f.Check.Err
}

func (f *Backend) SpecNames(model.ID) []string { return f.Check.Specs }

func (f *Backend) CheckAccess(_ context.Context, _ flowi.FlowTarget, o flowi.AccessOptions) (flowi.AccessReport, error) {
	f.record("CheckAccess " + o.Owner)
	return f.Check.Access, f.Check.Err
}

func (f *Backend) CheckDrift(_ context.Context, _ flowi.FlowTarget, o flowi.DriftOptions) (flowi.DriftReport, error) {
	f.record("CheckDrift " + o.Env)
	return f.Check.Drift, f.Check.Err
}

func (f *Backend) OpenBundle(path string) error {
	f.record("OpenBundle " + path)
	return nil
}

func (f *Backend) Unmount(id model.ID) error {
	f.record("Unmount " + string(id))
	return nil
}

func (f *Backend) Relocate(id model.ID, dir string) error {
	f.record("Relocate " + string(id) + " " + dir)
	return nil
}

func (f *Backend) InitCollection(dir, name string) (model.Manifest, error) {
	f.record("InitCollection " + dir + " " + name)
	return model.Manifest{}, nil
}

func (f *Backend) PreviewImportSource(src flowi.ImportSource, collection model.ID) (flowi.ImportPlan, error) {
	f.record("PreviewImport " + string(src.Kind) + " " + string(collection))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Previewed = append(f.Previewed, src.Data)
	f.PreviewedSources = append(f.PreviewedSources, src)
	return f.Plan, f.PreviewErr
}

func (f *Backend) ApplyImport(p flowi.ImportPlan, o flowi.ApplyOptions) error {
	f.record("ApplyImport " + string(p.Collection))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.ApplyOpts = append(f.ApplyOpts, o)
	return f.ApplyErr
}

func (f *Backend) Watch(ctx context.Context) (<-chan flowi.Change, error) {
	f.record("Watch")
	if f.Changes == nil {
		return nil, nil
	}
	// Each watch gets its own channel fed from Changes until ctx ends.
	out := make(chan flowi.Change)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case ch, ok := <-f.Changes:
				if !ok {
					return
				}
				select {
				case out <- ch:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out, nil
}

// WatchCalls counts started watches.
func (f *Backend) WatchCalls() int {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	n := 0
	for _, c := range f.Calls {
		if c == "Watch" {
			n++
		}
	}
	return n
}

func (f *Backend) Reload(flowi.Change) (diag.List, error) {
	f.record("Reload")
	return nil, nil
}

// SaveFlow records the flow and replaces it in Cols.
func (f *Backend) SaveFlow(t flowi.FlowTarget, fl model.Flow, drop bool) error {
	f.record("SaveFlow")
	if f.SaveGate != nil {
		<-f.SaveGate
	}
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.SaveErr != nil && !drop {
		return f.SaveErr
	}
	f.SavedFlows = append(f.SavedFlows, fl)
	for ci := range f.Cols {
		for fi := range f.Cols[ci].Flows {
			if f.Cols[ci].Flows[fi].ID == t.Flow {
				f.Cols[ci].Flows[fi] = fl
			}
		}
	}
	return nil
}

func (f *Backend) NewFlow(collection model.ID, path string) (flowi.FlowTarget, error) {
	f.record("NewFlow " + path)
	if f.NewErr != nil {
		return flowi.FlowTarget{}, f.NewErr
	}
	return flowi.FlowTarget{Collection: collection, Flow: "new"}, nil
}

func (f *Backend) NewUntitledFlow(collection model.ID) (flowi.FlowTarget, error) {
	f.record("NewUntitledFlow " + string(collection))
	if f.NewErr != nil {
		return flowi.FlowTarget{}, f.NewErr
	}
	return flowi.FlowTarget{Collection: collection, Flow: "new"}, nil
}

func (f *Backend) NewFlowFromBlueprint(bp flowi.BlueprintTarget, collection model.ID, path string, values map[string]string) (flowi.FlowTarget, error) {
	call := "NewFlowFromBlueprint " + string(bp.Blueprint) + " " + path
	for _, k := range slices.Sorted(maps.Keys(values)) {
		call += " " + k + "=" + values[k]
	}
	f.record(call)
	if f.NewErr != nil {
		return flowi.FlowTarget{}, f.NewErr
	}
	return flowi.FlowTarget{Collection: collection, Flow: "new"}, nil
}

func (f *Backend) CreateBlueprint(flow flowi.FlowTarget, path, name string) (flowi.BlueprintTarget, error) {
	f.record("CreateBlueprint " + string(flow.Flow) + " " + path + " " + name)
	if f.NewErr != nil {
		return flowi.BlueprintTarget{}, f.NewErr
	}
	return flowi.BlueprintTarget{Collection: flow.Collection, Blueprint: "new"}, nil
}

func (f *Backend) CreateHookBlueprint(collection model.ID, path, name, phase string, nodes []model.Node) (flowi.BlueprintTarget, error) {
	f.record(fmt.Sprintf("CreateHookBlueprint %s %s %s %s %d", collection, path, name, phase, len(nodes)))
	if f.NewErr != nil {
		return flowi.BlueprintTarget{}, f.NewErr
	}
	return flowi.BlueprintTarget{Collection: collection, Blueprint: "new"}, nil
}

func (f *Backend) DeleteBlueprint(t flowi.BlueprintTarget) error {
	f.record("DeleteBlueprint " + string(t.Blueprint))
	return f.NewErr
}

func (f *Backend) Announcements(context.Context, bool) (announce.Result, error) {
	f.record("Announcements")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.News, f.NewsErr
}

func (f *Backend) ServeDefaults() flowi.ServeOptions {
	return flowi.ServeOptions{ReplayAddr: "127.0.0.1:8525", TriggerAddr: "127.0.0.1:8526", ShutdownTimeout: 30 * time.Second, MaxRuns: 4, TrafficLog: 1000}
}

func (f *Backend) Serve(ctx context.Context, o flowi.ServeOptions) (vm.Serving, error) {
	f.record("Serve " + o.ReplayAddr)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.ServedWith = append(f.ServedWith, o)
	if f.ServeErr != nil {
		return nil, f.ServeErr
	}
	if f.ServeLog == nil {
		f.ServeLog = replay.NewLog(100)
	}
	s := &Serving{addr: o.ReplayAddr, log: f.ServeLog, err: f.ServeWait, done: make(chan struct{})}
	go func() { <-ctx.Done(); close(s.done) }() // the fake server stops with its context
	return s, nil
}

// Serving is the fake server Backend.Serve returns.
type Serving struct {
	addr string
	log  *replay.Log
	err  error
	done chan struct{}
}

func (s *Serving) ReplayAddr() string  { return s.addr }
func (s *Serving) TriggerAddr() string { return "127.0.0.1:8526" }
func (s *Serving) Upcoming() []scheduler.Upcoming {
	return []scheduler.Upcoming{{Name: "nightly", At: time.Date(2026, 1, 2, 3, 0, 0, 0, time.UTC)}}
}
func (s *Serving) Notes() []string      { return []string{"a note"} }
func (s *Serving) Traffic() *replay.Log { return s.log }
func (s *Serving) Wait() error          { <-s.done; return s.err }

// ScratchID is the collection the fake's Scratch and NewScratchRequest use.
const ScratchID model.ID = "SCRATCH"

func (f *Backend) NewDraft(name string) (model.Manifest, error) {
	f.record("NewDraft " + name)
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.NewErr != nil {
		return model.Manifest{}, f.NewErr
	}
	m := model.Manifest{Meta: model.Meta{ID: model.ID("D-" + name), Name: name}, File: "/drafts/" + name + "/flowi.yaml"}
	f.Cols = append(slices.Clone(f.Cols), model.Collection{Root: "/drafts/" + name, Manifest: m})
	return m, nil
}

func (f *Backend) Scratch() (model.Manifest, error) {
	f.record("Scratch")
	return model.Manifest{Meta: model.Meta{ID: ScratchID, Name: "Scratch"}}, nil
}

func (f *Backend) NewScratchRequest() (flowi.RequestTarget, error) {
	f.record("NewScratchRequest")
	if f.NewErr != nil {
		return flowi.RequestTarget{}, f.NewErr
	}
	return flowi.RequestTarget{Collection: ScratchID, Request: "new"}, nil
}

func (f *Backend) IsDraft(id model.ID) bool {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return f.DraftIDs[id]
}

func (f *Backend) SaveDraft(id model.ID, dir string) error {
	f.record("SaveDraft " + string(id) + " " + dir)
	return nil
}

func (f *Backend) DiscardDraft(id model.ID) error {
	f.record("DiscardDraft " + string(id))
	return f.NewErr
}

func (f *Backend) FlowLayout(t flowi.FlowTarget) (model.Layout, diag.List, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Layout.Nodes == nil {
		return model.Layout{Flow: t.Flow, Nodes: map[string]model.Point{}}, nil, nil
	}
	return f.Layout, nil, nil
}

func (f *Backend) SaveFlowLayout(_ flowi.FlowTarget, l model.Layout) error {
	f.record("SaveFlowLayout")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.Layout = l
	return nil
}

func (f *Backend) ResponseFields(flowi.RequestTarget, string) ([]flowi.Completion, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.FieldList), nil
}

func (f *Backend) Completions(model.ID, model.Flow, string) ([]flowi.Completion, error) {
	f.Mu.Lock()
	defer f.Mu.Unlock()
	return slices.Clone(f.CompletionList), nil
}

func (f *Backend) RunFlow(_ context.Context, _ flowi.FlowTarget, o flowi.RunOptions) (vm.Running, error) {
	f.record("RunFlow")
	f.Mu.Lock()
	defer f.Mu.Unlock()
	f.LastRunOptions = o
	f.RunOptionsLog = append(f.RunOptionsLog, o)
	if f.RunFor != nil {
		r, err := f.RunFor(o)
		if err != nil {
			return nil, err
		}
		return r, nil
	}
	if f.NextRun == nil {
		return nil, errors.New("no run prepared")
	}
	return f.NextRun, nil
}

// Running replays EventList and then returns Result.
type Running struct {
	EventList []engine.Event
	Result    engine.RunResult
}

// Events replays the events and closes.
func (r *Running) Events() <-chan engine.Event {
	ch := make(chan engine.Event, len(r.EventList))
	for _, e := range r.EventList {
		ch <- e
	}
	close(ch)
	return ch
}

// Wait returns Result.
func (r *Running) Wait() engine.RunResult { return r.Result }

// Sending replays EventList and then returns Result.
type Sending struct {
	EventList []engine.Event
	Result    engine.SendResult
}

func (s *Sending) Events() <-chan engine.Event {
	ch := make(chan engine.Event, len(s.EventList))
	for _, e := range s.EventList {
		ch <- e
	}
	close(ch)
	return ch
}

func (s *Sending) Wait() engine.SendResult { return s.Result }

// SyncWorker runs calls inline on the test's goroutine: the test acts as
// the UI goroutine, so results must not arrive from another one.
func SyncWorker() *vm.Worker { return vm.NewInlineWorker() }

// SampleCollection is a collection with two requests in a folder, one at
// the root, two environments and one flow.
func SampleCollection() model.Collection {
	v := func(method, url string) []model.RequestVersion {
		return []model.RequestVersion{{Label: "1", Method: method, URL: url}}
	}
	return model.Collection{
		Root:     "/c",
		Manifest: model.Manifest{Meta: model.Meta{ID: "C1", Name: "Shop"}, File: "/c/flowi.yaml"},
		Environments: []model.Environment{
			{Meta: model.Meta{Name: "staging"}, File: "/c/environments/staging.yaml", Vars: []model.EnvVar{{Name: "base_url", Value: "https://s"}, {Name: "password", Secret: true}}},
			{Meta: model.Meta{Name: "local"}, File: "/c/environments/local.yaml"},
		},
		Requests: []model.Request{
			{Meta: model.Meta{ID: "R1", Name: "Login"}, Path: "auth/login", File: "/c/requests/auth/login.req.yaml", Versions: v("POST", "{{env.base_url}}/login")},
			{Meta: model.Meta{ID: "R2", Name: "Verify"}, Path: "auth/verify", File: "/c/requests/auth/verify.req.yaml", Versions: v("POST", "{{env.base_url}}/verify")},
			{Meta: model.Meta{ID: "R3", Name: "Health"}, Path: "health", File: "/c/requests/health.req.yaml", Versions: v("GET", "{{env.base_url}}/health")},
		},
		Flows: []model.Flow{{Meta: model.Meta{ID: "F1", Name: "Signup"}, Path: "signup", File: "/c/flows/signup.flow.yaml"}},
	}
}

// IdleScheduler never runs what it is given: tests of screens that edit
// requests must not leave a real 10-second autosave timer behind, which
// would fire into a later test.
type IdleScheduler struct{}

// After returns a stop that does nothing.
func (IdleScheduler) After(time.Duration, func()) func() { return func() {} }

// KeepFlowEdits remembers the flow in Kept; KeptFlowEdits returns it as "nodes: <count>" and forgets it.
func (f *Backend) KeepFlowEdits(t flowi.FlowTarget, fl model.Flow) error {
	f.record("KeepFlowEdits " + string(t.Flow))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	if f.Kept == nil {
		f.Kept = map[model.ID]model.Flow{}
	}
	f.Kept[t.Flow] = fl
	return nil
}

func (f *Backend) KeptFlowEdits(t flowi.FlowTarget) (string, bool, error) {
	f.record("KeptFlowEdits " + string(t.Flow))
	f.Mu.Lock()
	defer f.Mu.Unlock()
	fl, ok := f.Kept[t.Flow]
	delete(f.Kept, t.Flow)
	return fmt.Sprintf("nodes: %d\n", len(fl.Nodes)), ok, nil
}
