// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"time"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/announce"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/flowioverlay"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/schemainfer"
	"github.com/githugt/flowi/lib/serve/replay"
	"github.com/githugt/flowi/lib/serve/scheduler"
	"github.com/githugt/flowi/lib/settings"
)

// Backend is the part of the flowi-lib workspace the view-models use. Its
// methods block (disk, network, vault): call them only through a Worker.
type Backend interface {
	WorkspaceName() string
	// SessionDir is where what carries over between sessions is kept.
	SessionDir() string
	Collections() []model.Collection
	Missing() []model.ID
	MountedPaths() map[model.ID]string
	Validate() diag.List
	Settings(collection model.ID) settings.Resolved
	SetUserSetting(key, raw string) error
	ResetUserSetting(key string) error

	SaveRequest(t flowi.RequestTarget, r model.Request, dropComments bool) error
	// StoreEdit keeps v as the request's dirty edit of version base; overlay names the flowi overlay it was made with ("" for none).
	StoreEdit(t flowi.RequestTarget, base, overlay string, v model.RequestVersion) error
	DiscardEdit(t flowi.RequestTarget) error
	PreviewRequest(t flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions, includeSecrets bool) (engine.Preview, error)
	NewRequest(collection model.ID, path string) (flowi.RequestTarget, error)
	// NewAsyncRequest is NewRequest for an async request of a protocol.
	NewAsyncRequest(collection model.ID, path, protocol string) (flowi.RequestTarget, error)
	// NewGRPCRequest is NewRequest for a gRPC request.
	NewGRPCRequest(collection model.ID, path string) (flowi.RequestTarget, error)
	// NewDatasetRequest is NewRequest for a dataset request of the given driver.
	NewDatasetRequest(collection model.ID, path, driver string) (flowi.RequestTarget, error)
	// GenerateGRPCRequests makes a gRPC request per method of base's schema.
	GenerateGRPCRequests(collection model.ID, base model.GRPC, methods []grpcx.Method) (flowi.GeneratedGRPC, error)
	// GRPCMethods lists the methods of the schema the gRPC version v would use.
	GRPCMethods(ctx context.Context, t flowi.RequestTarget, v model.RequestVersion, o flowi.SendOptions) ([]grpcx.Method, error)
	// NewFolder creates a folder under tree ("requests" or "flows").
	NewFolder(collection model.ID, tree, path string) error
	// SetFolderOrder sets the order the entries of a folder are shown in (dir "" is the top of the tree).
	SetFolderOrder(collection model.ID, tree, dir string, order []string, dropComments bool) error
	MoveFolder(collection model.ID, tree, from, to string) error
	DeleteFolder(collection model.ID, tree, path string) error
	SeedRequestHooks(t flowi.RequestTarget) error
	MoveRequest(t flowi.RequestTarget, to model.ID, path string) (flowi.RequestTarget, error)
	DuplicateRequest(t flowi.RequestTarget, path string) (flowi.RequestTarget, error)
	// DuplicateRequestTo copies a request to path in another (or the same) collection.
	DuplicateRequestTo(t flowi.RequestTarget, to model.ID, path string) (flowi.RequestTarget, error)
	DeleteRequest(t flowi.RequestTarget) error
	DuplicateFlow(t flowi.FlowTarget, path string) (flowi.FlowTarget, error)
	DeleteFlow(t flowi.FlowTarget) error
	DeleteEnvironment(file string) error
	// Environments are the workspace's own: the same in every collection.
	Environments() []model.Environment
	// ExportMermaid is the saved flow as a Mermaid flowchart.
	ExportMermaid(t flowi.FlowTarget) (string, error)
	ExportArazzo(t flowi.FlowTarget) ([]byte, diag.List, error)
	// FlowSource shows f (flow t with unsaved edits) as text; ParseFlowSource reads such text back.
	FlowSource(t flowi.FlowTarget, f model.Flow) (flowi.FlowText, error)
	ParseFlowSource(t flowi.FlowTarget, text string) (model.Flow, diag.List, error)
	// CheckFlowDraft validates f in place of flow t and returns the problems in that flow.
	CheckFlowDraft(t flowi.FlowTarget, f model.Flow) diag.List
	// KeepFlowEdits stores a flow's unsaved edits for the next start; KeptFlowEdits returns them as source text and forgets them.
	KeepFlowEdits(t flowi.FlowTarget, f model.Flow) error
	KeptFlowEdits(t flowi.FlowTarget) (string, bool, error)
	SaveEnvironment(e model.Environment, dropComments bool) error
	// NewEnvironment creates an empty environment and returns its file.
	NewEnvironment(name string) (string, error)

	Send(ctx context.Context, t flowi.RequestTarget, o flowi.SendOptions) (Sending, error)
	// LastResponse is the response the last send of version saved on disk.
	LastResponse(t flowi.RequestTarget, version string) (engine.Record, bool)
	// FlowRuns lists the saved runs of a flow, newest first.
	FlowRuns(t flowi.FlowTarget, limit int) []flowi.RunSummary
	// FlowRunDetail reads what one saved run did at each node and the variables it ended with.
	FlowRunDetail(t flowi.FlowTarget, id string) (flowi.RunDetail, error)
	// FlowCompatibility lists the steps of a flow only Flowi can run (FLW-ARAZZO-002).
	FlowCompatibility(t flowi.FlowTarget) diag.List
	// ListSchemas, SchemaFor and DeleteSchema are the schemas learned from
	// (or imported for) a request version.
	ListSchemas(t flowi.RequestTarget, version string) []flowi.SchemaPart
	SchemaFor(t flowi.RequestTarget, version string, part flowi.SchemaPart) (schemainfer.Schema, flowi.SchemaSource, bool)
	DeleteSchema(t flowi.RequestTarget, version string, part flowi.SchemaPart) error
	PinExample(rec engine.Record, name string) (string, error)
	// TrashStaleDrafts moves old, unused try-out requests and flows of Scratch to the trash;
	// ListTrash, RestoreFromTrash, DeleteFromTrash and EmptyTrash are the rest of the trash.
	TrashStaleDrafts(olderThan time.Duration, open func(kind string, id model.ID) bool) ([]flowi.TrashedItem, error)
	ListTrash() ([]flowi.TrashedItem, error)
	RestoreFromTrash(id string) (string, error)
	DeleteFromTrash(id string) error
	EmptyTrash() error
	// Overlays lists the flowi overlays of a collection; ApplyOverlay lays one over a version
	// as a send would; DiffForOverlay and SaveAsOverlay store an edit as one; OverlayText and
	// WriteOverlayText read and write an overlay's file for the overlay editor.
	Overlays(collection model.ID) ([]model.Overlay, error)
	ApplyOverlay(t flowi.RequestTarget, name string, v model.RequestVersion) (model.RequestVersion, []flowioverlay.Problem, error)
	DiffForOverlay(t flowi.RequestTarget, base string, edited model.RequestVersion) (flowi.OverlayDiff, error)
	SaveAsOverlay(t flowi.RequestTarget, in flowi.SaveOverlayInput, dropComments bool) error
	DeleteOverlay(collection model.ID, name string) error
	OverlayText(collection model.ID, name string) (string, error)
	ExportOverlayAsSpec(collection model.ID, name string) (flowi.OverlayExport, error)
	WriteOverlayText(collection model.ID, name, text string) error
	// StoredResponses lists what is kept for a request: pinned examples and saved responses.
	StoredResponses(t flowi.RequestTarget) ([]flowi.StoredResponse, error)
	DeleteStoredResponse(t flowi.RequestTarget, kind, version, name string) error
	// StoredRecord reads a saved response back, to show it.
	StoredRecord(t flowi.RequestTarget, version, name string) (engine.Record, error)
	ClearCookies(collection string) error
	Cookies(collection string) ([]flowi.CookieInfo, error)
	DeleteCookie(collection, key string) error
	Impact(t flowi.RequestTarget, version string) (analysis.Report, diag.List, error)

	Secrets() ([]flowi.SecretInfo, error)
	SetSecret(env, name, value string) error
	RevealSecret(env, name string) (string, error)
	DiscardSecret(env, name string) error
	UnsetSecret(env, name string) error

	Mount(dir string) (model.Manifest, error)
	// OpenBundle opens a run bundle file as a collection whose runs play the recording back. It lasts until
	// the app quits.
	OpenBundle(path string) error
	// OpenArazzo opens an Arazzo document (a file or URL) as a collection whose flows run against its own server.
	OpenArazzo(ctx context.Context, ref string) error
	// Coverage measures how much of the imported API descriptions the flows use ("" measures every collection).
	Coverage(collection model.ID) (analysis.Coverage, error)
	// ImpactAgainst says what a changed OpenAPI description would break; name picks a kept description ("" when
	// there is one).
	ImpactAgainst(collection model.ID, name string, spec []byte) (flowi.SpecImpact, error)
	// SpecNames are the OpenAPI descriptions a collection kept at import.
	SpecNames(collection model.ID) []string
	CheckAccess(ctx context.Context, t flowi.FlowTarget, o flowi.AccessOptions) (flowi.AccessReport, error)
	CheckDrift(ctx context.Context, t flowi.FlowTarget, o flowi.DriftOptions) (flowi.DriftReport, error)
	Unmount(id model.ID) error
	Relocate(id model.ID, dir string) error
	InitCollection(dir, name string) (model.Manifest, error)
	NewDraft(name string) (model.Manifest, error)
	Scratch() (model.Manifest, error)
	NewScratchRequest() (flowi.RequestTarget, error)
	IsDraft(id model.ID) bool
	SaveDraft(id model.ID, dir string) error
	DiscardDraft(id model.ID) error
	PreviewImportSource(src flowi.ImportSource, collection model.ID) (flowi.ImportPlan, error)
	ApplyImport(p flowi.ImportPlan, o flowi.ApplyOptions) error

	SaveFlow(t flowi.FlowTarget, f model.Flow, dropComments bool) error
	NewFlow(collection model.ID, path string) (flowi.FlowTarget, error)
	NewUntitledFlow(collection model.ID) (flowi.FlowTarget, error)
	NewFlowFromBlueprint(bp flowi.BlueprintTarget, collection model.ID, path string, values map[string]string) (flowi.FlowTarget, error)
	CreateBlueprint(flow flowi.FlowTarget, path, name string) (flowi.BlueprintTarget, error)
	DeleteBlueprint(t flowi.BlueprintTarget) error
	CreateHookBlueprint(collection model.ID, path, name, phase string, nodes []model.Node) (flowi.BlueprintTarget, error)
	FlowLayout(t flowi.FlowTarget) (model.Layout, diag.List, error)
	SaveFlowLayout(t flowi.FlowTarget, l model.Layout) error
	Completions(collection model.ID, f model.Flow, nodePath string) ([]flowi.Completion, error)
	// ResponseFields lists the body fields a request version is known to return (body.user.id).
	ResponseFields(t flowi.RequestTarget, version string) ([]flowi.Completion, error)
	RunFlow(ctx context.Context, t flowi.FlowTarget, o flowi.RunOptions) (Running, error)

	ServeDefaults() flowi.ServeOptions
	Schedules() []model.Schedule
	AddSchedule(s model.Schedule) error
	SaveSchedule(old string, s model.Schedule) error
	RemoveSchedule(name string) error
	HasTriggerToken() (bool, error)
	NewTriggerToken() (string, error)
	Announcements(ctx context.Context, refresh bool) (announce.Result, error)
	Serve(ctx context.Context, o flowi.ServeOptions) (Serving, error)

	Watch(ctx context.Context) (<-chan flowi.Change, error)
	Reload(ch flowi.Change) (diag.List, error)
}

// Sending is a send in progress (engine.Send).
type Sending interface {
	Events() <-chan engine.Event
	Wait() engine.SendResult
}

// Running is a flow run in progress (engine.Run).
type Running interface {
	Events() <-chan engine.Event
	Wait() engine.RunResult
}

// Serving is a running `flowi serve` (flowi.Server).
type Serving interface {
	ReplayAddr() string
	TriggerAddr() string
	// Upcoming is when each enabled schedule fires next, soonest first.
	Upcoming() []scheduler.Upcoming
	Notes() []string
	Traffic() *replay.Log
	// Wait returns once the context given to Serve has ended and the
	// server has shut down.
	Wait() error
}
