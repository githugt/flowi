// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"time"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/engine"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/version"
)

// AuthKind is what the Auth tab writes into the Authorization header.
type AuthKind int

// Auth kinds.
const (
	AuthNone AuthKind = iota
	AuthBearer
	AuthBasic
	AuthOther // a header the helper does not recognise: shown, not rewritten
)

const authHeader = "Authorization"

// SettingRow is one of a version's own settings as text.
type SettingRow struct {
	Key, Raw string
}

// Request is the request screen's state: an editable copy of one request,
// the selected version, and saving.
//
// All methods run on the UI goroutine; Save runs the write on the worker.
type Request struct {
	b          Backend
	w          *Worker
	schemas    *Schemas       // made by Schemas on first use
	methods    []grpcx.Method // the methods of a gRPC request's schema, as last loaded
	methodsErr string         // why they could not be loaded
	coll       model.Collection
	target     flowi.RequestTarget
	disk       model.Request // last loaded or saved
	draft      model.Request // being edited
	selected   int           // index into draft.Versions
	dirty      bool
	onDisk     bool // the file changed on disk while there were edits
	confirm    bool // saving would drop comments: ask, then Save(true)
	problems   []string
	rawRows    []SettingRow // the Settings tab as typed, partial rows included; nil: derive from the draft
	pending    func()       // what a save waiting for ConfirmDrop runs after it succeeds
	onDrop     []func()
	rev        int // bumped by every edit
	err        string
	onChange   []func()
	onEdit     []func()

	// Dirty edit (spec: dirty-edit slot). The slot on disk holds the
	// selected version's unsaved edit; the draft shows it applied.
	sched        Scheduler
	stopTimer    func()
	abandoned    bool   // the tab is gone: nothing is written for it any more
	slot         bool   // the file on disk has an edit slot
	orphan       string // base label of a stored edit whose version no longer exists
	saving       int    // saves queued or running: autosave stands aside
	deleting     bool   // a version delete is being written: edits and saves wait, as they would write the version back
	writing      bool
	queue        []writeJob
	autosaveProb string

	// The flowi overlay laid over the selected version (see overlay.go).
	overlay      string
	overlays     []model.Overlay // the collection's, as last loaded
	overlayNotes []string
	overlaySeq   int

	// What the Body section's pickers were set to while the body is empty:
	// an empty body cannot tell JSON from none.
	chosenKind      BodyKind
	chosenFormat    BodyFormat
	chosenMultipart bool
}

// AutosaveDelay is how long after the last change the edit is written to
// its slot.
const AutosaveDelay = 10 * time.Second

type writeJob struct {
	work func() error
	done func(error)
}

// NewRequest returns the view-model for request r of collection c.
func NewRequest(b Backend, w *Worker, c model.Collection, r model.Request) *Request {
	vm := &Request{b: b, w: w, coll: c, target: flowi.RequestTarget{Collection: c.Manifest.ID, Request: r.ID}}
	vm.load(r)
	return vm
}

func (r *Request) load(disk model.Request) {
	label := r.SelectedLabel()
	edit, base, editOverlay := disk.Edit, disk.EditBase, disk.EditOverlay
	disk.Edit, disk.EditBase, disk.EditOverlay = nil, "", ""
	r.disk, r.draft = disk, cloneRequest(disk)
	r.selected = max(0, slices.IndexFunc(r.draft.Versions, func(v model.RequestVersion) bool { return v.Label == label }))
	if label == "" {
		r.selected = defaultIndex(r.draft)
	}
	r.dirty, r.onDisk, r.confirm, r.problems, r.rawRows = false, false, false, nil, nil
	r.slot, r.autosaveProb, r.orphan = edit != nil, "", ""
	r.chosenKind, r.chosenFormat, r.chosenMultipart = "", "", false
	r.applyEdit(edit, base)
	if r.dirty {
		r.overlay = editOverlay // the edit shows the overlay it was made with; it is chosen again with the edit
	}
	r.applyOverlay() // a reload keeps the chosen overlay laid over the version
}

func orphanText(base string) string {
	return fmt.Sprintf("A stored edit belongs to version %s, which no longer exists. It is kept until you save (which replaces it) or discard it; autosave is paused.", base)
}

// applyEdit shows the on-disk edit in the draft: it replaces the version
// it started from, which becomes the selected one. An edit whose base is
// gone is kept in the file but not shown.
func (r *Request) applyEdit(edit *model.RequestVersion, base string) {
	if edit == nil {
		return
	}
	i := slices.IndexFunc(r.draft.Versions, func(v model.RequestVersion) bool { return v.Label == base })
	if i < 0 {
		r.orphan = base // kept in the file; never overwritten silently
		r.autosaveProb = orphanText(base)
		return
	}
	shown := cloneRequest(model.Request{Versions: []model.RequestVersion{*edit}}).Versions[0]
	shown.Label = base
	r.draft.Versions[i], r.selected = shown, i
	r.dirty = !reflect.DeepEqual(shown, r.disk.Versions[i])
}

// defaultIndex is the version shown first: the default, else the last.
func defaultIndex(r model.Request) int {
	if i := slices.IndexFunc(r.Versions, func(v model.RequestVersion) bool { return v.Label == r.Default }); i >= 0 {
		return i
	}
	return max(0, len(r.Versions)-1)
}

// cloneRequest deep-copies the slices and maps an edit can change.
func cloneRequest(r model.Request) model.Request {
	r.Versions = slices.Clone(r.Versions)
	for i, v := range r.Versions {
		v.Query, v.Headers, v.Extract, v.Assert = slices.Clone(v.Query), slices.Clone(v.Headers), slices.Clone(v.Extract), slices.Clone(v.Assert)
		v.Body.Form, v.Body.Multipart = slices.Clone(v.Body.Form), slices.Clone(v.Body.Multipart)
		v.Outcomes = slices.Clone(v.Outcomes)
		v.Settings = cloneSettings(v.Settings)
		v.Hooks = cloneHooks(v.Hooks)
		v.Async = cloneAsync(v.Async)
		v.GRPC = cloneGRPC(v.GRPC)
		v.Dataset = cloneDataset(v.Dataset)
		r.Versions[i] = v
	}
	r.Hooks = cloneHooks(r.Hooks)
	return r
}

// cloneHooks copies hook steps deeply: an edit of an inline node must not
// reach the copy it came from.
func cloneHooks(h model.Hooks) model.Hooks {
	steps := func(in []model.HookStep) []model.HookStep {
		if in == nil {
			return nil
		}
		out := make([]model.HookStep, len(in))
		for i, s := range in {
			if s.Flow != nil {
				f := *s.Flow
				s.Flow = &f
			}
			s.Inline = cloneNodes(s.Inline)
			out[i] = s
		}
		return out
	}
	return model.Hooks{Pre: steps(h.Pre), Post: steps(h.Post), Message: steps(h.Message)}
}

// IsDropsComments reports whether err is a save stopped because it would drop
// the file's comments, which the user may confirm.
func IsDropsComments(err error) bool { return hasCode(err, diag.CodeWriteDropsComments) }

func cloneSettings(s model.Settings) model.Settings {
	if s == nil {
		return nil
	}
	out := model.Settings{}
	for k, v := range s {
		out[k] = v
	}
	return out
}

// OnChange registers f to run after any state change.
func (r *Request) OnChange(f func()) { r.onChange = append(r.onChange, f) }

// OnEdit registers f to run after the user edits the request (not after
// saves, autosaves or reloads).
func (r *Request) OnEdit(f func()) { r.onEdit = append(r.onEdit, f) }

// OnDropConfirm registers f to run once each time a save stops because
// it would drop the file's comments; answer with ConfirmDrop or
// DeclineDrop.
func (r *Request) OnDropConfirm(f func()) { r.onDrop = append(r.onDrop, f) }

// ConfirmDrop saves again, dropping the comments, and then continues what
// the save was for (a send).
func (r *Request) ConfirmDrop() {
	then := r.pending
	r.pending = nil
	r.SaveThen(true, then)
}

// DeclineDrop gives up the save that would drop comments.
func (r *Request) DeclineDrop() {
	r.confirm, r.pending = false, nil
	r.changed()
}

func (r *Request) changed() {
	for _, f := range r.onChange {
		f()
	}
}

// Collection is the collection the request is in, as of its last load.
func (r *Request) Collection() model.Collection { return r.coll }

// Hooks are the request's own hooks (not its versions' or other levels').
func (r *Request) Hooks() model.Hooks { return r.draft.Hooks }

// SaveHooks writes the request with hooks h as its own hooks, together with
// any other edit of it. The draft takes the hooks only once they are written,
// so a save that fails leaves it as it was. done gets the outcome on the UI
// goroutine; a save that would drop comments is reported as an error that
// IsDropsComments recognises, to be tried again with dropComments.
func (r *Request) SaveHooks(h model.Hooks, dropComments bool, done func(error)) {
	if len(r.problems) > 0 {
		r.err = "fix the settings first: " + strings.Join(r.problems, "; ")
		r.changed()
		done(errors.New(r.err))
		return
	}
	r.cancelAutosave()
	draft, rev := cloneRequest(r.draft), r.rev
	draft.Hooks = cloneHooks(h)
	r.saving++
	r.write(func() error { return r.b.SaveRequest(r.target, draft, dropComments) }, func(err error) {
		r.saving--
		switch {
		case IsDropsComments(err):
		case err != nil:
			r.err = ErrorText(err)
		default:
			r.saved(draft, rev, nil)
			r.draft.Hooks = cloneHooks(h)
			r.err = ""
		}
		r.changed()
		done(err)
	})
}

// Schemas is the schemas learned for this request's selected version. It is
// made on first use and kept.
func (r *Request) Schemas() *Schemas {
	if r.schemas == nil {
		r.schemas = NewSchemas(r.b, r.w, r)
	}
	return r.schemas
}

// Target identifies the request.
func (r *Request) Target() flowi.RequestTarget { return r.target }

// Draft is the request as edited.
func (r *Request) Draft() model.Request { return r.draft }

// Current is the selected version as edited.
func (r *Request) Current() model.RequestVersion {
	if len(r.draft.Versions) == 0 {
		return model.RequestVersion{}
	}
	return r.draft.Versions[r.selected]
}

// Labels lists the version labels.
func (r *Request) Labels() []string {
	out := make([]string, len(r.draft.Versions))
	for i, v := range r.draft.Versions {
		out[i] = v.Label
	}
	return out
}

// SelectedLabel is the selected version's label.
func (r *Request) SelectedLabel() string {
	if r.selected >= len(r.draft.Versions) {
		return ""
	}
	return r.draft.Versions[r.selected].Label
}

// Dirty reports unsaved edits.
func (r *Request) Dirty() bool { return r.dirty }

// CanDiscardEdit reports that there is an edit worth discarding: unsaved
// changes to a version that has content to go back to (an edit of a request
// that was never filled in has nothing to return to).
func (r *Request) CanDiscardEdit() bool {
	if !r.dirty || r.selected >= len(r.disk.Versions) {
		return false
	}
	return !isBlankVersion(r.disk.Versions[r.selected])
}

// isBlankVersion is a version with nothing but a method and a label.
func isBlankVersion(v model.RequestVersion) bool {
	return v.URL == "" && len(v.Query) == 0 && len(v.Headers) == 0 && !bodyHasContent(v.Body) && len(v.Extract) == 0 &&
		len(v.Assert) == 0 && len(v.Outcomes) == 0 && len(v.Variables) == 0 && v.Docs == "" && len(v.Settings) == 0 &&
		len(v.Hooks.Pre) == 0 && len(v.Hooks.Post) == 0 && len(v.Hooks.Message) == 0
}

// ChangedOnDisk reports that the file changed on disk while there were
// unsaved edits; Revert takes the disk version.
func (r *Request) ChangedOnDisk() bool { return r.onDisk }

// NeedsDropConfirm reports that saving would drop the file's comments.
func (r *Request) NeedsDropConfirm() bool { return r.confirm }

// Err is the last problem to show, or "".
func (r *Request) Err() string { return r.err }

// SelectVersion shows another version.
func (r *Request) SelectVersion(label string) {
	if r.dirty && label != r.SelectedLabel() {
		// The edit slot holds one version's edit: switching would orphan it.
		r.err = "Save or discard your edit before switching versions."
		r.changed()
		return
	}
	if i := slices.Index(r.Labels(), label); i >= 0 {
		r.selected, r.rawRows, r.problems = i, nil, nil
		r.chosenKind, r.chosenFormat, r.chosenMultipart = "", "", false
		r.applyOverlay() // the chosen overlay goes with the user to the other version
		r.changed()
	}
}

// Edit changes the selected version.
func (r *Request) Edit(f func(v *model.RequestVersion)) {
	if len(r.draft.Versions) == 0 || r.deleting {
		return
	}
	f(&r.draft.Versions[r.selected])
	r.dirty = true
	r.rev++
	r.scheduleAutosave()
	r.changed()
	for _, f := range r.onEdit {
		f()
	}
}

// SetDefault makes the selected version the request's default.
func (r *Request) SetDefault() {
	r.draft.Default = r.SelectedLabel()
	r.dirty = true
	r.rev++
	r.changed()
}

// NewVersionFromCurrent copies the selected version as label and selects
// it. The label must be a valid, unused version label.
func (r *Request) NewVersionFromCurrent(label string) error {
	label = version.Normalize(label)
	if _, err := version.Parse(label); err != nil {
		return fmt.Errorf("%q is not a version label (use dotted numbers, e.g. 2 or 1.1)", label)
	}
	if existing, taken := sameVersionLabel(r.Labels(), label); taken {
		return fmt.Errorf("version %q already exists (%q is the same version)", label, existing)
	}
	v := cloneRequest(model.Request{Versions: []model.RequestVersion{r.Current()}}).Versions[0]
	v.Label = label
	if r.selected < len(r.disk.Versions) && r.draft.Versions[r.selected].Label == r.disk.Versions[r.selected].Label {
		// "Save as new version": the edit moves to the new version and the
		// one it started from keeps its saved content.
		r.draft.Versions[r.selected] = cloneRequest(model.Request{Versions: []model.RequestVersion{r.disk.Versions[r.selected]}}).Versions[0]
	}
	r.draft.Versions = append(r.draft.Versions, v)
	r.selected = len(r.draft.Versions) - 1
	r.dirty = true
	r.rev++
	r.changed()
	return nil
}

// DeleteVersion removes the selected version from the request file at once.
// The default version and the last one stay. A version that was never saved
// is just dropped from the draft. Only the selected version can hold unsaved
// edits (switching refuses otherwise), so nothing else is lost. done gets the
// outcome; a drop-comments error is for the caller to confirm and repeat
// with dropComments set.
func (r *Request) DeleteVersion(dropComments bool, done func(error)) {
	label := r.SelectedLabel()
	switch {
	case len(r.draft.Versions) < 2:
		done(errors.New("a request keeps at least one version"))
		return
	case label == r.draft.Default || label == r.disk.Default:
		done(fmt.Errorf("version %s is the default: make another version the default first", label))
		return
	case r.deleting:
		done(errors.New("a delete is already in progress"))
		return
	case !slices.ContainsFunc(r.disk.Versions, func(v model.RequestVersion) bool { return v.Label == label }):
		r.cancelAutosave()
		r.drop(label)
		r.dirty = !sameSaved(r.draft, r.disk) // what is left may equal the file again
		r.changed()
		done(nil)
		return
	}
	next := cloneRequest(r.disk)
	next.Versions = slices.DeleteFunc(next.Versions, func(v model.RequestVersion) bool { return v.Label == label })
	r.cancelAutosave()
	r.saving++
	r.deleting = true
	r.write(func() error { return r.b.SaveRequest(r.target, next, dropComments) }, func(err error) {
		r.saving--
		r.deleting = false
		if err == nil {
			r.drop(label)
			r.disk = cloneRequest(next)
			r.rev++
			r.dirty, r.slot = r.draft.Default != r.disk.Default, false // an unsaved change of default stays unsaved
			r.err = ""
		} else if !IsDropsComments(err) {
			r.err = ErrorText(err)
		}
		r.changed()
		done(err)
	})
}

// drop removes a version from the draft and selects the default.
func (r *Request) drop(label string) {
	r.draft.Versions = slices.DeleteFunc(r.draft.Versions, func(v model.RequestVersion) bool { return v.Label == label })
	r.selected = defaultIndex(r.draft)
	r.rawRows, r.problems = nil, nil
	r.chosenKind, r.chosenFormat, r.chosenMultipart = "", "", false
}

// sameVersionLabel finds a label that names the same version as label
// ("1" and "1.0" do, as the file format compares them).
func sameVersionLabel(labels []string, label string) (string, bool) {
	want, err := version.Parse(label)
	if err != nil {
		return "", slices.Contains(labels, label)
	}
	for _, l := range labels {
		if have, err := version.Parse(l); err == nil && have.Compare(want) == 0 {
			return l, true
		}
	}
	return "", false
}

// Auth reads the Authorization header of the selected version.
func (r *Request) Auth() (AuthKind, string) {
	i := slices.IndexFunc(r.Current().Headers, func(kv model.KeyValue) bool { return strings.EqualFold(kv.Key, authHeader) })
	if i < 0 {
		return AuthNone, ""
	}
	value := r.Current().Headers[i].Value
	scheme, rest, _ := strings.Cut(value, " ")
	switch {
	case strings.EqualFold(scheme, "Bearer"):
		return AuthBearer, rest
	case strings.EqualFold(scheme, "Basic"):
		return AuthBasic, rest
	}
	return AuthOther, value
}

// AuthScheme is the scheme of the version's auth block ("" when it has none)
// and its parameters in file order.
func (r *Request) AuthScheme() (string, []model.KeyValue) {
	a := r.Current().Auth
	if a == nil {
		return "", nil
	}
	return a.Type, a.Params
}

// SetAuthScheme chooses the auth block's scheme; "" removes the block. A
// different scheme starts with no parameters, since they do not carry over.
func (r *Request) SetAuthScheme(scheme string) {
	r.Edit(func(v *model.RequestVersion) {
		switch {
		case scheme == "":
			v.Auth = nil
		case v.Auth == nil || v.Auth.Type != scheme:
			v.Auth = &model.Auth{Type: scheme}
		}
	})
}

// SetAuthParam sets a parameter of the auth block; an empty value removes it.
func (r *Request) SetAuthParam(name, value string) {
	r.Edit(func(v *model.RequestVersion) {
		if v.Auth == nil {
			return
		}
		params := slices.DeleteFunc(slices.Clone(v.Auth.Params), func(kv model.KeyValue) bool { return kv.Key == name })
		if value != "" {
			params = append(params, model.KeyValue{Key: name, Value: value})
		}
		v.Auth = &model.Auth{Type: v.Auth.Type, Params: params}
	})
}

// SetAuth writes (or, for AuthNone, removes) the Authorization header.
func (r *Request) SetAuth(kind AuthKind, value string) {
	r.Edit(func(v *model.RequestVersion) {
		v.Auth = nil // the simple forms replace an auth block
		// On a copy: callers may still hold the old slice from Current.
		v.Headers = slices.DeleteFunc(slices.Clone(v.Headers), func(kv model.KeyValue) bool { return strings.EqualFold(kv.Key, authHeader) })
		switch kind {
		case AuthBearer:
			v.Headers = append(v.Headers, model.KeyValue{Key: authHeader, Value: "Bearer " + value})
		case AuthBasic:
			v.Headers = append(v.Headers, model.KeyValue{Key: authHeader, Value: "Basic " + value})
		case AuthOther:
			v.Headers = append(v.Headers, model.KeyValue{Key: authHeader, Value: value})
		}
	})
}

// SettingRows are the Settings tab's rows: as typed, or the selected
// version's own settings (sorted, lists comma-separated).
func (r *Request) SettingRows() []SettingRow {
	if r.rawRows != nil {
		return slices.Clone(r.rawRows)
	}
	var out []SettingRow
	for k, v := range r.Current().Settings {
		out = append(out, SettingRow{Key: k, Raw: valueText(v)})
	}
	slices.SortFunc(out, func(a, b SettingRow) int { return strings.Compare(a.Key, b.Key) })
	return out
}

// SetSettings takes the Settings tab's rows as typed. Every row is kept
// for display; rows that parse become the version's settings, the others
// are returned as problems (and block Save). Empty rows are ignored.
func (r *Request) SetSettings(rows []SettingRow) []string {
	r.rawRows = slices.Clone(rows)
	if r.rawRows == nil {
		r.rawRows = []SettingRow{}
	}
	out := model.Settings{}
	var problems []string
	for _, row := range rows {
		if row.Key == "" && row.Raw == "" {
			continue
		}
		v, err := parseSetting(row)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", row.Key, err))
			continue
		}
		out[row.Key] = v
	}
	r.problems = problems
	r.Edit(func(v *model.RequestVersion) { v.Settings = out })
	return problems
}

func parseSetting(row SettingRow) (any, error) {
	k, ok := settings.Lookup(settings.Schema(), row.Key)
	if !ok || !slices.Contains(k.FileLayers, settings.LayerRequest) {
		return nil, errors.New("not a setting a request can set")
	}
	return settings.ParseValue(k, row.Raw)
}

// HookChain lists the hooks that run around the selected version, in run
// order: "pre · folder · hooks/sign", "post · collection · inline".
func (r *Request) HookChain() []string {
	chain := engine.ChainFor(r.coll, r.draft, r.Current())
	var out []string
	for _, h := range chain.Pre {
		out = append(out, "pre · "+h.Level+" · "+h.Label())
	}
	for _, h := range chain.Post {
		out = append(out, "post · "+h.Level+" · "+h.Label())
	}
	return out
}

// ExtractAsVariable adds {name, from: body, path} to the selected
// version's extract block and saves. A name already extracted is refused.
func (r *Request) ExtractAsVariable(name, path string) error {
	if name == "" {
		return errors.New("enter a variable name")
	}
	if slices.ContainsFunc(r.Current().Extract, func(e model.Extraction) bool { return e.Name == name }) {
		return fmt.Errorf("version %s already extracts %q", r.SelectedLabel(), name)
	}
	r.Edit(func(v *model.RequestVersion) {
		v.Extract = append(v.Extract, model.Extraction{Name: name, From: "body", Path: path})
	})
	r.Save(false)
	return nil
}

// Save writes the draft. dropComments confirms rewriting a file with
// comments (after NeedsDropConfirm).
func (r *Request) Save(dropComments bool) { r.SaveThen(dropComments, nil) }

// SaveThen saves, then runs then on the UI goroutine if the save
// succeeded (or there was nothing to save).
func (r *Request) SaveThen(dropComments bool, then func()) {
	if r.deleting {
		return // the delete in progress writes the file; saving now would write the deleted version back
	}
	if len(r.problems) > 0 {
		r.err = "fix the settings first: " + strings.Join(r.problems, "; ")
		r.changed()
		return
	}
	if !r.dirty && then != nil {
		then()
		return
	}
	r.cancelAutosave()
	draft, rev := cloneRequest(r.draft), r.rev
	r.saving++
	r.write(func() error { return r.b.SaveRequest(r.target, draft, dropComments) }, func(err error) {
		r.saving--
		ok := r.saved(draft, rev, err)
		switch {
		case ok && then != nil:
			then()
		case r.confirm:
			r.pending = then
			for _, f := range r.onDrop {
				f()
			}
		}
	})
}

// write runs backend writes one at a time, in order, so an autosave can
// never land after the save that follows it. work runs on the worker, done
// on the UI goroutine.
func (r *Request) write(work func() error, done func(error)) {
	r.queue = append(r.queue, writeJob{work: work, done: done})
	if !r.writing {
		r.nextWrite()
	}
}

func (r *Request) nextWrite() {
	if len(r.queue) == 0 {
		r.writing = false
		return
	}
	job := r.queue[0]
	r.queue = r.queue[1:]
	r.writing = true
	Run(r.w, job.work, func(err error) {
		job.done(err)
		r.nextWrite()
	})
}

// saved records a save's outcome and reports whether it succeeded.
func (r *Request) saved(draft model.Request, rev int, err error) bool {
	r.confirm, r.err = false, ""
	switch {
	case hasCode(err, diag.CodeWriteDropsComments):
		r.confirm = true
	case err != nil:
		r.err = ErrorText(err)
	default:
		r.disk, r.dirty, r.onDisk, r.slot, r.autosaveProb = draft, r.rev != rev, false, false, "" // edits made during the save stay unsaved
	}
	r.changed()
	return err == nil
}

// Reloaded tells the view-model its file was reloaded; found is false
// when the request no longer exists. Unsaved edits are kept and flagged,
// except when only the edit slot changed (our own autosave).
func (r *Request) Reloaded(c model.Collection, disk model.Request, found bool) {
	r.coll = c
	switch {
	case !found:
		r.err = "this request was removed or its file no longer loads (see Problems)"
	case r.dirty && sameSaved(disk, r.disk):
		r.slot = disk.Edit != nil
	case r.dirty:
		disk.Edit, disk.EditBase = nil, ""
		r.disk, r.onDisk = disk, true
	default:
		r.load(disk)
	}
	r.changed()
}

// sameSaved reports whether two loads of a request hold the same saved
// content, whatever their edit slots hold.
func sameSaved(a, b model.Request) bool {
	a.Edit, a.EditBase, a.EditOverlay, b.Edit, b.EditBase, b.EditOverlay = nil, "", "", nil, "", ""
	return reflect.DeepEqual(a, b)
}

// Revert drops the edits and shows the request as on disk.
func (r *Request) Revert() {
	r.DiscardEdit()
	r.err = ""
}

// DiscardEdit drops the edit, including the one stored in the file, and
// shows the saved request.
func (r *Request) DiscardEdit() { r.DiscardEditThen(nil) }

// DiscardEditThen is DiscardEdit that reports: done(true) once the edit is
// gone from the file too, done(false) (with the problem shown) when it
// could not be cleared. The clear is queued behind any write in flight, so
// an autosave cannot bring the edit back.
func (r *Request) DiscardEditThen(done func(ok bool)) {
	r.cancelAutosave()
	saved := r.disk
	r.write(func() error { return r.b.DiscardEdit(r.target) }, func(err error) {
		if err != nil {
			r.err = ErrorText(err)
			r.changed()
		} else {
			r.load(saved)
			r.changed()
		}
		if done != nil {
			done(err == nil)
		}
	})
}

// SetScheduler enables autosave: edits are written to the file's edit slot
// AutosaveDelay after the last change.
func (r *Request) SetScheduler(s Scheduler) { r.sched = s }

// AutosaveProblem is why the last autosave failed, or "".
func (r *Request) AutosaveProblem() string { return r.autosaveProb }

func (r *Request) scheduleAutosave() {
	r.cancelAutosave()
	if r.sched == nil || r.abandoned {
		return
	}
	r.stopTimer = r.sched.After(AutosaveDelay, func() { r.Flush(nil) })
}

// Abandon is for a closed tab: no autosave or flush writes the edit any more.
// A request deleted from disk would otherwise be written back by a timer that
// outlived its tab.
func (r *Request) Abandon() {
	r.abandoned = true
	r.cancelAutosave()
}

func (r *Request) cancelAutosave() {
	if r.stopTimer != nil {
		r.stopTimer()
		r.stopTimer = nil
	}
}

// Flush writes the edit to the slot now (tab switch, before a send) and
// then runs then. then runs even when there is nothing to write; when the
// write fails the problem is shown (AutosaveProblem) and then is not run.
func (r *Request) Flush(then func()) {
	r.FlushResult(func(ok bool) {
		if ok && then != nil {
			then()
		}
	})
}

// FlushResult is Flush that always reports: done(true) when the edit is
// stored (or there is nothing to store), done(false) when it could not be:
// the write failed, or the changes do not fit the edit slot (a new version,
// a new default) and only Save can keep them.
func (r *Request) FlushResult(done func(stored bool)) {
	r.cancelAutosave()
	if r.abandoned {
		done(true)
		return
	}
	if r.saving > 0 {
		// A save is queued or running: wait for it, then decide (it may
		// have stored everything already).
		r.write(func() error { return nil }, func(error) { r.FlushResult(done) })
		return
	}
	switch {
	case !r.dirty:
		done(true)
	case r.orphan != "":
		r.autosaveProb = orphanText(r.orphan)
		r.changed()
		done(false)
	case !r.slotable():
		r.autosaveProb = "These changes (a new version or default) can only be kept by saving."
		r.changed()
		done(false)
	default:
		base, edit := r.SelectedLabel(), cloneRequest(model.Request{Versions: []model.RequestVersion{r.Current()}}).Versions[0]
		overlay := r.overlay
		r.write(func() error { return r.b.StoreEdit(r.target, base, overlay, edit) }, func(err error) {
			r.autosaved(err)
			done(err == nil)
		})
	}
}

func (r *Request) autosaved(err error) {
	switch {
	case err == nil:
		r.slot, r.autosaveProb = true, ""
	case hasCode(err, diag.CodeWriteDropsComments):
		r.autosaveProb = "Autosave paused: this file has comments. Save to keep your edit (the comments will be dropped)."
	default:
		r.autosaveProb = "Autosave failed: " + ErrorText(err)
	}
	r.changed()
}

// slotable reports whether the unsaved changes fit the edit slot: only the
// selected version differs from the file.
func (r *Request) slotable() bool {
	d, k := r.draft, r.disk
	if d.Default != k.Default || len(d.Versions) != len(k.Versions) {
		return false
	}
	for i := range d.Versions {
		if d.Versions[i].Label != k.Versions[i].Label || (i != r.selected && !reflect.DeepEqual(d.Versions[i], k.Versions[i])) {
			return false
		}
	}
	return true
}

// PrepareSend gets the request ready to send and then calls then. useEdit
// is true when the edit was stored in the slot (the send uses it); other
// unsaved changes are saved instead. then is not called when preparing
// failed (the problem is shown).
func (r *Request) PrepareSend(then func(useEdit bool)) {
	switch {
	case !r.dirty:
		then(false)
	case r.slotable() && r.orphan == "" && r.saving == 0:
		r.FlushResult(func(stored bool) {
			if stored {
				then(true)
				return
			}
			r.SaveThen(false, func() { then(false) }) // e.g. a commented file: asks to drop the comments
		})
	default:
		r.SaveThen(false, func() { then(false) })
	}
}

func hasCode(err error, code diag.Code) bool {
	var le *diag.ListError
	return errors.As(err, &le) && slices.ContainsFunc(le.List, func(d diag.Diagnostic) bool { return d.Code == code })
}

// diagsText renders a send's diagnostics as one text: the errors if there are
// any, else the warnings (a send can pass and still have something to say).
func diagsText(diags diag.List) string {
	if err := diags.Err(); err != nil {
		return ErrorText(err)
	}
	return ErrorText(&diag.ListError{List: diags})
}

// ErrorText renders an error for the user: diagnostics through the
// English catalogue with their codes, anything else as is.
func ErrorText(err error) string {
	if err == nil {
		return ""
	}
	var le *diag.ListError
	if !errors.As(err, &le) {
		return err.Error()
	}
	var lines []string
	for _, d := range le.List {
		msg, hint := diag.English().Message(d)
		line := string(d.Code) + ": " + msg
		if hint != "" {
			line += " (" + hint + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// AvailableSetting is a setting a request version may set, for the Settings
// tab's list of what can be changed.
type AvailableSetting struct {
	Key         string
	Description string
	Default     string   // as typed
	Choices     []string // for an enum
	Set         bool     // the version already sets it
}

// AvailableSettings lists the settings that may be written in a request
// file, in schema order.
func (r *Request) AvailableSettings() []AvailableSetting {
	have := map[string]bool{}
	for _, row := range r.SettingRows() {
		have[row.Key] = true
	}
	var out []AvailableSetting
	for _, k := range settings.Schema() {
		if !slices.Contains(k.FileLayers, settings.LayerRequest) {
			continue
		}
		out = append(out, AvailableSetting{Key: k.Name, Description: k.Description, Default: valueText(k.Default), Choices: k.Enum, Set: have[k.Name]})
	}
	return out
}

// AddSetting adds the setting key to the Settings tab with its default
// value, unless it is already there.
func (r *Request) AddSetting(key string) {
	rows := r.SettingRows()
	if slices.ContainsFunc(rows, func(row SettingRow) bool { return row.Key == key }) {
		return
	}
	for _, s := range r.AvailableSettings() {
		if s.Key == key {
			r.SetSettings(append(rows, SettingRow{Key: key, Raw: s.Default}))
			return
		}
	}
}

// RelativeToRoot is path as the request's collection names a file: relative
// to its root with forward slashes, which is the only form a send accepts
// (a file outside the collection cannot be sent). ok is false for a path
// outside the root.
func (r *Request) RelativeToRoot(path string) (rel string, ok bool) {
	rel, err := filepath.Rel(r.coll.Root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
