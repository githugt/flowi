// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge

import (
	"slices"

	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/vm"
)

// AsyncInfo is what the screen of a message request needs besides its block.
type AsyncInfo struct {
	Protocols            []Choice    `json:"protocols"`
	Modes                []string    `json:"modes"` // send, receive, send_receive; none for a one-way protocol
	Mode                 string      `json:"mode"`
	Bindings             []string    `json:"bindings"`    // the protocol's own options
	Credentials          []string    `json:"credentials"` // the entries that sign in; none for ws and sse
	Cloud                bool        `json:"cloud"`       // a cloud service: its sign-in is the provider's own lookup or the credentials written here
	HeadersAreConnection bool        `json:"headersAreConnection"`
	Block                model.Async `json:"block"`
}

// GRPCMethodView is a method of the request's schema: the picker lists the
// methods under their service with an icon for the kind of call.
type GRPCMethodView struct {
	Full         string `json:"full"`
	Service      string `json:"service"`
	Name         string `json:"name"`
	ClientStream bool   `json:"clientStream"`
	ServerStream bool   `json:"serverStream"`
	Input        string `json:"input"`  // the request message's full name
	Output       string `json:"output"` // the response message's full name
}

// GRPCInfo is what the screen of a gRPC request needs besides its block.
type GRPCInfo struct {
	Block        model.GRPC       `json:"block"`
	Methods      []GRPCMethodView `json:"methods"`
	MethodsError string           `json:"methodsError,omitempty"`
	Streams      bool             `json:"streams"` // the chosen method streams responses
	StreamsKnown bool             `json:"streamsKnown"`
	Source       string           `json:"source"` // files or reflection
}

func asyncInfoOf(r *vm.Request) *AsyncInfo {
	if !r.IsAsync() {
		return nil
	}
	a := r.Async()
	modes := []string{}
	for _, m := range vm.AsyncModesFor(a.Protocol) {
		modes = append(modes, string(m))
	}
	return &AsyncInfo{
		Protocols: protocolChoices(), Modes: modes, Mode: string(r.Mode()), Bindings: nonNil(model.AsyncBindings(a.Protocol)),
		Credentials: nonNil(model.AsyncCredentials(a.Protocol)), Cloud: model.CloudProtocol(a.Protocol), HeadersAreConnection: vm.AsyncHeadersAreConnection(a.Protocol), Block: a,
	}
}

func grpcInfoOf(r *vm.Request) *GRPCInfo {
	if !r.IsGRPC() {
		return nil
	}
	streams, known := r.MethodStreams()
	info := &GRPCInfo{Block: r.GRPC(), Methods: []GRPCMethodView{}, MethodsError: r.MethodsError(), Streams: streams, StreamsKnown: known, Source: r.MethodsSource()}
	for _, m := range r.Methods() {
		info.Methods = append(info.Methods, GRPCMethodView{Full: m.Full(), Service: m.Service, Name: m.Name,
			ClientStream: m.ClientStream, ServerStream: m.ServerStream, Input: m.Input, Output: m.Output})
	}
	return info
}

// SetAsync replaces the async block of the selected message version.
func (s *RequestService) SetAsync(node string, a model.Async) {
	s.with(node, func(o *openRequest) { o.req.EditAsync(func(cur *model.Async) { *cur = a }) })
}

// SetProtocol moves the message request to another protocol; options the
// new protocol does not know are dropped.
func (s *RequestService) SetProtocol(node, protocol string) {
	s.with(node, func(o *openRequest) { o.req.SetProtocol(protocol) })
}

// SetMode makes the message request send, receive or both.
func (s *RequestService) SetMode(node, mode string) {
	s.with(node, func(o *openRequest) { o.req.SetMode(vm.AsyncMode(mode)) })
}

// SetGRPC replaces the gRPC block of the selected version.
func (s *RequestService) SetGRPC(node string, g model.GRPC) {
	s.with(node, func(o *openRequest) { o.req.EditGRPC(func(cur *model.GRPC) { *cur = g }) })
}

// LoadMethods asks for the methods of the request's schema (its .proto
// files, or the server's reflection); the "request" event delivers them.
func (s *RequestService) LoadMethods(node string) {
	s.with(node, func(o *openRequest) { s.app.send.LoadMethods(o.req) })
}

// PickMethod chooses a method and fills an empty message with its example.
func (s *RequestService) PickMethod(node, full string) {
	s.with(node, func(o *openRequest) {
		i := slices.IndexFunc(o.req.Methods(), func(m grpcx.Method) bool { return m.Full() == full })
		if i >= 0 {
			o.req.PickMethod(o.req.Methods()[i])
		}
	})
}

// PrettyJSON indents a JSON message; ok is false for text that is not JSON.
func (s *RequestService) PrettyJSON(text string) (pretty string, ok bool) { return vm.PrettyJSON(text) }
