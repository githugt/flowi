// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/internal/names"
	"github.com/githugt/flowi/lib/model"
)

// Where an entity came from, as Activity.Origin says it.
const (
	OriginCreated    = "created"    // made here, from scratch
	OriginDuplicated = "duplicated" // a copy of another request or flow
	OriginImported   = "imported"   // made by an import (OpenAPI, HAR, Postman, …)
	OriginRecorded   = "recorded"   // made by the replayer from a call no saved request matched
	OriginFound      = "found"      // it was already in the files when this client first saw it (cloned, pulled, copied in)
)

// Activity is what this client knows about the life of one request or flow: where it came from, when, and how its
// runs went. It lives in the workspace data directory, not in the request or flow file, so it is never committed
// and never differs between people. Because of that it may be missing for something that arrived through git or a
// copy of the files: the first look then starts it, with Origin OriginFound and the first-seen time, and the run
// facts fill in from then on. Nothing here is needed to run anything.
type Activity struct {
	Kind         string // "request" or "flow"
	ID           model.ID
	Origin       string
	OriginDetail string    // "HAR import", "copy of auth/login", "replayer: GET /v1/users/7"; may be empty
	Created      time.Time // for OriginFound, the file's time when it is older than the first look, else the first look
	Runs         int
	LastRun      time.Time
	LastStatus   string // the status of the last run: passed, failed, error, cancelled
	LastSuccess  time.Time
	LastFailure  time.Time
}

// Activity kinds.
const (
	activityRequest = "request"
	activityFlow    = "flow"
)

// RequestActivity is the activity of request t, started now when this client has none yet.
func (w *Workspace) RequestActivity(t RequestTarget) (Activity, error) {
	_, r, err := w.requestFor(t)
	if err != nil {
		return Activity{}, err
	}
	return w.activityOf(t.Collection, activityRequest, r.ID, r.File), nil
}

// FlowActivity is the activity of flow t, started now when this client has none yet.
func (w *Workspace) FlowActivity(t FlowTarget) (Activity, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return Activity{}, err
	}
	return w.activityOf(t.Collection, activityFlow, f.ID, f.File), nil
}

func (w *Workspace) activityFile(collection model.ID, kind string, id model.ID) string {
	if !names.Valid(string(id)) {
		return ""
	}
	return filepath.Join(w.DataDir(collection), "activity", kind+"s", string(id)+".json")
}

// activityOf reads the activity, or starts it for something that has none (see Activity). entityFile is where the
// request or flow is kept, to date a first look from.
func (w *Workspace) activityOf(collection model.ID, kind string, id model.ID, entityFile string) Activity {
	w.activityMu.Lock()
	defer w.activityMu.Unlock()
	if a, ok := w.readActivity(collection, kind, id); ok {
		return a
	}
	now := time.Now()
	a := Activity{Kind: kind, ID: id, Origin: OriginFound, Created: now}
	if fi, err := os.Stat(entityFile); err == nil && fi.ModTime().Before(now) {
		a.Created = fi.ModTime()
	}
	w.writeActivity(collection, a)
	return a
}

// readActivityLocked is readActivity for a caller that does not hold the lock.
func (w *Workspace) readActivityLocked(collection model.ID, kind string, id model.ID) (Activity, bool) {
	w.activityMu.Lock()
	defer w.activityMu.Unlock()
	return w.readActivity(collection, kind, id)
}

// readActivity reads the stored activity; a missing or unreadable file is "none", never an error: it is a cache.
func (w *Workspace) readActivity(collection model.ID, kind string, id model.ID) (Activity, bool) {
	file := w.activityFile(collection, kind, id)
	if file == "" {
		return Activity{}, false
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return Activity{}, false
	}
	var a Activity
	if json.Unmarshal(data, &a) != nil || a.ID != id {
		return Activity{}, false
	}
	return a, true
}

// writeActivity stores a through a temporary file. A failure is dropped: the facts are a convenience, and a send
// or an edit must not fail because they could not be kept.
func (w *Workspace) writeActivity(collection model.ID, a Activity) {
	file := w.activityFile(collection, a.Kind, a.ID)
	if file == "" {
		return
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil || os.MkdirAll(filepath.Dir(file), 0o750) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(file), ".activity-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), file) != nil {
		_ = os.Remove(tmp.Name())
	}
}

// noteOrigin records where a request or flow came from, replacing what was there: the entity was just made.
func (w *Workspace) noteOrigin(collection model.ID, kind string, id model.ID, origin, detail string) {
	w.activityMu.Lock()
	defer w.activityMu.Unlock()
	a, _ := w.readActivity(collection, kind, id)
	a.Kind, a.ID, a.Origin, a.OriginDetail = kind, id, origin, detail
	if a.Created.IsZero() || origin != OriginFound {
		a.Created = time.Now()
	}
	w.writeActivity(collection, a)
}

// noteRun records how a run of a request or flow ended. A run that was cancelled is counted, but is neither a
// success nor a failure.
func (w *Workspace) noteRun(collection model.ID, kind string, id model.ID, status engine.Status, entityFile string) {
	w.activityMu.Lock()
	defer w.activityMu.Unlock()
	a, ok := w.readActivity(collection, kind, id)
	if !ok {
		a = Activity{Kind: kind, ID: id, Origin: OriginFound, Created: time.Now()}
		if fi, err := os.Stat(entityFile); err == nil && fi.ModTime().Before(a.Created) {
			a.Created = fi.ModTime()
		}
	}
	now := time.Now()
	a.Runs++
	a.LastRun, a.LastStatus = now, string(status)
	switch status {
	case engine.StatusPassed:
		a.LastSuccess = now
	case engine.StatusFailed, engine.StatusError:
		a.LastFailure = now
	}
	w.writeActivity(collection, a)
}

// onSend is the engine hook that notes the end of a send in the request's activity.
func (w *Workspace) onSend(t RequestTarget, file string) func(engine.SendResult) {
	return func(res engine.SendResult) {
		if res.Status != "" {
			w.noteRun(t.Collection, activityRequest, t.Request, res.Status, file)
		}
	}
}

// onRun is the engine hook that notes the end of a flow run in the flow's activity.
func (w *Workspace) onRun(t FlowTarget, file string) func(engine.RunResult) {
	return func(res engine.RunResult) {
		if res.Status != "" {
			w.noteRun(t.Collection, activityFlow, t.Flow, res.Status, file)
		}
	}
}

// forgetActivity drops what is kept for an entity that was deleted.
func (w *Workspace) forgetActivity(collection model.ID, kind string, id model.ID) {
	if file := w.activityFile(collection, kind, id); file != "" {
		_ = os.Remove(file) // a file that is not there is what was wanted
	}
}
