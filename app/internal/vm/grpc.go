// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"context"
	"slices"
	"strings"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
)

// SectionProto is the section that lists a gRPC request's .proto files and
// import roots.
const SectionProto Section = "proto"

// IsGRPC reports whether the selected version is a gRPC call.
func (r *Request) IsGRPC() bool { return r.Current().GRPC != nil }

// GRPC is a copy of the selected version's gRPC block; the zero value for any
// other version.
func (r *Request) GRPC() model.GRPC {
	if g := r.Current().GRPC; g != nil {
		return *cloneGRPC(g)
	}
	return model.GRPC{}
}

// EditGRPC changes the gRPC block of the selected version; it does nothing for
// another kind.
func (r *Request) EditGRPC(f func(g *model.GRPC)) {
	if !r.IsGRPC() {
		return
	}
	r.Edit(func(v *model.RequestVersion) { f(v.GRPC) })
}

// cloneGRPC deep-copies a gRPC block, so an edit never reaches the copy it
// came from.
func cloneGRPC(g *model.GRPC) *model.GRPC {
	if g == nil {
		return nil
	}
	out := *g
	out.Proto, out.Import = slices.Clone(g.Proto), slices.Clone(g.Import)
	out.Body.Form, out.Body.Multipart = slices.Clone(g.Body.Form), slices.Clone(g.Body.Multipart)
	if g.Receive != nil {
		rc := *g.Receive
		out.Receive = &rc
	}
	return &out
}

// grpcConfigured is whether section s of a gRPC version has content.
func grpcConfigured(g model.GRPC, s Section) bool {
	switch s {
	case SectionMessage:
		return bodyHasContent(g.Body)
	case SectionReceive:
		return g.Receive != nil && (g.Receive.Count > 0 || g.Receive.Timeout > 0 || g.Receive.Match != "")
	case SectionProto:
		return len(g.Proto) > 0 || len(g.Import) > 0
	}
	return false
}

// Methods are the methods of the request's schema as last loaded (LoadMethods).
func (r *Request) Methods() []grpcx.Method { return r.methods }

// MethodsError says why the methods could not be loaded; "" when they were, or
// have not been tried.
func (r *Request) MethodsError() string { return r.methodsErr }

// MethodStreams reports whether the selected gRPC version's method streams
// responses; known is false until the methods are loaded, or when the method
// is not among them.
func (r *Request) MethodStreams() (streams, known bool) {
	want := r.GRPC().Method
	for _, m := range r.methods {
		if m.Full() == strings.TrimPrefix(want, "/") {
			return m.ServerStream, true
		}
	}
	return false, false
}

// The places a gRPC request's schema comes from.
const (
	SourceFiles      = "files"
	SourceReflection = "reflection"
)

// MethodsSource says where the selected gRPC version's schema comes from: its
// .proto files, or the server's reflection.
func (r *Request) MethodsSource() string {
	if len(r.GRPC().Proto) > 0 {
		return SourceFiles
	}
	return SourceReflection
}

// PickMethod makes m the selected gRPC version's method, and, when its body is
// empty, fills it with the method's example message.
func (r *Request) PickMethod(m grpcx.Method) {
	r.EditGRPC(func(g *model.GRPC) {
		g.Method = m.Full()
		if !bodyHasContent(g.Body) && m.Example != "" {
			g.Body = model.Body{JSON: exampleBody(m)}
		}
	})
}

// exampleBody is the starting message of a method. A method that streams from
// the client takes an array, one element for each message to send.
func exampleBody(m grpcx.Method) string {
	if m.ClientStream {
		return "[" + m.Example + "]"
	}
	return m.Example
}

// GenerateRequests creates a gRPC request for each method loaded for req, in
// req's collection, with req's server and schema files (see
// flowi.Workspace.GenerateGRPCRequests); done gets what was created and skipped.
func (s *Send) GenerateRequests(req *Request, done func(flowi.GeneratedGRPC), fail func(error)) {
	if !req.IsGRPC() || len(req.Methods()) == 0 {
		return
	}
	s.ws.GenerateGRPCRequests(req.Target().Collection, req.GRPC(), req.Methods(), done, fail)
}

// LoadMethods asks for the methods of the selected gRPC version's schema (its
// .proto files, or the server's reflection) in the background, then calls the
// request's change callbacks. env is the selected environment, which the
// server may be written in terms of.
func (s *Send) LoadMethods(req *Request) {
	if !req.IsGRPC() {
		return
	}
	version, target, env := req.Current(), req.Target(), s.ws.SelectedEnv()
	type outcome struct {
		methods []grpcx.Method
		err     error
	}
	Run(s.w, func() outcome {
		methods, err := s.b.GRPCMethods(context.Background(), target, version, flowi.SendOptions{Env: env, Version: version.Label})
		return outcome{methods, err}
	}, func(o outcome) {
		req.methods, req.methodsErr = o.methods, ""
		if o.err != nil {
			req.methods, req.methodsErr = nil, o.err.Error()
		}
		req.changed()
	})
}
