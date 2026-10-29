// SPDX-License-Identifier: AGPL-3.0-or-later

// Package backend adapts the flowi-lib workspace to vm.Backend. Every call
// first checks that it is not on the UI goroutine (see uithread).
package backend

import (
	"context"
	"fmt"
	"os"
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
	"github.com/githugt/flowi-lib/settings"

	"github.com/githugt/flowi/internal/uithread"
	"github.com/githugt/flowi/internal/vm"
)

// Workspace is vm.Backend over an open flowi-lib workspace.
type Workspace struct {
	ws    *flowi.Workspace
	guard *uithread.Guard
}

// New wraps ws; guard checks every call.
func New(ws *flowi.Workspace, guard *uithread.Guard) *Workspace {
	return &Workspace{ws: ws, guard: guard}
}

var _ vm.Backend = (*Workspace)(nil)

func (b *Workspace) Collections() []model.Collection {
	b.guard.Check("Collections")
	return b.ws.Collections()
}

func (b *Workspace) Missing() []model.ID {
	b.guard.Check("Missing")
	return b.ws.Missing()
}

// MountedPaths maps each mounted collection id to its path as written in
// workspace.yaml (missing ones included).
func (b *Workspace) MountedPaths() map[model.ID]string {
	b.guard.Check("MountedPaths")
	out := map[model.ID]string{}
	for _, m := range b.ws.Model().Collections {
		out[m.ID] = m.Path
	}
	return out
}

func (b *Workspace) Validate() diag.List {
	b.guard.Check("Validate")
	return b.ws.Validate()
}

func (b *Workspace) Settings(collection model.ID) settings.Resolved {
	b.guard.Check("Settings")
	return b.ws.Settings(flowi.Scope{Collection: collection})
}

func (b *Workspace) SetUserSetting(key, raw string) error {
	b.guard.Check("SetSetting")
	return b.ws.SetSetting(settings.LayerUser, key, raw)
}

func (b *Workspace) ResetUserSetting(key string) error {
	b.guard.Check("ResetSetting")
	return b.ws.ResetSetting(settings.LayerUser, key)
}

// WorkspaceName is the open workspace's display name.
func (b *Workspace) WorkspaceName() string { return b.ws.Model().Name }

func (b *Workspace) SaveRequest(t flowi.RequestTarget, r model.Request, dropComments bool) error {
	b.guard.Check("SaveRequest")
	return b.ws.SaveRequest(t, r, flowi.SaveOptions{DropComments: dropComments})
}

func (b *Workspace) StoreEdit(t flowi.RequestTarget, base, overlay string, v model.RequestVersion) error {
	b.guard.Check("StoreEdit")
	return b.ws.StoreEditUnder(t, base, overlay, v, flowi.SaveOptions{}) // autosave never drops comments
}

func (b *Workspace) DiscardEdit(t flowi.RequestTarget) error {
	b.guard.Check("DiscardEdit")
	return b.ws.DiscardEdit(t, flowi.SaveOptions{DropComments: true}) // clearing the slot is the user's explicit choice
}

func (b *Workspace) PreviewRequest(t flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions, includeSecrets bool) (engine.Preview, error) {
	b.guard.Check("PreviewRequest")
	return b.ws.PreviewRequest(t, v, o, includeSecrets)
}

func (b *Workspace) MoveRequest(t flowi.RequestTarget, to model.ID, path string) (flowi.RequestTarget, error) {
	b.guard.Check("MoveRequest")
	return b.ws.MoveRequest(t, to, path)
}

func (b *Workspace) SessionDir() string {
	b.guard.Check("SessionDir")
	return b.ws.SessionDir()
}

func (b *Workspace) DuplicateRequest(t flowi.RequestTarget, path string) (flowi.RequestTarget, error) {
	b.guard.Check("DuplicateRequest")
	return b.ws.DuplicateRequest(t, path)
}

func (b *Workspace) DeleteRequest(t flowi.RequestTarget) error {
	b.guard.Check("DeleteRequest")
	return b.ws.DeleteRequest(t)
}

func (b *Workspace) DuplicateFlow(t flowi.FlowTarget, path string) (flowi.FlowTarget, error) {
	b.guard.Check("DuplicateFlow")
	return b.ws.DuplicateFlow(t, path)
}

func (b *Workspace) DeleteFlow(t flowi.FlowTarget) error {
	b.guard.Check("DeleteFlow")
	return b.ws.DeleteFlow(t)
}

func (b *Workspace) DeleteEnvironment(file string) error {
	b.guard.Check("DeleteEnvironment")
	return b.ws.DeleteEnvironment(file)
}

func (b *Workspace) ExportMermaid(t flowi.FlowTarget) (string, error) {
	b.guard.Check("ExportMermaid")
	return b.ws.ExportMermaid(t)
}

func (b *Workspace) NewRequest(collection model.ID, path string) (flowi.RequestTarget, error) {
	b.guard.Check("NewRequest")
	return b.ws.NewRequest(collection, path)
}

// NewAsyncRequest creates an async request of the given protocol.
func (b *Workspace) NewAsyncRequest(collection model.ID, path, protocol string) (flowi.RequestTarget, error) {
	b.guard.Check("NewAsyncRequest")
	return b.ws.NewAsyncRequest(collection, path, protocol)
}

// NewGRPCRequest creates a gRPC request.
func (b *Workspace) NewGRPCRequest(collection model.ID, path string) (flowi.RequestTarget, error) {
	b.guard.Check("NewGRPCRequest")
	return b.ws.NewGRPCRequest(collection, path)
}

// GenerateGRPCRequests creates a gRPC request for each method.
func (b *Workspace) GenerateGRPCRequests(collection model.ID, base model.GRPC, methods []grpcx.Method) (flowi.GeneratedGRPC, error) {
	b.guard.Check("GenerateGRPCRequests")
	return b.ws.GenerateGRPCRequests(collection, base, methods)
}

// GRPCMethods lists the methods of the schema a gRPC version would use.
func (b *Workspace) GRPCMethods(ctx context.Context, t flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions) ([]grpcx.Method, error) {
	b.guard.Check("GRPCMethods")
	return b.ws.GRPCMethods(ctx, t, v, o)
}

func (b *Workspace) SeedRequestHooks(t flowi.RequestTarget) error {
	b.guard.Check("SeedRequestHooks")
	return b.ws.SeedRequestHooks(t)
}

func (b *Workspace) SaveEnvironment(e model.Environment, dropComments bool) error {
	b.guard.Check("SaveEnvironment")
	return b.ws.SaveEnvironment(e, flowi.SaveOptions{DropComments: dropComments})
}

func (b *Workspace) CollectionEnvironments(collection model.ID) ([]model.Environment, diag.List, error) {
	b.guard.Check("CollectionEnvironments")
	return b.ws.CollectionEnvironments(collection)
}

func (b *Workspace) Send(ctx context.Context, t flowi.RequestTarget, o flowi.SendOptions) (vm.Sending, error) {
	b.guard.Check("SendRequest")
	s, err := b.ws.SendRequest(ctx, t, o)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Workspace) PinExample(rec engine.Record, name string) (string, error) {
	b.guard.Check("PinExample")
	return b.ws.PinExample(rec, name)
}

// Overlays lists the flowi overlays of a collection.
func (b *Workspace) Overlays(collection model.ID) ([]model.Overlay, error) {
	b.guard.Check("Overlays")
	return b.ws.Overlays(collection)
}

// ApplyOverlay lays a flowi overlay over a version, as a send would.
func (b *Workspace) ApplyOverlay(t flowi.RequestTarget, name string, v model.RequestVersion) (model.RequestVersion, []flowioverlay.Problem, error) {
	b.guard.Check("ApplyOverlay")
	return b.ws.ApplyOverlay(t, name, v)
}

// DiffForOverlay works out what saving an edit as an overlay would store.
func (b *Workspace) DiffForOverlay(t flowi.RequestTarget, base string, edited model.RequestVersion) (flowi.OverlayDiff, error) {
	b.guard.Check("DiffForOverlay")
	return b.ws.DiffForOverlay(t, base, edited)
}

// SaveAsOverlay stores an edit as a flowi overlay.
func (b *Workspace) SaveAsOverlay(t flowi.RequestTarget, in flowi.SaveOverlayInput, dropComments bool) error {
	b.guard.Check("SaveAsOverlay")
	return b.ws.SaveAsOverlay(t, in, flowi.SaveOptions{DropComments: dropComments})
}

// DeleteOverlay removes a flowi overlay's file.
func (b *Workspace) DeleteOverlay(collection model.ID, name string) error {
	b.guard.Check("DeleteOverlay")
	return b.ws.DeleteOverlay(collection, name)
}

// OverlayText reads a flowi overlay's file.
func (b *Workspace) OverlayText(collection model.ID, name string) (string, error) {
	b.guard.Check("OverlayText")
	return b.ws.OverlayText(collection, name)
}

// ExportOverlayAsSpec writes a flowi overlay as OpenAPI Overlays for the specs the collection keeps.
func (b *Workspace) ExportOverlayAsSpec(collection model.ID, name string) (flowi.OverlayExport, error) {
	b.guard.Check("ExportOverlayAsSpec")
	return b.ws.ExportOverlayAsSpec(collection, name)
}

// WriteOverlayText saves text as the flowi overlay name.
func (b *Workspace) WriteOverlayText(collection model.ID, name, text string) error {
	b.guard.Check("WriteOverlayText")
	return b.ws.WriteOverlayText(collection, name, text)
}

func (b *Workspace) ClearCookies(collection string) error {
	b.guard.Check("ClearCookies")
	return b.ws.ClearCookies(collection)
}

func (b *Workspace) Cookies(collection string) ([]flowi.CookieInfo, error) {
	b.guard.Check("Cookies")
	return b.ws.Cookies(collection)
}

func (b *Workspace) DeleteCookie(collection, key string) error {
	b.guard.Check("DeleteCookie")
	return b.ws.DeleteCookie(collection, key)
}

func (b *Workspace) Schedules() []model.Schedule {
	b.guard.Check("Schedules")
	return b.ws.Schedules()
}

func (b *Workspace) AddSchedule(s model.Schedule) error {
	b.guard.Check("AddSchedule")
	return b.ws.AddSchedule(s)
}

func (b *Workspace) SaveSchedule(old string, s model.Schedule) error {
	b.guard.Check("SaveSchedule")
	return b.ws.SaveSchedule(old, s)
}

func (b *Workspace) RemoveSchedule(name string) error {
	b.guard.Check("RemoveSchedule")
	return b.ws.RemoveSchedule(name)
}

func (b *Workspace) HasTriggerToken() (bool, error) {
	b.guard.Check("HasTriggerToken")
	return b.ws.HasTriggerToken()
}

func (b *Workspace) NewTriggerToken() (string, error) {
	b.guard.Check("NewTriggerToken")
	return b.ws.NewTriggerToken()
}

func (b *Workspace) DeleteBlueprint(t flowi.BlueprintTarget) error {
	b.guard.Check("DeleteBlueprint")
	return b.ws.DeleteBlueprint(t)
}

func (b *Workspace) ExportArazzo(t flowi.FlowTarget) ([]byte, diag.List, error) {
	b.guard.Check("ExportArazzo")
	return b.ws.ExportArazzo(t)
}

func (b *Workspace) FlowSource(t flowi.FlowTarget, f model.Flow) (flowi.FlowText, error) {
	b.guard.Check("FlowSource")
	return b.ws.FlowSource(t, f)
}

func (b *Workspace) CheckFlowDraft(t flowi.FlowTarget, f model.Flow) diag.List {
	b.guard.Check("CheckFlowDraft")
	return b.ws.CheckFlowDraft(t, f)
}

func (b *Workspace) ParseFlowSource(t flowi.FlowTarget, text string) (model.Flow, diag.List, error) {
	b.guard.Check("ParseFlowSource")
	return b.ws.ParseFlowSource(t, text)
}

func (b *Workspace) Impact(t flowi.RequestTarget, version string) (analysis.Report, diag.List, error) {
	b.guard.Check("Impact")
	return b.ws.Impact(t, version)
}

func (b *Workspace) Secrets() ([]flowi.SecretInfo, error) {
	b.guard.Check("Secrets")
	return b.ws.Secrets()
}

func (b *Workspace) SetSecret(env, name, value string) error {
	b.guard.Check("SetSecret")
	return b.ws.SetSecret(env, name, value)
}

func (b *Workspace) RevealSecret(env, name string) (string, error) {
	b.guard.Check("RevealSecret")
	return b.ws.RevealSecret(env, name)
}

func (b *Workspace) DiscardSecret(env, name string) error {
	b.guard.Check("DiscardSecret")
	return b.ws.DiscardSecret(env, name)
}

func (b *Workspace) UnsetSecret(env, name string) error {
	b.guard.Check("UnsetSecret")
	return b.ws.UnsetSecret(env, name)
}

func (b *Workspace) Mount(dir string) (model.Manifest, error) {
	b.guard.Check("Mount")
	return b.ws.Mount(dir)
}

// OpenArazzo opens an Arazzo document as a collection of flows that lasts until the app quits.
func (b *Workspace) OpenArazzo(ctx context.Context, ref string) error {
	b.guard.Check("OpenArazzo")
	_, err := b.ws.KeepArazzo(ctx, ref)
	return err
}

func (b *Workspace) Coverage(collection model.ID) (analysis.Coverage, error) {
	b.guard.Check("Coverage")
	cov, _, err := b.ws.Coverage(collection)
	return cov, err
}

func (b *Workspace) ImpactAgainst(collection model.ID, name string, spec []byte) (flowi.SpecImpact, error) {
	b.guard.Check("ImpactAgainst")
	res, _, err := b.ws.ImpactAgainst(collection, name, spec)
	return res, err
}

func (b *Workspace) SpecNames(collection model.ID) []string {
	b.guard.Check("SpecNames")
	specs, err := b.ws.KeptSpecs(collection)
	if err != nil {
		return nil
	}
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

func (b *Workspace) CheckAccess(ctx context.Context, t flowi.FlowTarget, o flowi.AccessOptions) (flowi.AccessReport, error) {
	b.guard.Check("CheckAccess")
	return b.ws.CheckAccess(ctx, t, o)
}

func (b *Workspace) CheckDrift(ctx context.Context, t flowi.FlowTarget, o flowi.DriftOptions) (flowi.DriftReport, error) {
	b.guard.Check("CheckDrift")
	return b.ws.CheckDrift(ctx, t, o)
}

// OpenBundle reads the run bundle at path and keeps it open as a collection of replays.
func (b *Workspace) OpenBundle(path string) error {
	b.guard.Check("OpenBundle")
	data, err := os.ReadFile(path) //nolint:gosec // the person chose the file in the window
	if err != nil {
		return fmt.Errorf("reading %s: %w", path, err)
	}
	_, err = b.ws.KeepBundle(data)
	return err
}

func (b *Workspace) Unmount(id model.ID) error {
	b.guard.Check("Unmount")
	return b.ws.Unmount(id)
}

func (b *Workspace) Relocate(id model.ID, dir string) error {
	b.guard.Check("Relocate")
	return b.ws.Relocate(id, dir)
}

func (b *Workspace) InitCollection(dir, name string) (model.Manifest, error) {
	b.guard.Check("InitCollection")
	return b.ws.InitCollection(dir, name, true)
}

func (b *Workspace) NewDraft(name string) (model.Manifest, error) {
	b.guard.Check("NewDraft")
	return b.ws.NewDraft(name)
}

func (b *Workspace) Scratch() (model.Manifest, error) {
	b.guard.Check("Scratch")
	return b.ws.Scratch()
}

func (b *Workspace) NewScratchRequest() (flowi.RequestTarget, error) {
	b.guard.Check("NewScratchRequest")
	return b.ws.NewScratchRequest()
}

func (b *Workspace) IsDraft(id model.ID) bool {
	b.guard.Check("IsDraft")
	return b.ws.IsDraft(id)
}

func (b *Workspace) SaveDraft(id model.ID, dir string) error {
	b.guard.Check("SaveDraft")
	return b.ws.SaveDraft(id, dir)
}

func (b *Workspace) DiscardDraft(id model.ID) error {
	b.guard.Check("DiscardDraft")
	return b.ws.DiscardDraft(id)
}

func (b *Workspace) PreviewImportSource(src flowi.ImportSource, collection model.ID) (flowi.ImportPlan, error) {
	b.guard.Check("PreviewImportSource")
	return b.ws.PreviewImportSource(src, collection)
}

func (b *Workspace) ApplyImport(p flowi.ImportPlan, o flowi.ApplyOptions) error {
	b.guard.Check("ApplyImport")
	return b.ws.ApplyImport(p, o)
}

func (b *Workspace) SaveFlow(t flowi.FlowTarget, f model.Flow, dropComments bool) error {
	b.guard.Check("SaveFlow")
	return b.ws.SaveFlow(t, f, flowi.SaveOptions{DropComments: dropComments})
}

func (b *Workspace) NewEnvironment(name string) (string, error) {
	b.guard.Check("NewEnvironment")
	return b.ws.NewEnvironment(name)
}

func (b *Workspace) FlowRunDetail(t flowi.FlowTarget, id string) (flowi.RunDetail, error) {
	b.guard.Check("FlowRunDetail")
	return b.ws.FlowRunDetail(t, id)
}

func (b *Workspace) FlowRuns(t flowi.FlowTarget, limit int) []flowi.RunSummary {
	b.guard.Check("FlowRuns")
	return b.ws.FlowRuns(t, limit)
}

func (b *Workspace) TrashStaleDrafts(olderThan time.Duration, open func(kind string, id model.ID) bool) ([]flowi.TrashedItem, error) {
	b.guard.Check("TrashStaleDrafts")
	return b.ws.TrashStaleDrafts(olderThan, open)
}

func (b *Workspace) ListTrash() ([]flowi.TrashedItem, error) {
	b.guard.Check("ListTrash")
	return b.ws.ListTrash()
}

func (b *Workspace) RestoreFromTrash(id string) (string, error) {
	b.guard.Check("RestoreFromTrash")
	return b.ws.RestoreFromTrash(id)
}

func (b *Workspace) DeleteFromTrash(id string) error {
	b.guard.Check("DeleteFromTrash")
	return b.ws.DeleteFromTrash(id)
}

func (b *Workspace) EmptyTrash() error {
	b.guard.Check("EmptyTrash")
	return b.ws.EmptyTrash()
}

func (b *Workspace) StoredResponses(t flowi.RequestTarget) ([]flowi.StoredResponse, error) {
	b.guard.Check("StoredResponses")
	return b.ws.StoredResponses(t)
}

func (b *Workspace) DeleteStoredResponse(t flowi.RequestTarget, kind, version, name string) error {
	b.guard.Check("DeleteStoredResponse")
	return b.ws.DeleteStoredResponse(t, kind, version, name)
}

func (b *Workspace) StoredRecord(t flowi.RequestTarget, version, name string) (engine.Record, error) {
	b.guard.Check("StoredRecord")
	return b.ws.StoredRecord(t, version, name)
}

func (b *Workspace) LastResponse(t flowi.RequestTarget, version string) (engine.Record, bool) {
	b.guard.Check("LastResponse")
	return b.ws.LastResponse(t, version)
}

func (b *Workspace) SetFolderOrder(collection model.ID, tree, dir string, order []string, dropComments bool) error {
	b.guard.Check("SetFolderOrder")
	return b.ws.SetFolderOrder(collection, tree, dir, order, flowi.SaveOptions{DropComments: dropComments})
}

func (b *Workspace) NewFolder(collection model.ID, tree, path string) error {
	b.guard.Check("NewFolder")
	return b.ws.NewFolder(collection, tree, path)
}

func (b *Workspace) MoveFolder(collection model.ID, tree, from, to string) error {
	b.guard.Check("MoveFolder")
	return b.ws.MoveFolder(collection, tree, from, to)
}

func (b *Workspace) DeleteFolder(collection model.ID, tree, path string) error {
	b.guard.Check("DeleteFolder")
	return b.ws.DeleteFolder(collection, tree, path)
}

func (b *Workspace) ListSchemas(t flowi.RequestTarget, version string) []flowi.SchemaPart {
	b.guard.Check("ListSchemas")
	return b.ws.ListSchemas(t, version)
}

func (b *Workspace) SchemaFor(t flowi.RequestTarget, version string, part flowi.SchemaPart) (schemainfer.Schema, flowi.SchemaSource, bool) {
	b.guard.Check("SchemaFor")
	return b.ws.SchemaFor(t, version, part)
}

func (b *Workspace) DeleteSchema(t flowi.RequestTarget, version string, part flowi.SchemaPart) error {
	b.guard.Check("DeleteSchema")
	return b.ws.DeleteSchema(t, version, part)
}

func (b *Workspace) FlowCompatibility(t flowi.FlowTarget) diag.List {
	b.guard.Check("FlowCompatibility")
	return b.ws.FlowCompatibility(t)
}

func (b *Workspace) Environments() []model.Environment {
	b.guard.Check("Environments")
	return b.ws.Environments()
}

func (b *Workspace) NewUntitledFlow(collection model.ID) (flowi.FlowTarget, error) {
	b.guard.Check("NewUntitledFlow")
	return b.ws.NewUntitledFlow(collection)
}

func (b *Workspace) NewFlow(collection model.ID, path string) (flowi.FlowTarget, error) {
	b.guard.Check("NewFlow")
	return b.ws.NewFlow(collection, path)
}

func (b *Workspace) NewFlowFromBlueprint(bp flowi.BlueprintTarget, collection model.ID, path string, values map[string]string) (flowi.FlowTarget, error) {
	b.guard.Check("NewFlowFromBlueprint")
	return b.ws.NewFlowFromBlueprint(bp, collection, path, values)
}

func (b *Workspace) CreateHookBlueprint(collection model.ID, path, name, phase string, nodes []model.Node) (flowi.BlueprintTarget, error) {
	b.guard.Check("CreateHookBlueprint")
	return b.ws.CreateHookBlueprint(collection, path, name, phase, nodes)
}

func (b *Workspace) CreateBlueprint(flow flowi.FlowTarget, path, name string) (flowi.BlueprintTarget, error) {
	b.guard.Check("CreateBlueprint")
	return b.ws.CreateBlueprint(flow, path, name)
}

func (b *Workspace) FlowLayout(t flowi.FlowTarget) (model.Layout, diag.List, error) {
	b.guard.Check("FlowLayout")
	return b.ws.FlowLayout(t)
}

func (b *Workspace) SaveFlowLayout(t flowi.FlowTarget, l model.Layout) error {
	b.guard.Check("SaveFlowLayout")
	return b.ws.SaveFlowLayout(t, l)
}

func (b *Workspace) ResponseFields(t flowi.RequestTarget, version string) ([]flowi.Completion, error) {
	b.guard.Check("ResponseFields")
	return b.ws.ResponseFields(t, version)
}

func (b *Workspace) Completions(collection model.ID, f model.Flow, nodePath string) ([]flowi.Completion, error) {
	b.guard.Check("Completions")
	return b.ws.CompletionsIn(collection, f, nodePath)
}

func (b *Workspace) RunFlow(ctx context.Context, t flowi.FlowTarget, o flowi.RunOptions) (vm.Running, error) {
	b.guard.Check("RunFlow")
	r, err := b.ws.RunFlow(ctx, t, o)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (b *Workspace) ServeDefaults() flowi.ServeOptions {
	b.guard.Check("ServeDefaults")
	return b.ws.ServeDefaults()
}

func (b *Workspace) Announcements(ctx context.Context, refresh bool) (announce.Result, error) {
	b.guard.Check("Announcements")
	return b.ws.Announcements(ctx, refresh)
}

func (b *Workspace) Serve(ctx context.Context, o flowi.ServeOptions) (vm.Serving, error) {
	b.guard.Check("Serve")
	s, err := b.ws.Serve(ctx, o)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (b *Workspace) Watch(ctx context.Context) (<-chan flowi.Change, error) {
	b.guard.Check("Watch")
	return b.ws.Watch(ctx)
}

func (b *Workspace) Reload(ch flowi.Change) (diag.List, error) {
	b.guard.Check("Reload")
	return b.ws.Reload(ch)
}

func (b *Workspace) KeepFlowEdits(t flowi.FlowTarget, f model.Flow) error {
	b.guard.Check("KeepFlowEdits")
	return b.ws.KeepFlowEdits(t, f)
}

func (b *Workspace) KeptFlowEdits(t flowi.FlowTarget) (string, bool, error) {
	b.guard.Check("KeptFlowEdits")
	return b.ws.KeptFlowEdits(t)
}
