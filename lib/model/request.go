// SPDX-License-Identifier: Apache-2.0

package model

import "time"

// Body is a request body. At most one field is set.
type Body struct {
	JSON      string
	Text      string
	File      string
	Form      []KeyValue
	Multipart []Field
}

// Extraction names a value taken from a response. From is one of body,
// header, cookie or status; the matching selector field is set.
type Extraction struct {
	Name   string
	From   string
	Path   string
	Header string
	Cookie string
	Regex  string
}

// RequestVersion is one complete definition of a request.
type RequestVersion struct {
	Label string
	// Name is a short title for the version ("Before the tax change"); the label
	// ("1.1") stays what it is called everywhere else. Docs is its description.
	Name     string
	Method   string
	URL      string
	Query    []KeyValue
	Headers  []KeyValue
	Body     Body
	Extract  []Extraction
	Assert   []string
	Settings Settings
	Hooks    Hooks
	// Variables are the version's default values for {{var.name}}; the
	// environment's variable of the same name wins.
	Variables []KeyValue
	// Docs is the version's documentation, written as Markdown.
	Docs string
	// Auth, when set, signs or authenticates the request when it is sent: an
	// API key, Digest, OAuth 2.0 or 1.0a, AWS Signature v4, a signed JWT, Hawk
	// or a client certificate. The Authorization header of Headers is the
	// simple form of the same thing.
	Auth *Auth
	// Async, when set, makes this an async (message) request instead of an
	// HTTP one: Method, Query and Body are then unused.
	Async *Async
	// GRPC, when set, makes this a gRPC call instead of an HTTP request. The
	// version's Headers are the call's metadata; Method, URL, Query and Body
	// are unused.
	GRPC *GRPC
}

// Async is what an async request version does: it sends a message to a
// channel, receives messages from it, or both (request and reply).
type Async struct {
	Protocol string // ws, sse, mqtt, kafka, amqp or nats
	Server   string // templated; e.g. mqtt://host:1883
	Channel  string // topic, subject, queue, or routing key
	Send     *AsyncSend
	Receive  *AsyncReceive
	// Binding is the protocol's own options (qos, key, exchange, …), in
	// file order; the allowed names depend on the protocol.
	Binding []KeyValue
}

// AsyncSend is the message an async request sends.
type AsyncSend struct {
	Body    Body
	Headers []KeyValue
}

// AsyncReceive says which messages an async request waits for. Count 0 means
// 1 and Timeout 0 means 30s when it runs.
type AsyncReceive struct {
	Count   int
	Timeout time.Duration
	// Match, when set, is an expression a message must satisfy to count.
	Match string
}

// The actions an Async version can have.
const (
	AsyncSendOnly     = "send"
	AsyncReceiveOnly  = "receive"
	AsyncRequestReply = "request_reply"
)

// Action is send, receive or request_reply; "" when neither side is set.
func (a Async) Action() string {
	switch {
	case a.Send != nil && a.Receive != nil:
		return AsyncRequestReply
	case a.Send != nil:
		return AsyncSendOnly
	case a.Receive != nil:
		return AsyncReceiveOnly
	}
	return ""
}

// Request is a saved API request with one or more versions. Path is
// relative to requests/ without the .req.yaml suffix, e.g. "auth/login".
type Request struct {
	Meta
	// Tags label the request; the tags of its folders apply as well.
	Tags      []string
	Path      string
	Operation string // the OpenAPI operationId of its first version; Arazzo steps refer to it
	Default   string
	Versions  []RequestVersion
	Hooks     Hooks
	File      string

	// Edit is the one dirty edit: an autosaved working copy of a version.
	// Runs, serve and the CLI ignore it; only the GUI sends it. EditBase is
	// the label of the version it started from.
	Edit     *RequestVersion
	EditBase string
	// EditOverlay names the flowi overlay the edit was made with laid over
	// its base version: the edit then shows the overlay's changes too, and the
	// window restores the overlay choice with it. "" when there was none.
	EditOverlay string
}
