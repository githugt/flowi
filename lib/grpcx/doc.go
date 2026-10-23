// SPDX-License-Identifier: Apache-2.0

// Package grpcx makes gRPC calls without generated code: the schema comes
// from .proto files (compiled in pure Go) or from the server's reflection
// service, messages are JSON converted with protojson and dynamicpb, and a
// unary or server-streaming method is called over one short-lived connection.
//
// A non-OK gRPC status is a normal Result (its code and message), not an
// error; errors are for what stops a call from being made: a schema that
// cannot be loaded, a body that does not fit the message, a method that is not
// supported yet.
package grpcx
