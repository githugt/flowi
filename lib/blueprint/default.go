// SPDX-License-Identifier: Apache-2.0

package blueprint

import "github.com/githugt/flowi-lib/model"

// Default is the blueprint new requests are seeded with. Its nodes become the
// request's inline post hooks, which see the response: they write its status
// to the run log and fail the request unless the status is 2xx. Redirects
// need no node: the engine follows them up to http.max_redirects and fails
// the request beyond it.
func Default() model.Blueprint {
	return model.Blueprint{
		Meta: model.Meta{Name: "Default request hooks", Description: "Logs the response status and fails on a non-2xx status."},
		Body: model.Flow{Nodes: []model.Node{
			{ID: "response", Type: "log", Params: []model.Field{
				{Key: "value", Value: "Response {{response.status}}"},
			}},
			{ID: "is-success", Type: "assert", Params: []model.Field{
				{Key: "check", Value: "response.status >= 200 && response.status < 300"},
				{Key: "message", Value: "Expected a 2xx response, got {{response.status}}"},
			}},
		}},
	}
}

// DefaultAsyncPre is the pre hook new async requests are seeded with: it
// writes what is about to be sent to the run log.
func DefaultAsyncPre() model.Blueprint {
	return model.Blueprint{
		Meta:  model.Meta{Name: "Default async pre hook", Description: "Logs the message request before it runs."},
		Phase: "pre",
		Body: model.Flow{Nodes: []model.Node{
			{ID: "request", Type: "log", Params: []model.Field{
				{Key: "value", Value: "Request {{request.method}} {{request.url}}"},
			}},
		}},
	}
}

// DefaultGRPCPre is the pre hook new gRPC requests are seeded with: it writes
// the call about to be made to the run log.
func DefaultGRPCPre() model.Blueprint {
	bp := DefaultAsyncPre()
	bp.Meta = model.Meta{Name: "Default gRPC pre hook", Description: "Logs the call before it is made."}
	return bp
}

// DefaultGRPC is the post hook new gRPC requests are seeded with. A gRPC call
// has a status code, not an HTTP status, so it logs the code and fails unless
// it is OK.
func DefaultGRPC() model.Blueprint {
	return model.Blueprint{
		Meta: model.Meta{Name: "Default gRPC post hook", Description: "Logs the status code and fails unless it is OK."},
		Body: model.Flow{Nodes: []model.Node{
			{ID: "response", Type: "log", Params: []model.Field{
				{Key: "value", Value: "Response {{code}}"},
			}},
			{ID: "is-success", Type: "assert", Params: []model.Field{
				{Key: "check", Value: `code == "OK"`},
				{Key: "message", Value: "Expected the call to end OK, got {{code}}"},
			}},
		}},
	}
}

// DefaultAsync is the post hook new async requests are seeded with. An async
// request has no status, so it logs the outcome and the number of messages
// received, and fails unless the run ended well.
func DefaultAsync() model.Blueprint {
	return model.Blueprint{
		Meta: model.Meta{Name: "Default async post hook", Description: "Logs the outcome and fails unless it is ok."},
		Body: model.Flow{Nodes: []model.Node{
			{ID: "response", Type: "log", Params: []model.Field{
				{Key: "value", Value: "Response {{outcome}}, {{received}} received"},
			}},
			{ID: "is-success", Type: "assert", Params: []model.Field{
				{Key: "check", Value: `outcome == "ok"`},
				{Key: "message", Value: "Expected the run to end ok, got {{outcome}}"},
			}},
		}},
	}
}
