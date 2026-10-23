// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/githugt/flowi-lib/transport"
)

// DefaultTimeout is the deadline of a call that sets none.
const DefaultTimeout = 30 * time.Second

// Call is one gRPC call: where, what and with which schema.
type Call struct {
	// Server is grpc://host:port, grpcs://host:port or host:port (see
	// model.GRPCTarget for which of those is plaintext).
	Server string
	// Method is package.Service/Method.
	Method string
	// JSON is the request message; empty means {}.
	JSON []byte
	// Metadata is sent with the call; names are lower-cased, and -bin names are
	// refused.
	Metadata []transport.Header
	TLS      *tls.Config
	// Root is the collection directory the Protos and Imports are relative to.
	Root string
	// Protos are .proto files relative to Root. Empty: the schema is asked of the
	// server by reflection.
	Protos []string
	// Imports are the roots the files' imports are resolved against, relative to
	// Root. Root itself is always one.
	Imports []string
	// Receive says which messages of a server stream to keep and how long to
	// wait for them; unused for a unary call.
	Receive *transport.Receive
	// Timeout is the deadline of the whole call; zero means DefaultTimeout.
	Timeout time.Duration
	Clock   func() time.Time // nil: time.Now
}

// Result is what a call did. Messages holds the request as sent, then the
// responses kept. It is also returned with transport.ErrTimeout, holding what
// arrived before the wait ended.
type Result struct {
	Messages []transport.Message
	Header   []transport.Header // response metadata
	Trailer  []transport.Header
	Code     codes.Code
	Message  string // the status message
	// Warnings are things the call went ahead despite: a field of the JSON
	// that the request message does not have, which was left out.
	Warnings []string
	Connect  time.Duration
	Total    time.Duration
}

// Method is one method of a service, for pickers and validation.
type Method struct {
	Service      string // acme.orders.v1.Orders
	Name         string // Get
	ClientStream bool
	ServerStream bool
	Input        string // full name of the request message
	Output       string
	// Example is a request message as compact JSON with every field at its
	// zero value, for a starting point.
	Example string
}

// Full is package.Service/Method.
func (m Method) Full() string { return m.Service + "/" + m.Name }

// SchemaError means the schema could not be loaded or does not hold the
// method: a missing import, a server without reflection, an unknown method.
type SchemaError struct {
	Detail string
	// Proto is true when the .proto files are at fault (they did not compile or
	// register), not the method or the server.
	Proto bool
}

func (e *SchemaError) Error() string { return e.Detail }

// BadBodyError means the JSON does not fit the request message; nothing was
// sent.
type BadBodyError struct{ Detail string }

func (e *BadBodyError) Error() string { return "the message does not fit: " + e.Detail }

func (c Call) now() time.Time {
	if c.Clock != nil {
		return c.Clock()
	}
	return time.Now()
}

func (c Call) timeout() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return DefaultTimeout
}
