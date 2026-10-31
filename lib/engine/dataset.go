// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/githugt/flowi-lib/dataset"
	"github.com/githugt/flowi-lib/dataset/readonly"
	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/vault"
)

// DatasetRunner runs one dataset query. The engine uses dataset.Default; tests pass fakes.
type DatasetRunner interface {
	Run(ctx context.Context, q dataset.Query) (dataset.Result, error)
}

func (r *runner) datasetRunner() DatasetRunner {
	if r.deps.Dataset != nil {
		return r.deps.Dataset
	}
	return dataset.Default()
}

// datasetVerb is the method a dataset request shows where an HTTP one shows GET.
const datasetVerb = "SQL"

// builtDataset is a dataset version rendered and ready to run.
type builtDataset struct {
	query   dataset.Query
	sent    SentRequest // as recorded: secret-bearing fields already masked
	missing []string    // declared secrets with no value (FLW-SEC-001)
}

// datasetExchange is the dataset family: a query goes out, rows come back.
type datasetExchange struct{ x *execution }

func (e datasetExchange) send(ctx context.Context, post []HookRef) { e.x.sendDataset(ctx, post) }

func (e datasetExchange) view() map[string]any {
	b, _ := e.x.renderDataset(true)
	return map[string]any{"method": datasetVerb, "url": b.sent.URL, "headers": map[string]any{}, "body": b.query.Statement}
}

func (datasetExchange) preview(in buildInput, includeSecrets bool) Preview {
	b, diags := buildDataset(in)
	out := Preview{Masked: PreviewRequest{Method: datasetVerb, URL: b.sent.URL, BodyKind: "text", Body: b.sent.Body.Text}, Diags: diags}
	if includeSecrets {
		exact := PreviewRequest{Method: datasetVerb, URL: b.query.Server, BodyKind: "text", Body: shownStatement(b.query.Statement, b.query.Params, nil)}
		out.Real = &exact
	}
	return out
}

// sendDataset is everything after the pre-hooks for a dataset request: render, run the query, record it, then
// extract, assert, post-hooks and save, in the order an HTTP request has.
func (x *execution) sendDataset(ctx context.Context, post []HookRef) {
	b, ok := x.buildDataset()
	if !ok {
		return
	}
	x.emitDatasetSent(b)
	res, err := x.r.datasetRunner().Run(ctx, b.query)
	if ctx.Err() != nil {
		x.fail(StatusCancelled, nil)
		return
	}
	x.record = x.newDatasetRecord(b, res, err)
	x.emitDatasetReceived(res, err)
	if err != nil {
		x.failDataset(err)
		x.save()
		return
	}
	x.observeDataset(res)
	x.extractOutputs()
	x.checkAssertions()
	x.runPhase(ctx, post, nodes.PhasePost)
	if ctx.Err() == nil {
		x.save()
		x.checkSchemas()
	}
}

func (x *execution) emitDatasetSent(b builtDataset) {
	x.r.emit(RequestSent{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), RequestPath: x.c.request.Path,
		Version: x.version.Label, VersionSource: x.source, Method: datasetVerb, URL: b.sent.URL, Attempt: 1, Depth: x.c.depth})
}

func (x *execution) emitDatasetReceived(res dataset.Result, err error) {
	ev := ResponseReceived{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), Duration: res.Duration, Attempt: 1,
		Depth: x.c.depth, Outcome: datasetOutcome(err)}
	if err != nil {
		ev.Error = x.r.secrets.String(err.Error())
	}
	x.r.emit(ev)
}

// datasetOutcome is how a query ended: ok, timeout, or error for anything else.
func datasetOutcome(err error) string {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, dataset.ErrTimeout):
		return OutcomeTimeout
	}
	return OutcomeError
}

// failDataset turns an error into the request's diagnostics. A query the guard refused or a driver that does not
// exist is a mistake in the request; anything else is what the store said.
func (x *execution) failDataset(err error) {
	switch {
	case errors.Is(err, dataset.ErrTimeout):
		x.fail(StatusError, diag.List{diag.New(diag.CodeDatasetTimeout, diag.Location{}, map[string]string{"timeout": timeoutOf(err)})})
	case isRequestMistake(err):
		x.fail(StatusError, diag.List{diag.New(diag.CodeDatasetRequest, diag.Location{}, map[string]string{"detail": x.r.secrets.String(err.Error())})})
	default:
		x.fail(StatusError, diag.List{diag.New(diag.CodeDatasetQuery, diag.Location{}, map[string]string{"detail": x.r.secrets.String(err.Error())})})
	}
}

func isRequestMistake(err error) bool {
	return errors.Is(err, dataset.ErrUnknownDriver) || errors.Is(err, readonly.ErrNotReadOnly) ||
		errors.Is(err, readonly.ErrManyStatements) || errors.Is(err, readonly.ErrEmptyStatement)
}

// timeoutOf is the duration an ErrTimeout says it waited ("after 30s"), "the timeout" when it says none.
func timeoutOf(err error) string {
	_, after, found := strings.Cut(err.Error(), " after ")
	if !found {
		return "the timeout"
	}
	return after
}

// buildDataset renders the version strictly, after the pre-hooks.
func (x *execution) buildDataset() (builtDataset, bool) {
	b, diags := x.renderDataset(false)
	diags = append(diags, x.r.secrets.problems(x.env.name, b.missing)...)
	if diags.HasErrors() {
		x.fail(StatusError, diags)
		return builtDataset{}, false
	}
	return b, true
}

func (x *execution) renderDataset(lenient bool) (builtDataset, diag.List) {
	b, diags := buildDataset(buildInput{
		version: x.version, scope: x.scope(lenient), empty: x.cfg.empty, label: x.c.request.Path,
		root: x.c.collection.Root, secretVars: x.env.secrets, missingSecret: x.r.secrets.isMissing(x.env.name), memo: x.memo,
		secret: x.r.secrets.derived, verbatim: lenient,
	})
	b.query.Timeout = x.cfg.timeout
	return b, diags
}

// buildDataset renders the templated fields of a dataset version: the server and the params. The statement is sent
// as written; a value reaches the store only as a param.
func buildDataset(in buildInput) (builtDataset, diag.List) {
	d := *in.version.Dataset
	b := &builder{in: in, tainted: map[string]bool{}}
	server, _ := b.text("server", "server", d.Server)
	params := make([]string, len(d.Params))
	secretParam := make([]bool, len(d.Params))
	for i, p := range d.Params {
		key := fmt.Sprintf("param:%d", i)
		params[i], _ = b.text(fmt.Sprintf("param $%d", i+1), key, p)
		secretParam[i] = b.tainted[key]
	}
	out := builtDataset{
		query:   dataset.Query{Driver: d.Driver, Server: server, Statement: d.Query, Params: params, Limit: d.Limit, Root: in.root},
		missing: b.missingSecrets,
	}
	shownServer := model.RedactedTarget(server)
	if b.tainted["server"] {
		shownServer = vault.Mask
	}
	shown := shownStatement(d.Query, params, secretParam)
	out.sent = SentRequest{Method: datasetVerb, URL: shownServer, Body: &Body{Text: shown, Size: int64(len(shown)), ContentType: "application/sql"}}
	return out, b.diags
}

// shownStatement is the statement followed by its params as SQL comments, so what is shown still runs when it is
// copied. A param that holds a secret shows as masked.
func shownStatement(statement string, params []string, secret []bool) string {
	if len(params) == 0 {
		return statement
	}
	var out strings.Builder
	out.WriteString(strings.TrimRight(statement, "\n"))
	for i, p := range params {
		value := p
		if i < len(secret) && secret[i] {
			value = vault.Mask
		}
		fmt.Fprintf(&out, "\n-- $%d = %s", i+1, value)
	}
	return out.String()
}

// datasetBody is the result as one JSON object: columns, rows as objects by column name, how many, and whether more
// were cut. The same object is what extraction and assertions read as body.
func datasetBody(res dataset.Result) map[string]any {
	rows := make([]any, len(res.Rows))
	for i, row := range res.Rows {
		object := make(map[string]any, len(res.Columns))
		for j, name := range res.Columns {
			object[name] = row[j]
		}
		rows[i] = object
	}
	columns := make([]any, len(res.Columns))
	for i, name := range res.Columns {
		columns[i] = name
	}
	return map[string]any{"columns": columns, "rows": rows, "row_count": len(rows), "truncated": res.Truncated}
}

// Rows is how many rows a dataset record kept and whether the store had more; zero for any other record. It reads
// the body, so it works on a record read back from a file as well as one just made.
func (r *Record) Rows() (count int, truncated bool) {
	if r.Kind != KindDataset {
		return 0, false
	}
	body, ok := r.Body.JSON.(map[string]any)
	if !ok {
		return 0, false
	}
	truncated, _ = body["truncated"].(bool)
	switch n := body["row_count"].(type) {
	case int:
		return n, truncated
	case float64: // a number read back from JSON
		return int(n), truncated
	case json.Number: // the number of a body parsed as it was received
		parsed, err := n.Int64()
		if err != nil {
			return 0, truncated
		}
		return int(parsed), truncated
	}
	return 0, truncated
}

// newDatasetRecord builds the record of a query, whatever its outcome.
func (x *execution) newDatasetRecord(b builtDataset, res dataset.Result, err error) *Record {
	rec := &Record{
		ID: x.r.deps.NewID(), RunID: x.r.runID, CollectionID: x.c.collection.Manifest.ID, RequestID: x.c.request.ID,
		RequestPath: x.c.request.Path, Version: x.version.Label, VersionSource: x.source, At: x.r.deps.Clock(),
		Request: b.sent, Kind: KindDataset, Proto: b.query.Driver, Outcome: datasetOutcome(err), Overlay: x.c.overlayName(),
		Timings:  Timings{Total: res.Duration},
		Attempts: []AttemptRecord{{Duration: res.Duration}},
	}
	if err != nil {
		rec.Attempts[0].Error = x.r.secrets.String(err.Error())
		return rec
	}
	data, _ := json.Marshal(datasetBody(res)) // plain values and strings only: Marshal cannot fail
	rec.Body = messageBody(data)
	return rec
}

// observeDataset makes the result visible to extraction, assertions and post-hooks: body, rows, columns, row_count,
// truncated and outcome. status is not defined for a dataset request.
func (x *execution) observeDataset(res dataset.Result) {
	body := datasetBody(res)
	x.mu.Lock()
	x.async = map[string]any{
		"body": body, "rows": body["rows"], "columns": body["columns"], "row_count": body["row_count"],
		"truncated": body["truncated"], "outcome": x.record.Outcome,
	}
	x.mu.Unlock()
	x.state.Response = &nodes.Response{Body: body, Text: x.record.Body.Text, Time: res.Duration}
	x.state.Request = nil
}
