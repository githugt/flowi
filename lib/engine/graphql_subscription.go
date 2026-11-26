// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"slices"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/graphqlx"
	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// graphQLExchange runs a request with a GraphQL body: a query or a mutation is an HTTP request, a subscription is a
// stream of events over a WebSocket.
type graphQLExchange struct{ x *execution }

func (e graphQLExchange) send(ctx context.Context, post []HookRef) {
	if isSubscription(e.x.version) {
		e.x.sendSubscription(ctx, post)
		return
	}
	e.x.sendHTTP(ctx, post)
}

func (e graphQLExchange) view() map[string]any {
	if isSubscription(e.x.version) {
		b, _ := e.x.renderSubscription(true)
		return subscriptionView(b)
	}
	return httpExchange(e).view()
}

func (graphQLExchange) preview(in buildInput, includeSecrets bool) Preview {
	if isSubscription(in.version) {
		b, diags := buildSubscription(in)
		return previewBuiltAsync(b, diags, includeSecrets)
	}
	return previewHTTP(in, includeSecrets)
}

func (e graphQLExchange) probe(ctx context.Context) Probe {
	if isSubscription(e.x.version) {
		b, diags := e.x.renderSubscription(false)
		return e.x.probeAsync(ctx, b, diags)
	}
	return httpExchange(e).probe(ctx)
}

// isSubscription says whether the operation the version runs is a subscription. A document that cannot be read is
// not: it goes to the server as a query, which answers with what is wrong.
func isSubscription(v model.RequestVersion) bool {
	g := v.Body.GraphQL
	if g == nil {
		return false
	}
	op, err := graphqlx.Select(g.Query, g.Operation)
	return err == nil && op.Kind == graphqlx.Subscription
}

// sendSubscription is everything after the pre-hooks for a subscription: render, subscribe, record the events,
// then extract, assert, post-hooks and save, as for a message request.
func (x *execution) sendSubscription(ctx context.Context, post []HookRef) {
	b, diags := x.renderSubscription(false)
	diags = append(diags, x.r.secrets.problems(x.env, b.missing)...)
	if diags.HasErrors() {
		x.fail(StatusError, diags)
		return
	}
	x.finishAsync(ctx, post, b)
}

// renderSubscription is buildSubscription with what only a run knows: the TLS settings, the clock and the filter.
func (x *execution) renderSubscription(lenient bool) (builtAsync, diag.List) {
	b, diags := buildSubscription(x.asyncInput(lenient))
	tlsCfg, err := httpx.TLSConfig(x.cfg.caFiles)
	if err != nil {
		diags = append(diags, diag.New(diag.CodeHTTPFailed, diag.Location{}, map[string]string{"detail": err.Error()}))
	}
	b.op.TLS = tlsCfg
	b.op.Clock = x.r.deps.Clock
	if b.op.Receive != nil && !lenient {
		b.op.Receive.Match = x.matcher(b.match)
	}
	return b, diags
}

// buildSubscription renders a subscription: the URL, the headers that sign in to the WebSocket, and the operation
// as the message to subscribe with.
func buildSubscription(in buildInput) (builtAsync, diag.List) {
	b := &builder{in: in, tainted: map[string]bool{}}
	g := *in.version.Body.GraphQL
	server, shownURL := b.url()
	header, shown := b.headers()
	out := builtAsync{protocol: model.ProtocolGraphQLWS, receive: normalizedReceive(g.Receive)}
	if g.Receive != nil {
		out.match = g.Receive.Match
	}
	payload, ok := b.graphQLRequest()
	if !ok {
		return out, b.diags
	}
	text, err := marshalGraphQL(payload)
	if err != nil {
		b.add(diag.CodeGraphQLRequest, map[string]string{"detail": "the operation of " + in.label + " cannot be sent: " + err.Error()})
		return out, b.diags
	}
	_, shownBody := b.inMemory(text, "application/json")
	op := transport.Operation{Server: server, Send: &transport.Message{Body: []byte(text)}, Binding: map[string]string{}}
	names := make([]string, 0, len(header))
	for name := range header {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		for _, v := range header[name] {
			op.Headers = append(op.Headers, transport.Header{Name: name, Value: v})
		}
	}
	op.Receive = &transport.Receive{Count: out.receive.Count, Timeout: out.receive.Timeout, MaxBytes: asyncMaxBytes}
	out.op = op
	out.sent = SentRequest{Method: subscriptionVerb, URL: shownURL, Header: shown, Body: shownBody}
	out.missing = b.missingSecrets
	return out, b.diags
}

// subscriptionVerb is what a subscription shows where an HTTP request shows its method.
const subscriptionVerb = "GQL"

func subscriptionView(b builtAsync) map[string]any {
	headers := map[string]any{}
	for _, h := range b.op.Headers {
		headers[h.Name] = h.Value
	}
	body := ""
	if b.op.Send != nil {
		body = string(b.op.Send.Body)
	}
	return map[string]any{"method": subscriptionVerb, "url": b.op.Server, "headers": headers, "body": body}
}

// checkSubscriptionErrors fails a subscription whose events carry an errors list, as a query's answer does: a
// server that sends {"errors": [...]} in a next message has not delivered what was subscribed to.
func (x *execution) checkSubscriptionErrors() {
	if !isSubscription(x.version) || x.record == nil {
		return
	}
	var events []any
	for _, m := range x.record.Messages {
		if m.Direction == directionRecvd && m.Body.JSON != nil {
			events = append(events, m.Body.JSON)
		}
	}
	x.reportGraphQLErrors(events, "")
}
