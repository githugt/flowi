// SPDX-License-Identifier: Apache-2.0

package model

import "strings"

// GRPC is what a gRPC request version does: it calls one method of a service
// with a JSON message and reads the response (one message, or a stream).
type GRPC struct {
	// Server is grpc://host:port (plaintext), grpcs://host:port (TLS) or a bare
	// host:port; templated, e.g. {{env.api}}.
	Server string
	// Method is package.Service/Method.
	Method string
	// Body is the request message, as JSON (or text holding JSON).
	Body Body
	// Proto lists .proto files, relative to the collection. Empty: the schema
	// is asked of the server by reflection.
	Proto []string
	// Import lists the roots .proto imports are resolved against, relative to
	// the collection.
	Import []string
	// Receive says which messages of a server stream to keep; unused for a
	// unary call. Count 0 means 1 and Timeout 0 means 30s when it runs.
	Receive *AsyncReceive
	// Web makes the call over gRPC-Web: an HTTP POST to Server, which is then
	// an http:// or https:// address, for a proxy or gateway that speaks it.
	// It needs Proto files (such a proxy has no reflection) and a method that
	// does not stream from the client.
	Web bool
}

// The URL schemes of a gRPC server.
const (
	GRPCSchemePlaintext = "grpc"
	GRPCSchemeTLS       = "grpcs"
)

// GRPCTarget is the host and port to dial and whether to use TLS. A scheme
// decides; a bare host:port is plaintext for the local machine (localhost,
// 127.0.0.1, ::1) and TLS for any other host.
func GRPCTarget(server string) (hostport string, useTLS bool) {
	scheme, rest, found := strings.Cut(server, "://")
	if !found {
		rest = server
	}
	hostport = strings.TrimRight(rest, "/")
	switch strings.ToLower(scheme) {
	case GRPCSchemePlaintext:
		return hostport, false
	case GRPCSchemeTLS:
		return hostport, true
	}
	return hostport, !isLocalHost(hostport)
}

// isLocalHost reports whether hostport names the local machine.
func isLocalHost(hostport string) bool {
	host := hostport
	if strings.HasPrefix(host, "[") { // [::1]:50051
		if end := strings.Index(host, "]"); end > 0 {
			return host[1:end] == "::1"
		}
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	return host == "localhost" || host == "127.0.0.1"
}

// SplitMethod splits package.Service/Method into the service and method
// names. A leading slash (grpcurl's way of writing it) is allowed.
func SplitMethod(m string) (service, method string, ok bool) {
	service, method, found := strings.Cut(strings.TrimPrefix(m, "/"), "/")
	if !found || service == "" || method == "" || strings.Contains(method, "/") {
		return "", "", false
	}
	return service, method, true
}

// GRPCTargetURL is where a gRPC call goes, for display: the server with the
// scheme the call will use, then the method. A templated server is shown as
// written, since its scheme is only known when it runs.
func GRPCTargetURL(server, method string) string {
	shown := server
	if !strings.HasPrefix(strings.TrimSpace(server), "{{") && !strings.Contains(server, "://") && server != "" {
		scheme := GRPCSchemePlaintext
		if _, useTLS := GRPCTarget(server); useTLS {
			scheme = GRPCSchemeTLS
		}
		shown = scheme + "://" + server
	}
	if method == "" {
		return shown
	}
	return strings.TrimRight(shown, "/") + "/" + strings.TrimLeft(method, "/")
}
