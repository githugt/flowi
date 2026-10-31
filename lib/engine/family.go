// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"

	"github.com/githugt/flowi-lib/model"
)

// exchange is how the engine runs the requests of one family (model.Family). Everything the engine does that depends
// on what kind of request it has goes through it, so a new family is one more implementation here and not another
// branch in every place that sends, previews or shows a request.
type exchange interface {
	// send is everything after the pre-hooks: render, send, record, extract, assert, post-hooks and save.
	send(ctx context.Context, post []HookRef)
	// view is the request as pre-hooks read it: a lenient render, so fields that use values not set yet are empty.
	view() map[string]any
	// preview renders the request without sending it.
	preview(in buildInput, includeSecrets bool) Preview
}

// exchange is the exchange of the family of the version being run.
func (x *execution) exchange() exchange {
	switch x.version.Family() {
	case model.FamilyRPC:
		return rpcExchange{x}
	case model.FamilyMessaging:
		return messagingExchange{x}
	case model.FamilyDataset:
		return datasetExchange{x}
	}
	return httpExchange{x}
}

type httpExchange struct{ x *execution }

func (e httpExchange) send(ctx context.Context, post []HookRef) { e.x.sendHTTP(ctx, post) }

func (e httpExchange) view() map[string]any {
	b, _ := e.x.render(true)
	return viewOf(b)
}

func (httpExchange) preview(in buildInput, includeSecrets bool) Preview {
	return previewHTTP(in, includeSecrets)
}

type rpcExchange struct{ x *execution }

func (e rpcExchange) send(ctx context.Context, post []HookRef) { e.x.sendGRPC(ctx, post) }
func (e rpcExchange) view() map[string]any                     { return e.x.grpcView() }
func (rpcExchange) preview(in buildInput, includeSecrets bool) Preview {
	return previewGRPC(in, includeSecrets)
}

type messagingExchange struct{ x *execution }

func (e messagingExchange) send(ctx context.Context, post []HookRef) { e.x.sendAsync(ctx, post) }
func (e messagingExchange) view() map[string]any                     { return e.x.asyncView() }
func (messagingExchange) preview(in buildInput, includeSecrets bool) Preview {
	return previewAsync(in, includeSecrets)
}
