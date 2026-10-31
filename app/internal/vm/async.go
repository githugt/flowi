// SPDX-License-Identifier: AGPL-3.0-or-later

package vm

import (
	"slices"

	"github.com/githugt/flowi-lib/model"
)

// The sections only an async request has.
const (
	SectionMessage Section = "message" // what it sends
	SectionReceive Section = "receive" // what it waits for
	SectionBinding Section = "binding" // the protocol's own options

	// SectionCredentials is the Authentication section of an async request:
	// how it signs in to the broker or cloud service.
	SectionCredentials Section = "credentials"
)

// Verb is what stands where an HTTP request shows its method: the method
// itself, or the protocol of an async version (MQTT, WS, …).
func Verb(v model.RequestVersion) string {
	if v.Dataset != nil {
		return datasetVerb
	}
	if v.GRPC != nil {
		return model.ProtocolVerbGRPC
	}
	if v.Async != nil {
		return model.ProtocolVerb(v.Async.Protocol)
	}
	return v.Method
}

// AsyncHeadersAreConnection says what the Headers section of an async
// request edits: the connection's headers (the WebSocket handshake, the SSE
// request) for ws and sse, the sent message's headers for the others.
func AsyncHeadersAreConnection(protocol string) bool {
	return protocol == model.ProtocolWS || protocol == model.ProtocolSSE
}

// IsAsync reports whether the selected version is an async one.
func (r *Request) IsAsync() bool { return r.Current().Async != nil }

// Async is a copy of the selected version's async block; the zero value for
// an HTTP version.
func (r *Request) Async() model.Async {
	if a := r.Current().Async; a != nil {
		return *cloneAsync(a)
	}
	return model.Async{}
}

// EditAsync changes the async block of the selected version; it does nothing
// for an HTTP one.
func (r *Request) EditAsync(f func(a *model.Async)) {
	if !r.IsAsync() {
		return
	}
	r.Edit(func(v *model.RequestVersion) { f(v.Async) })
}

// SetProtocol moves the selected async version to another protocol. Binding
// options the new protocol does not know are dropped, so the switch never
// leaves the version invalid.
func (r *Request) SetProtocol(protocol string) {
	r.EditAsync(func(a *model.Async) {
		a.Protocol = protocol
		known := model.AsyncBindings(protocol)
		a.Binding = slices.DeleteFunc(a.Binding, func(kv model.KeyValue) bool { return !slices.Contains(known, kv.Key) })
		switch protocol {
		case model.ProtocolSSE: // a one-way stream: it listens, and cannot send
			a.Send = nil
			if a.Receive == nil {
				a.Receive = &model.AsyncReceive{}
			}
		case model.ProtocolSNS: // it only publishes
			a.Receive = nil
			if a.Send == nil {
				a.Send = &model.AsyncSend{}
			}
		}
	})
}

// cloneAsync deep-copies an async block, so an edit never reaches the copy
// it came from.
func cloneAsync(a *model.Async) *model.Async {
	if a == nil {
		return nil
	}
	out := *a
	out.Binding = slices.Clone(a.Binding)
	if a.Send != nil {
		s := *a.Send
		s.Headers = slices.Clone(a.Send.Headers)
		s.Body.Form, s.Body.Multipart = slices.Clone(a.Send.Body.Form), slices.Clone(a.Send.Body.Multipart)
		out.Send = &s
	}
	if a.Receive != nil {
		rc := *a.Receive
		out.Receive = &rc
	}
	return &out
}

// asyncConfigured is whether section s of an async version has content.
func asyncConfigured(a model.Async, s Section) bool {
	switch s {
	case SectionMessage:
		return a.Send != nil && (bodyHasContent(a.Send.Body) || len(a.Send.Headers) > 0)
	case SectionReceive:
		return a.Receive != nil && (a.Receive.Count > 0 || a.Receive.Timeout > 0 || a.Receive.Match != "")
	case SectionBinding:
		return len(optionsOf(a)) > 0
	case SectionCredentials:
		return len(credentialsOf(a)) > 0 || authModeOf(a) == model.AuthExplicit
	}
	return false
}

// asyncTip is the target of a version as a tooltip: its URL, or for an async
// one the server and channel.
func asyncTip(v model.RequestVersion) string {
	if v.Dataset != nil {
		return datasetTip(*v.Dataset)
	}
	if v.GRPC != nil {
		return model.GRPCTargetURL(v.GRPC.Server, v.GRPC.Method)
	}
	if v.Async == nil {
		return v.URL
	}
	return model.TargetURL(v.Async.Protocol, model.NormalizeServer(v.Async.Protocol, v.Async.Server), v.Async.Channel)
}

// AsyncMode is what running an async request does: send, receive, or send and
// then receive (request and reply).
type AsyncMode string

// The modes of an async request.
const (
	ModeSend        AsyncMode = "send"
	ModeReceive     AsyncMode = "receive"
	ModeSendReceive AsyncMode = "send_receive"
)

// AsyncModesFor lists the modes a protocol lets the user choose between, in
// menu order; nil for the one-way protocols (SSE listens, SNS publishes).
func AsyncModesFor(protocol string) []AsyncMode {
	if protocol == model.ProtocolSSE || protocol == model.ProtocolSNS {
		return nil
	}
	return []AsyncMode{ModeSend, ModeReceive, ModeSendReceive}
}

// Mode is what the selected async version does when it runs.
func (r *Request) Mode() AsyncMode {
	a := r.Async()
	switch {
	case a.Send != nil && a.Receive != nil:
		return ModeSendReceive
	case a.Receive != nil:
		return ModeReceive
	}
	return ModeSend
}

// SetMode makes the selected async version send, receive or both. The side
// that is switched off is dropped, as unticking it in its section does.
func (r *Request) SetMode(m AsyncMode) {
	r.EditAsync(func(a *model.Async) {
		if m != ModeReceive && a.Send == nil {
			a.Send = &model.AsyncSend{}
		}
		if m != ModeSend && a.Receive == nil {
			a.Receive = &model.AsyncReceive{}
		}
		if m == ModeReceive {
			a.Send = nil
		}
		if m == ModeSend {
			a.Receive = nil
		}
	})
}
