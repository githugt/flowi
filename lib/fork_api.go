// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/engine"
	"github.com/githugt/flowi/lib/internal/tapeplayer"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/report"
)

// ForkOptions say where a recorded run branches and what changes there.
type ForkOptions struct {
	// From is the top-level step that runs again, with every step after it. The steps before it keep what the
	// recording says they made.
	From string
	// Set are variables changed before From runs, over the ones the recording ended the earlier steps with.
	Set map[string]any
	// Only runs just the From step again, and not the ones after it.
	Only bool
	// Respond gives a step's request another response than the recording has, by top-level step id. The step must
	// be one that runs in this fork and calls a request.
	Respond map[string]ForkResponse
}

// ForkResponse is the response a forked step is answered with in place of the recorded one.
type ForkResponse struct {
	// Status replaces the recorded status when not 0.
	Status int
	// Body replaces the recorded body when HasBody is set: a JSON value (a map, list, number, string, bool), or
	// nil for a body of JSON null.
	Body    any
	HasBody bool
	// Text replaces the recorded body when HasText is set, for a response that is not JSON (an XML or HTML page, a
	// CSV, a plain message). The step then sees text and no parsed value, as it would from a server that answered so.
	// ContentType, when set, is the body's content type. Text and Body cannot both be given.
	Text        string
	HasText     bool
	ContentType string
}

// ForkResult is a fork and how it went differently from the recording.
type ForkResult struct {
	Run engine.RunResult
	// Comparison covers only the steps that ran again.
	Comparison report.Comparison
}

// Fork runs the bundled flow again from a step with edited variables, and waits for it. Calls are answered from the
// recording, so no server is asked: a call the edit made different (another path) is reported as not recorded, which
// says that the edit changed what the flow sends, and the recording cannot answer for it.
func (b *BundleRun) Fork(ctx context.Context, o ForkOptions) (ForkResult, error) {
	run, err := b.StartFork(ctx, o)
	if err != nil {
		return ForkResult{}, err
	}
	for range run.Events() { //nolint:revive // draining the events lets the run end
	}
	res := run.Wait()
	return ForkResult{Run: res, Comparison: b.CompareFork(res)}, nil
}

// StartFork starts a fork as Fork does and returns the run to watch, for a caller that shows it as it goes.
func (b *BundleRun) StartFork(ctx context.Context, o ForkOptions) (*engine.Run, error) {
	return b.w.startFork(ctx, b.Target, b.Options, b.Original, o)
}

// ForkVariables are the variables as the recording left them before step from: what a fork starts with, frozen at the
// moment of the recording.
func (b *BundleRun) ForkVariables(from string) (map[string]any, error) {
	_, f, err := b.w.flowFor(b.Target)
	if err != nil {
		return nil, err
	}
	at := slices.IndexFunc(f.Nodes, func(n model.Node) bool { return n.ID == from })
	if at < 0 {
		return nil, fmt.Errorf("the flow has no step %q to fork from; its steps are %s", from, strings.Join(topLevelIDs(f), ", "))
	}
	_, vars := recordedBefore(b.Original.Report, f.Nodes[:at], nil)
	return vars, nil
}

// ForkReplay forks the flow t of a collection opened from a bundle (see KeepBundle), against the run it holds. It
// waits for the fork to end. t in any other collection is an error: there is no recording to answer from.
func (w *Workspace) ForkReplay(ctx context.Context, t FlowTarget, o ForkOptions) (ForkResult, error) {
	w.mu.RLock()
	base, ok := w.replays[t.Collection]
	w.mu.RUnlock()
	if !ok {
		return ForkResult{}, fmt.Errorf("this flow was not opened from a run bundle: there is no recording to fork")
	}
	run, err := w.startFork(ctx, t, base.options, base.original, o)
	if err != nil {
		return ForkResult{}, err
	}
	for range run.Events() { //nolint:revive // draining the events lets the run end
	}
	res := run.Wait()
	return ForkResult{Run: res, Comparison: compareFork(base.original, res)}, nil
}

func (w *Workspace) startFork(ctx context.Context, t FlowTarget, base RunOptions, original engine.RunResult, o ForkOptions) (*engine.Run, error) {
	_, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	at := slices.IndexFunc(f.Nodes, func(n model.Node) bool { return n.ID == o.From })
	if at < 0 {
		return nil, fmt.Errorf("the flow has no step %q to fork from; its steps are %s", o.From, strings.Join(topLevelIDs(f), ", "))
	}
	running := f.Nodes[at:]
	if o.Only {
		running = f.Nodes[at : at+1]
	}
	base.Only = topLevelIDs(model.Flow{Nodes: running})
	base.Prior, base.PriorVars = recordedBefore(original.Report, f.Nodes[:at], o.Set)
	if len(o.Respond) > 0 {
		if base.answers, err = answerWith(original.Captured, running, o.Respond); err != nil {
			return nil, err
		}
		base.Transport = tapeplayer.New(base.answers)
	}
	return w.RunFlow(ctx, t, base)
}

// answerWith is the records a fork answers from: the recorded ones, except that every recorded call of the request a
// step in respond calls carries the response given for it. The recording itself is not touched. A step that does
// not run in this fork, or calls no request, is an error: the response would silently go unused.
func answerWith(recorded []engine.Record, running []model.Node, respond map[string]ForkResponse) ([]engine.Record, error) {
	out := slices.Clone(recorded)
	for _, id := range slices.Sorted(maps.Keys(respond)) {
		i := slices.IndexFunc(running, func(n model.Node) bool { return n.ID == id })
		if i < 0 {
			return nil, fmt.Errorf("step %q does not run in this fork, so there is no call to answer; the steps that run are %s", id, strings.Join(topLevelIDs(model.Flow{Nodes: running}), ", "))
		}
		n := running[i]
		if n.Type != "request" || n.Request == nil {
			return nil, fmt.Errorf("step %q is a %s step: only a step that calls a request can be answered with a response", id, n.Type)
		}
		if respond[id].HasBody && respond[id].HasText {
			return nil, fmt.Errorf("step %q is given both a JSON body and a text body: choose one", id)
		}
		replaced := 0
		for j, rec := range out {
			if recordIsOf(rec, n.Request.Request) {
				out[j] = withResponse(rec, respond[id])
				replaced++
			}
		}
		if replaced == 0 {
			return nil, fmt.Errorf("the recording has no call of the request step %q makes, so there is no response to replace", id)
		}
	}
	return out, nil
}

// recordIsOf reports that a record is a call of the request ref names, by id or by path.
func recordIsOf(rec engine.Record, ref model.Ref) bool {
	return (ref.ID != "" && rec.RequestID == ref.ID) || (ref.Path != "" && rec.RequestPath == ref.Path)
}

// withResponse is rec answered with r instead: its status and/or its body, the headers and the rest kept.
func withResponse(rec engine.Record, r ForkResponse) engine.Record {
	if r.Status != 0 {
		rec.Status = r.Status
	}
	switch {
	case r.HasText:
		rec.Header = withContentType(rec.Header, r.ContentType)
		rec.Body = engine.Body{Text: r.Text, Size: int64(len(r.Text)), ContentType: contentTypeOrText(r.ContentType)}
	case r.HasBody:
		rec.Body = engine.Body{JSON: r.Body}
		if r.Body == nil {
			rec.Body = engine.Body{Text: "null"}
		}
	}
	return rec
}

// textContentType is the content type of a text response given without one.
const textContentType = "text/plain; charset=utf-8"

func contentTypeOrText(given string) string {
	if given == "" {
		return textContentType
	}
	return given
}

// withContentType is headers with the Content-Type replaced: the recorded one says JSON, and a text body under it
// would be read as JSON that does not parse.
func withContentType(headers []engine.Header, contentType string) []engine.Header {
	out := slices.DeleteFunc(slices.Clone(headers), func(h engine.Header) bool { return strings.EqualFold(h.Name, "content-type") })
	return append(out, engine.Header{Name: "Content-Type", Value: contentTypeOrText(contentType)})
}

// CompareFork says how a finished fork went differently from the recording, for the steps that ran again.
func (b *BundleRun) CompareFork(res engine.RunResult) report.Comparison {
	return compareFork(b.Original, res)
}

// compareFork is how the steps a fork ran went differently from the recording, and which of their calls were
// answered differently: a response given with Respond changes no step's result by itself, so without this the
// comparison would say nothing happened.
func compareFork(original, forked engine.RunResult) report.Comparison {
	c := onlyRerun(report.Compare(original.Report, forked.Report), forked.Report)
	c.Changes = append(c.Changes, callChanges(original.Captured, forked.Captured)...)
	if c.FirstDivergence == "" && len(c.Changes) > 0 {
		c.FirstDivergence = c.Changes[0].Node
	}
	return c
}

// callChanges lists the HTTP calls the fork got another status or body for than the recording, pairing the n-th call
// of a request in the fork with the n-th in the recording. They are named by request path: a record does not know
// the step it belongs to.
func callChanges(recorded, forked []engine.Record) []report.Change {
	var out []report.Change
	seen := map[string]int{}
	for _, rec := range forked {
		if rec.HasMessages() {
			continue
		}
		n := seen[rec.RequestPath]
		seen[rec.RequestPath]++
		was, ok := nthCall(recorded, rec.RequestPath, n)
		if !ok {
			continue
		}
		if was.Status != rec.Status {
			out = append(out, report.Change{Node: rec.RequestPath, What: "call status", From: strconv.Itoa(was.Status), To: strconv.Itoa(rec.Status)})
		}
		if bodyJSON(was.Body) != bodyJSON(rec.Body) {
			out = append(out, report.Change{Node: rec.RequestPath, What: "call body", From: "as recorded", To: "answered with another body"})
		}
	}
	return out
}

func nthCall(records []engine.Record, path string, n int) (engine.Record, bool) {
	for _, rec := range records {
		if rec.RequestPath != path || rec.HasMessages() {
			continue
		}
		if n == 0 {
			return rec, true
		}
		n--
	}
	return engine.Record{}, false
}

// bodyJSON is a body as canonical JSON text, so that the same value compares equal however the number was typed when
// it was read; a body that is not JSON compares by its text.
func bodyJSON(b engine.Body) string {
	if b.JSON == nil {
		return strings.TrimSpace(b.Text)
	}
	data, err := json.Marshal(b.JSON)
	if err != nil {
		return strings.TrimSpace(b.Text)
	}
	return string(data)
}

func topLevelIDs(f model.Flow) []string {
	ids := make([]string, len(f.Nodes))
	for i, n := range f.Nodes {
		ids[i] = n.ID
	}
	return ids
}

// recordedBefore is what the steps before the fork made: their outputs, and the variables as they left them with
// the edits laid over.
func recordedBefore(rec report.Run, before []model.Node, set map[string]any) (map[string]map[string]any, map[string]any) {
	prior, vars := map[string]map[string]any{}, map[string]any{}
	for _, n := range before {
		for _, r := range rec.Nodes {
			if r.ID != n.ID {
				continue
			}
			if r.Outputs != nil {
				prior[n.ID] = r.Outputs
			}
			for _, w := range writesWithin(r) {
				vars[w.Name] = w.Value
			}
		}
	}
	for k, v := range set {
		vars[k] = v
	}
	return prior, vars
}

// writesWithin lists the variables a step and the steps inside it wrote, in order.
func writesWithin(n report.Node) []report.VarWrite {
	out := slices.Clone(n.Wrote)
	for _, lane := range n.Lanes {
		for _, inner := range lane.Nodes {
			out = append(out, writesWithin(inner)...)
		}
	}
	return out
}

// onlyRerun keeps the changes about steps the fork ran.
func onlyRerun(c report.Comparison, forked report.Run) report.Comparison {
	ids := map[string]bool{}
	var walk func(list []report.Node)
	walk = func(list []report.Node) {
		for _, n := range list {
			ids[n.ID] = true
			for _, l := range n.Lanes {
				walk(l.Nodes)
			}
		}
	}
	walk(forked.Nodes)
	out := report.Comparison{Changes: []report.Change{}}
	for _, ch := range c.Changes {
		if ids[ch.Node] {
			out.Changes = append(out.Changes, ch)
		}
	}
	if len(out.Changes) > 0 {
		out.FirstDivergence = out.Changes[0].Node
	}
	return out
}

// ForkVariables are the variables as the recording left them before step from of flow t: what a fork starts with,
// and what a person can change. t must be a flow of a collection opened from a bundle.
func (w *Workspace) ForkVariables(t FlowTarget, from string) (map[string]any, error) {
	w.mu.RLock()
	base, ok := w.replays[t.Collection]
	w.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("this flow was not opened from a run bundle: there is no recording to fork")
	}
	_, f, err := w.flowFor(t)
	if err != nil {
		return nil, err
	}
	at := slices.IndexFunc(f.Nodes, func(n model.Node) bool { return n.ID == from })
	if at < 0 {
		return nil, fmt.Errorf("the flow has no step %q to fork from; its steps are %s", from, strings.Join(topLevelIDs(f), ", "))
	}
	_, vars := recordedBefore(base.original.Report, f.Nodes[:at], nil)
	return vars, nil
}
