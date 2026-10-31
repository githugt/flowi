// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/internal/nodes"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
	"github.com/githugt/flowi/lib/transport"
	transportall "github.com/githugt/flowi/lib/transport/all"
)

// asyncMaxBytes stops a receive whose messages add up to more than this: a
// message request that never ends must not fill memory.
const asyncMaxBytes = 64 << 20

// builtAsync is an async version rendered and ready to run.
type builtAsync struct {
	op       transport.Operation
	sent     SentRequest // as recorded: secret-bearing fields already masked
	missing  []string    // declared secrets with no value (FLW-SEC-001)
	protocol string
}

// registry is the transports this run uses.
func (r *runner) registry() transport.Registry {
	if r.deps.Transports != nil {
		return r.deps.Transports
	}
	return transportall.New()
}

// sendAsync is everything after the pre-hooks for an async request: render,
// run the transport, record the messages, then extract, assert, post-hooks
// and save, in the order an HTTP request has.
func (x *execution) sendAsync(ctx context.Context, post []HookRef) {
	b, ok := x.buildAsync()
	if !ok {
		return
	}
	tr, ok := x.r.registry().Lookup(b.protocol)
	if !ok {
		x.fail(StatusError, diag.List{diag.New(diag.CodeAsyncProtocol, diag.Location{}, map[string]string{"protocol": b.protocol})})
		return
	}
	x.emitAsyncSent(b)
	if b.op.Receive != nil {
		seq := 0
		b.op.Receive.OnMessage = func(m transport.Message) {
			seq++
			x.r.emit(MessageReceived{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), Seq: seq, Depth: x.c.depth,
				Message: recordMessage(m).masked(x.r.secrets.current())})
		}
		if len(x.messageHooks) > 0 {
			b.op.Receive.OnKept = func(m transport.Message) error { return x.runMessageHooks(ctx, seq, m) }
		}
	}
	res, err := x.runAsync(ctx, tr, b)
	if ctx.Err() != nil {
		x.fail(StatusCancelled, nil)
		return
	}
	x.record = x.newAsyncRecord(b, res, err)
	x.emitAsyncReceived(res, err)
	x.observeAsync(res)
	if err != nil {
		if !errors.Is(err, errMessageHook) {
			x.failAsync(err, res) // a failed message hook has reported itself
		}
		x.save() // the messages that did arrive are worth keeping
		return
	}
	x.extractOutputs()
	x.checkAssertions()
	x.runPhase(ctx, post, nodes.PhasePost)
	if ctx.Err() == nil {
		x.save()
		x.checkSchemas()
	}
}

// emitAsyncSent and emitAsyncReceived report the run the way an HTTP request
// reports its attempt, so the pipeline view and the run log follow an async
// request too. Masked: the target may hold credentials.
func (x *execution) emitAsyncSent(b builtAsync) {
	x.r.emit(RequestSent{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), RequestPath: x.c.request.Path,
		Version: x.version.Label, VersionSource: x.source, Method: b.sent.Method, URL: x.r.secrets.String(b.sent.URL), Attempt: 1, Depth: x.c.depth})
}

func (x *execution) emitAsyncReceived(res transport.Result, err error) {
	ev := ResponseReceived{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), Duration: res.Total, Attempt: 1,
		Depth: x.c.depth, Outcome: outcomeOf(err)}
	if err != nil {
		ev.Error = x.r.secrets.String(err.Error())
	}
	x.r.emit(ev)
}

// runTransport runs the operation with a ceiling of the request timeout for
// connecting plus the time the receive may wait.
func (x *execution) runTransport(ctx context.Context, tr transport.Transport, b builtAsync) (transport.Result, error) {
	budget := x.cfg.timeout
	if rc := b.op.Receive; rc != nil {
		budget += rc.Normalized().Timeout
	}
	if budget > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, budget)
		defer cancel()
	}
	return tr.Run(ctx, b.op)
}

// failAsync turns a transport error into the request's diagnostics.
func (x *execution) failAsync(err error, res transport.Result) {
	if errors.Is(err, transport.ErrTimeout) {
		rc := normalizedReceive(x.version.Async.Receive)
		x.fail(StatusError, diag.List{diag.New(diag.CodeAsyncTimeout, diag.Location{}, map[string]string{
			"got": itoa(len(res.Received)), "want": itoa(rc.Count), "timeout": rc.Timeout.String()})})
		return
	}
	var ce connectionError
	if errors.As(err, &ce) {
		x.fail(StatusError, diag.List{diag.New(diag.CodeAsyncConn, diag.Location{}, map[string]string{"detail": ce.Error()})})
		return
	}
	x.fail(StatusError, diag.List{diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()})})
}

// buildAsync renders the version strictly, after the pre-hooks.
func (x *execution) buildAsync() (builtAsync, bool) {
	b, diags := x.renderAsync(false)
	diags = append(diags, x.r.secrets.problems(x.env.name, b.missing)...)
	if diags.HasErrors() {
		x.fail(StatusError, diags)
		return builtAsync{}, false
	}
	return b, true
}

func (x *execution) renderAsync(lenient bool) (builtAsync, diag.List) {
	in := buildInput{
		version: x.version, scope: x.scope(lenient), empty: x.cfg.empty, label: x.c.request.Path,
		root: x.c.collection.Root, secretVars: x.env.secrets, missingSecret: x.r.secrets.isMissing(x.env.name), memo: x.memo,
		secret: x.r.secrets.derived, verbatim: lenient,
	}
	b, diags := buildAsync(in)
	tlsCfg, err := httpx.TLSConfig(x.cfg.caFiles)
	if err != nil {
		diags = append(diags, diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()}))
	}
	b.op.TLS = tlsCfg
	b.op.Clock = x.r.deps.Clock
	if rc := b.op.Receive; rc != nil && !lenient {
		rc.Match = x.matcher(x.version.Async.Receive.Match)
	}
	return b, diags
}

// buildAsync renders every templated field of an async version.
func buildAsync(in buildInput) (builtAsync, diag.List) {
	a := *in.version.Async
	// The existing HTTP field renderers read the version, so give them the
	// parts of it that are async: the connection headers and the sent body.
	v := in.version
	v.Body = model.Body{}
	if a.Send != nil {
		v.Body = a.Send.Body
	}
	in.version = v
	b := &builder{in: in, tainted: map[string]bool{}}
	out := builtAsync{protocol: a.Protocol}
	server, _ := b.text("server", "server", a.Server)
	server = model.NormalizeServer(a.Protocol, server)
	channel, _ := b.text("channel", "channel", a.Channel)
	conn, shownConn := b.headers()
	op := transport.Operation{Server: server, Channel: channel, Binding: map[string]string{}}
	for name, values := range conn {
		for _, val := range values {
			op.Headers = append(op.Headers, transport.Header{Name: name, Value: val})
		}
	}
	for _, kv := range a.Binding {
		if val, ok := b.text("binding "+kv.Key, "binding:"+kv.Key, kv.Value); ok {
			op.Binding[kv.Key] = val
		}
	}
	shownHeaders := shownConn
	var shownBody *Body
	if a.Send != nil {
		op.Send, shownHeaders, shownBody = renderSend(b, a.Send, shownConn)
	}
	if a.Receive != nil {
		rc := normalizedReceive(a.Receive)
		op.Receive = &transport.Receive{Count: rc.Count, Timeout: rc.Timeout, MaxBytes: asyncMaxBytes}
	}
	out.op = op
	out.sent = SentRequest{
		Method: model.ProtocolVerb(a.Protocol), URL: shownTarget(b, a.Protocol, server, channel), Header: shownHeaders, Body: shownBody,
	}
	out.missing = b.missingSecrets
	return out, b.diags
}

// renderSend renders the message to send: its body (through the HTTP body
// renderers, so JSON, text, form and file bodies work the same) and headers.
func renderSend(b *builder, s *model.AsyncSend, shown []Header) (*transport.Message, []Header, *Body) {
	msg := &transport.Message{}
	src, sentBody := b.body()
	if src != nil {
		data, err := readBody(src)
		if err != nil {
			b.fileError("message body", err)
		}
		msg.Body = data
	}
	for _, kv := range s.Headers {
		val, ok := b.text("message header "+kv.Key, "mheader:"+kv.Key, kv.Value)
		if !ok {
			continue
		}
		msg.Headers = append(msg.Headers, transport.Header{Name: kv.Key, Value: val})
		shown = append(shown, Header{Name: kv.Key, Value: maskIf(b.tainted["mheader:"+kv.Key], val)})
	}
	return msg, shown, sentBody
}

func readBody(src httpx.BodySource) ([]byte, error) {
	r, _, _, err := src.Open()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// shownTarget is the server and channel as recorded; a secret in the server
// (credentials in the URL) hides all of it.
func shownTarget(b *builder, protocol, server, channel string) string {
	if b.tainted["server"] {
		return "(masked)"
	}
	return model.RedactedTarget(model.TargetURL(protocol, server, channel))
}

// normalizedReceive fills the receive's defaults.
func normalizedReceive(r *model.AsyncReceive) model.AsyncReceive {
	rc := model.AsyncReceive{}
	if r != nil {
		rc = *r
	}
	if rc.Count <= 0 {
		rc.Count = transport.DefaultCount
	}
	if rc.Timeout <= 0 {
		rc.Timeout = transport.DefaultTimeout
	}
	return rc
}

// matcher turns the receive's match expression into the filter a transport
// applies to each message; nil when there is none.
func (x *execution) matcher(expr string) func(transport.Message) (bool, error) {
	if strings.TrimSpace(expr) == "" {
		return nil
	}
	return func(m transport.Message) (bool, error) {
		// Lenient: a filter skips messages, so a message without the field it
		// reads is not a match, not a failure of the whole receive.
		s := x.scope(true)
		msg := scopeMessage(recordMessage(m))
		s.Values["body"], s.Values["headers"] = msg["body"], msg["headers"]
		ok, _, err := template.Check(expr, s)
		return ok, err
	}
}

// newAsyncRecord builds the record of a run, whatever its outcome.
func (x *execution) newAsyncRecord(b builtAsync, res transport.Result, err error) *Record {
	rec := &Record{
		ID: x.r.deps.NewID(), RunID: x.r.runID, CollectionID: x.c.collection.Manifest.ID, RequestID: x.c.request.ID,
		RequestPath: x.c.request.Path, Version: x.version.Label, VersionSource: x.source, At: x.r.deps.Clock(),
		Request: b.sent, Kind: KindAsync, Proto: b.protocol, Outcome: outcomeOf(err), Overlay: x.c.overlayName(),
		Timings:  Timings{Connect: res.Connect, Total: res.Total},
		Attempts: []AttemptRecord{{Duration: res.Total}},
	}
	if err != nil {
		rec.Attempts[0].Error = err.Error()
	}
	if res.Sent != nil {
		rec.Messages = append(rec.Messages, recordMessage(*res.Sent))
	}
	for _, m := range res.Received {
		rec.Messages = append(rec.Messages, recordMessage(m))
	}
	rec.Body = headlineBody(rec.Messages)
	return rec
}

func outcomeOf(err error) string {
	switch {
	case err == nil:
		return OutcomeOK
	case errors.Is(err, transport.ErrTimeout):
		return OutcomeTimeout
	case errors.Is(err, errMessageHook):
		return OutcomeStopped
	}
	return OutcomeError
}

// headlineBody is the first message received, else the one sent: what the
// record's Body shows.
func headlineBody(msgs []RecordMessage) Body {
	for _, m := range msgs {
		if m.Direction == directionRecvd {
			return m.Body
		}
	}
	if len(msgs) > 0 {
		return msgs[0].Body
	}
	return Body{}
}

// recordMessage is a transport message as recorded: text or base64, parsed
// when it looks like a JSON object or array.
func recordMessage(m transport.Message) RecordMessage {
	out := RecordMessage{Direction: m.Direction, At: m.At, Key: m.Key, Partition: m.Partition, Offset: m.Offset, Sequence: m.Sequence}
	for _, h := range m.Headers {
		out.Headers = append(out.Headers, Header{Name: h.Name, Value: h.Value})
	}
	out.Body = messageBody(m.Body)
	return out
}

func messageBody(data []byte) Body {
	b := Body{Size: int64(len(data))}
	if !transport.IsText(data) {
		b.Base64 = base64.StdEncoding.EncodeToString(data)
		return b
	}
	b.Text = string(data)
	trimmed := strings.TrimSpace(b.Text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return b
	}
	b.ContentType = "application/json"
	parsed, err := decodeJSON(data)
	if err != nil {
		b.ParseErr = err.Error()
		return b
	}
	b.JSON = parsed
	return b
}

// scopeMessage is a recorded message as expressions see it.
func scopeMessage(m RecordMessage) map[string]any {
	headers := map[string]any{}
	for _, h := range m.Headers {
		headers[strings.ToLower(h.Name)] = h.Value
	}
	var body any = m.Body.Text
	if m.Body.JSON != nil {
		body = scopeValue(m.Body.JSON)
	}
	return map[string]any{"direction": m.Direction, "headers": headers, "body": body, "text": m.Body.Text,
		"key": m.Key, "partition": int(m.Partition), "offset": int(m.Offset), "sequence": int(m.Sequence)} //nolint:gosec // stream sequences fit an int on the 64-bit targets we build for
}

// observeAsync makes the result visible to extraction, assertions and
// post-hooks. status is not defined for an async request.
func (x *execution) observeAsync(res transport.Result) {
	first := firstRecordMessage(x.record.Messages)
	sc := scopeMessage(first)
	var received []any
	for _, m := range x.record.Messages {
		if m.Direction == directionRecvd {
			received = append(received, scopeMessage(m))
		}
	}
	all := make([]any, 0, len(x.record.Messages))
	for _, m := range x.record.Messages {
		all = append(all, scopeMessage(m))
	}
	x.mu.Lock()
	x.async = map[string]any{"messages": received, "all_messages": all, "received": len(received), "outcome": x.record.Outcome,
		"body": sc["body"], "headers": sc["headers"]}
	x.mu.Unlock()
	hdr := http.Header{}
	for _, h := range first.Headers {
		hdr.Add(h.Name, h.Value)
	}
	x.state.Response = &nodes.Response{Header: hdr, Body: sc["body"], Text: first.Body.Text, Time: res.Total}
	x.state.Request = nil
}

// firstRecordMessage is the first message received, else the first sent.
func firstRecordMessage(msgs []RecordMessage) RecordMessage {
	for _, m := range msgs {
		if m.Direction == directionRecvd {
			return m
		}
	}
	if len(msgs) > 0 {
		return msgs[0]
	}
	return RecordMessage{}
}

// asyncValues are the extra names expressions can use once an async request
// has run.
func (x *execution) asyncValues() map[string]any {
	x.mu.Lock()
	defer x.mu.Unlock()
	return x.async
}

// asyncView is the request as pre-hooks read it.
func (x *execution) asyncView() map[string]any {
	b, _ := x.renderAsync(true)
	headers := map[string]any{}
	for _, h := range b.op.Headers {
		headers[h.Name] = h.Value
	}
	body := ""
	if b.op.Send != nil {
		body = string(b.op.Send.Body)
	}
	return map[string]any{"method": model.ProtocolVerb(b.protocol), "url": model.TargetURL(b.protocol, b.op.Server, b.op.Channel), "headers": headers, "body": body}
}

// errMessageHook ends a receive whose message hook failed: the hook has
// reported its own diagnostics and failed the request.
var errMessageHook = errors.New("a message hook stopped the run")

// runMessageHooks runs the message hooks for the message just kept. They run
// with the rules of post hooks (the request has gone) and read this message as
// body, headers, message (seq, body, headers) and seq; a failing hook stops the
// receive.
func (x *execution) runMessageHooks(ctx context.Context, seq int, m transport.Message) error {
	x.showMessage(seq, m)
	if !x.runPhaseNamed(ctx, x.messageHooks, nodes.PhasePost, "message") {
		return errMessageHook
	}
	return nil
}

// showMessage makes one message the one expressions and hooks read.
func (x *execution) showMessage(seq int, m transport.Message) {
	msg := scopeMessage(recordMessage(m))
	x.mu.Lock()
	x.async = map[string]any{"body": msg["body"], "headers": msg["headers"], "seq": seq,
		"message": map[string]any{"seq": seq, "body": msg["body"], "headers": msg["headers"]}}
	x.mu.Unlock()
	hdr := http.Header{}
	for _, h := range m.Headers {
		hdr.Add(h.Name, h.Value)
	}
	x.state.Response = &nodes.Response{Header: hdr, Body: msg["body"], Text: string(m.Body), Time: 0}
	x.state.Request = nil
}
