// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"sort"
	"strings"

	"google.golang.org/grpc/codes"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/internal/nodes"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/transport"
)

// GRPCRunner makes one gRPC call. The engine uses grpcx.Run; tests pass fakes.
type GRPCRunner interface {
	Run(ctx context.Context, c grpcx.Call) (grpcx.Result, error)
}

type grpcClient struct{}

func (grpcClient) Run(ctx context.Context, c grpcx.Call) (grpcx.Result, error) {
	return grpcx.Run(ctx, c)
}

func (r *runner) grpcRunner() GRPCRunner {
	if r.deps.GRPC != nil {
		return r.deps.GRPC
	}
	return grpcClient{}
}

// builtGRPC is a gRPC version rendered and ready to run.
type builtGRPC struct {
	call    grpcx.Call
	sent    SentRequest // as recorded: secret-bearing fields already masked
	missing []string    // declared secrets with no value (FLW-SEC-001)
	method  string
	// auth is the auth block with its parameters rendered; only mutual TLS
	// applies to a gRPC call.
	auth *model.Auth
}

// sendGRPC is everything after the pre-hooks for a gRPC request: render, make
// the call, record it, then extract, assert, post-hooks and save, in the order
// an HTTP request has. A non-OK gRPC status is a result an assertion can read
// (code == "NOT_FOUND"), not a failure of the run.
func (x *execution) sendGRPC(ctx context.Context, post []HookRef) {
	b, ok := x.buildGRPC()
	if !ok {
		return
	}
	x.emitGRPCSent(b)
	if b.call.Receive != nil {
		seq := 0
		b.call.Receive.OnMessage = func(m transport.Message) {
			seq++
			x.r.emit(MessageReceived{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), Seq: seq, Depth: x.c.depth,
				Message: recordMessage(m).masked(x.r.secrets.current())})
		}
		if len(x.messageHooks) > 0 {
			b.call.Receive.OnKept = func(m transport.Message) error { return x.runMessageHooks(ctx, seq, m) }
		}
	}
	res, err := x.r.grpcRunner().Run(ctx, b.call)
	if ctx.Err() != nil {
		if receivedCount(res) > 0 { // a watch stopped after some messages: show them
			x.record = x.newGRPCRecord(b, res, err)
			x.record.Outcome = OutcomeCancelled
		}
		x.fail(StatusCancelled, nil)
		return
	}
	x.warnGRPC(res.Warnings)
	x.record = x.newGRPCRecord(b, res, err)
	x.emitGRPCReceived(res, err)
	x.observeGRPC(res)
	if err != nil {
		if !errors.Is(err, errMessageHook) {
			x.failGRPC(err, b, res) // a failed message hook has reported itself
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

func (x *execution) emitGRPCSent(b builtGRPC) {
	x.r.emit(RequestSent{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), RequestPath: x.c.request.Path,
		Version: x.version.Label, VersionSource: x.source, Method: b.sent.Method, URL: x.r.secrets.String(b.sent.URL), Attempt: 1, Depth: x.c.depth})
}

func (x *execution) emitGRPCReceived(res grpcx.Result, err error) {
	ev := ResponseReceived{RunID: x.r.runID, Path: x.c.path, RequestID: string(x.c.request.ID), Duration: res.Total, Attempt: 1,
		Depth: x.c.depth, Outcome: grpcOutcome(res, err)}
	if err != nil {
		ev.Error = x.r.secrets.String(err.Error())
	}
	x.r.emit(ev)
}

// grpcOutcome is how a call ended: ok for the OK status, timeout for a missed
// deadline or a stream wait that ran out, error for any other status or
// failure.
func grpcOutcome(res grpcx.Result, err error) string {
	switch {
	case errors.Is(err, transport.ErrTimeout), err == nil && res.Code == codes.DeadlineExceeded:
		return OutcomeTimeout
	case errors.Is(err, errMessageHook):
		return OutcomeStopped
	case err != nil:
		return OutcomeError
	case res.Code == codes.OK:
		return OutcomeOK
	}
	return OutcomeError
}

// failGRPC turns an error into the request's diagnostics. Nothing here is a
// non-OK status: those are results.
func (x *execution) failGRPC(err error, b builtGRPC, res grpcx.Result) {
	var schema *grpcx.SchemaError
	var body *grpcx.BadBodyError
	switch {
	case errors.As(err, &schema) && schema.Proto:
		x.fail(StatusError, diag.List{diag.New(diag.CodeGRPCProto, diag.Location{}, map[string]string{"detail": err.Error()})})
	case errors.As(err, &schema):
		x.fail(StatusError, diag.List{diag.New(diag.CodeGRPCMethod, diag.Location{}, map[string]string{"method": b.method, "detail": err.Error()})})
	case errors.As(err, &body):
		x.fail(StatusError, diag.List{diag.New(diag.CodeGRPCBody, diag.Location{}, map[string]string{"method": b.method, "detail": body.Detail})})
	case errors.Is(err, transport.ErrTimeout):
		rc := normalizedReceive(x.version.GRPC.Receive)
		x.fail(StatusError, diag.List{diag.New(diag.CodeAsyncTimeout, diag.Location{}, map[string]string{
			"got": itoa(receivedCount(res)), "want": itoa(rc.Count), "timeout": rc.Timeout.String()})})
	default:
		x.fail(StatusError, diag.List{diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": x.r.secrets.String(err.Error())})})
	}
}

// warnGRPC adds what the call went ahead despite to the send's diagnostics as
// warnings: they never change its status.
func (x *execution) warnGRPC(warnings []string) {
	x.mu.Lock()
	defer x.mu.Unlock()
	for _, w := range warnings {
		x.diags = append(x.diags, diag.New(diag.CodeGRPCField, diag.Location{}, map[string]string{"detail": w}))
	}
}

func receivedCount(res grpcx.Result) int {
	n := 0
	for _, m := range res.Messages {
		if m.Direction == directionRecvd {
			n++
		}
	}
	return n
}

// buildGRPC renders the version strictly, after the pre-hooks.
func (x *execution) buildGRPC() (builtGRPC, bool) {
	b, diags := x.renderGRPC(false)
	diags = append(diags, x.r.secrets.problems(x.env.name, b.missing)...)
	if diags.HasErrors() {
		x.fail(StatusError, diags)
		return builtGRPC{}, false
	}
	return b, true
}

func (x *execution) renderGRPC(lenient bool) (builtGRPC, diag.List) {
	in := buildInput{
		version: x.version, scope: x.scope(lenient), empty: x.cfg.empty, label: x.c.request.Path,
		root: x.c.collection.Root, secretVars: x.env.secrets, missingSecret: x.r.secrets.isMissing(x.env.name), memo: x.memo,
		secret: x.r.secrets.derived, verbatim: lenient,
	}
	b, diags := buildGRPC(in)
	tlsCfg, err := httpx.TLSConfig(x.cfg.caFiles)
	if err != nil {
		diags = append(diags, diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()}))
	}
	b.call.TLS = withClientCert(tlsCfg, b.auth, x.c.collection.Root, &diags)
	b.call.Clock = x.r.deps.Clock
	b.call.Timeout = x.cfg.timeout
	if g := x.version.GRPC; g.Receive != nil {
		b.call.Timeout += b.call.Receive.Normalized().Timeout
		if !lenient {
			b.call.Receive.Match = x.matcher(g.Receive.Match)
		}
	}
	return b, diags
}

// buildGRPC renders every templated field of a gRPC version: the server, the
// method, the message and the metadata.
func buildGRPC(in buildInput) (builtGRPC, diag.List) {
	g := *in.version.GRPC
	v := in.version
	v.Body = g.Body // the HTTP body renderers read the version's body
	in.version = v
	b := &builder{in: in, tainted: map[string]bool{}}
	server, _ := b.text("server", "server", g.Server)
	method, _ := b.text("method", "method", g.Method)
	meta, shownMeta := b.headers()
	src, shownBody := b.body()
	call := grpcx.Call{Server: server, Method: method, Root: in.root, Protos: g.Proto, Imports: g.Import, Metadata: metadataOf(meta), Web: g.Web}
	if src != nil {
		data, err := readBody(src)
		if err != nil {
			b.fileError("message body", err)
		}
		call.JSON = data
	}
	// Always set, so a stream's messages show as they arrive. Without receive
	// options there is no count and no timeout of its own: the stream is read until
	// it ends or the call's timeout passes (see grpcx).
	call.Receive = &transport.Receive{MaxBytes: asyncMaxBytes}
	if g.Receive != nil {
		rc := normalizedReceive(g.Receive)
		call.Receive = &transport.Receive{Count: rc.Count, Timeout: rc.Timeout, MaxBytes: asyncMaxBytes}
	}
	out := builtGRPC{call: call, method: method, missing: b.missingSecrets, auth: b.auth()}
	out.sent = SentRequest{Method: model.ProtocolVerbGRPC, URL: shownGRPCTarget(b, server, method), Header: shownMeta, Body: shownBody}
	return out, b.diags
}

// metadataOf lists the rendered metadata in name order.
func metadataOf(h http.Header) []transport.Header {
	names := make([]string, 0, len(h))
	for name := range h {
		names = append(names, name)
	}
	sort.Strings(names)
	var out []transport.Header
	for _, name := range names {
		for _, v := range h[name] {
			out = append(out, transport.Header{Name: name, Value: v})
		}
	}
	return out
}

// shownGRPCTarget is the server and method as recorded; a secret in the
// server hides all of it, and a password written in it is hidden.
func shownGRPCTarget(b *builder, server, method string) string {
	if b.tainted["server"] {
		return "(masked)"
	}
	return model.RedactedTarget(model.GRPCTargetURL(server, method))
}

// newGRPCRecord builds the record of a call, whatever its outcome.
func (x *execution) newGRPCRecord(b builtGRPC, res grpcx.Result, err error) *Record {
	rec := &Record{
		ID: x.r.deps.NewID(), RunID: x.r.runID, CollectionID: x.c.collection.Manifest.ID, RequestID: x.c.request.ID,
		RequestPath: x.c.request.Path, Version: x.version.Label, VersionSource: x.source, At: x.r.deps.Clock(),
		Request: b.sent, Kind: KindGRPC, Proto: "grpc", Outcome: grpcOutcome(res, err), Overlay: x.c.overlayName(),
		GRPCCode: grpcCodeName(res.Code), GRPCMessage: res.Message,
		Timings:  Timings{Connect: res.Connect, Total: res.Total},
		Attempts: []AttemptRecord{{Duration: res.Total}},
		Header:   headersOfTransport(res.Header), Trailers: headersOfTransport(res.Trailer),
	}
	if err != nil {
		rec.Attempts[0].Error = err.Error()
	}
	for _, m := range res.Messages {
		rec.Messages = append(rec.Messages, recordMessage(m))
	}
	rec.Body = headlineBody(rec.Messages)
	if res.Code != codes.OK && res.Message != "" && receivedCount(res) == 0 {
		rec.Body = Body{Text: res.Message, Size: int64(len(res.Message))} // the status message is all the server said
	}
	return rec
}

func headersOfTransport(hs []transport.Header) []Header {
	var out []Header
	for _, h := range hs {
		out = append(out, Header{Name: h.Name, Value: h.Value})
	}
	return out
}

// grpcCodeName is a status code as written in specs and by grpcurl:
// NOT_FOUND, DEADLINE_EXCEEDED, OK.
func grpcCodeName(c codes.Code) string {
	name := c.String()
	if name == "OK" {
		return name
	}
	var b strings.Builder
	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(r)
	}
	return strings.ToUpper(b.String())
}

// observeGRPC makes the result visible to extraction, assertions and
// post-hooks: body, messages, all_messages, received, headers, trailers, code
// and outcome. status is not defined for a gRPC request.
func (x *execution) observeGRPC(res grpcx.Result) {
	first := firstRecordMessage(x.record.Messages)
	sc := scopeMessage(first)
	var received []any
	all := make([]any, 0, len(x.record.Messages))
	for _, m := range x.record.Messages {
		all = append(all, scopeMessage(m))
		if m.Direction == directionRecvd {
			received = append(received, scopeMessage(m))
		}
	}
	x.mu.Lock()
	x.async = map[string]any{"messages": received, "all_messages": all, "received": len(received), "outcome": x.record.Outcome,
		"body": sc["body"], "headers": lowerHeaders(x.record.Header), "trailers": lowerHeaders(x.record.Trailers), "code": x.record.GRPCCode}
	x.mu.Unlock()
	hdr := http.Header{}
	for _, h := range x.record.Header {
		hdr.Add(h.Name, h.Value)
	}
	x.state.Response = &nodes.Response{Header: hdr, Body: sc["body"], Text: first.Body.Text, Time: res.Total}
	x.state.Request = nil
}

// lowerHeaders is metadata as expressions see it: names in lower case.
func lowerHeaders(hs []Header) map[string]any {
	out := map[string]any{}
	for _, h := range hs {
		out[strings.ToLower(h.Name)] = h.Value
	}
	return out
}

// grpcView is the request as pre-hooks read it.
func (x *execution) grpcView() map[string]any {
	b, _ := x.renderGRPC(true)
	headers := map[string]any{}
	for _, h := range b.call.Metadata {
		headers[h.Name] = h.Value
	}
	return map[string]any{"method": model.ProtocolVerbGRPC, "url": model.GRPCTargetURL(b.call.Server, b.call.Method), "headers": headers, "body": string(b.call.JSON)}
}

func previewGRPC(in buildInput, includeSecrets bool) Preview {
	b, diags := buildGRPC(in)
	masked := PreviewRequest{Method: b.sent.Method, URL: b.sent.URL, Headers: b.sent.Header}
	if b.sent.Body != nil {
		masked.BodyKind, masked.Body = "text", b.sent.Body.Text
	}
	out := Preview{Masked: masked, Diags: diags}
	if includeSecrets {
		exact := PreviewRequest{Method: b.sent.Method, URL: model.GRPCTargetURL(b.call.Server, b.call.Method), Headers: b.sent.Header}
		if b.call.JSON != nil {
			exact.BodyKind, exact.Body = "text", string(b.call.JSON)
		}
		out.Real = &exact
	}
	return out
}

// GRPCCallOf renders the gRPC version of the request in in as a call without
// making it: no hook runs and nothing is sent. It is what a method picker
// needs (the server, metadata, TLS and .proto files). includeSecrets reads the
// vault (which may prompt); without it a secret stays as written.
func GRPCCallOf(d Deps, in SendInput, includeSecrets bool) (grpcx.Call, diag.List) {
	r := newRunState(d, in.Seed, nil)
	r.in, r.kind, r.env, r.prefer = in, "send", in.Env, in.PreferVersion
	if !includeSecrets {
		r.secrets.lockErr = errPreviewLocked
	}
	x, diags := r.prepare(sendCall(in))
	if diags.HasErrors() {
		return grpcx.Call{}, diags
	}
	if x.version.GRPC == nil {
		return grpcx.Call{}, diag.List{diag.New(diag.CodeGRPCRequest, diag.Location{}, map[string]string{"detail": "this version is not a gRPC call"})}
	}
	b, diags := x.renderGRPC(true)
	return b.call, diags
}

// withClientCert adds the client certificate a mutual TLS auth block names to
// the TLS settings of a gRPC call; a plaintext call ignores TLS settings, so
// the certificate is only used when the server is reached over TLS.
func withClientCert(cfg *tls.Config, auth *model.Auth, root string, diags *diag.List) *tls.Config {
	if auth == nil || auth.Type != model.AuthMutualTLS || cfg == nil {
		return cfg
	}
	cert, err := loadClientCert(*auth, root)
	if err != nil {
		*diags = append(*diags, diag.New(diag.CodeAuthInvalid, diag.Location{}, map[string]string{"detail": err.Error()}))
		return cfg
	}
	out := cfg.Clone()
	out.Certificates = append(out.Certificates, cert)
	return out
}
