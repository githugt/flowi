// SPDX-License-Identifier: Apache-2.0

package model

// Family is the kind of conversation a request has with what it talks to. Every request version belongs to exactly
// one family; what differs inside a family (an HTTP method, a message protocol, a SQL driver) is that family's own
// detail. New kinds of request are new families, not new branches in the code that handles the existing ones.
type Family string

// The families.
const (
	// FamilyHTTP is a request and its response over HTTP.
	FamilyHTTP Family = "http"
	// FamilyRPC is a call of a method of a remote service (gRPC).
	FamilyRPC Family = "rpc"
	// FamilyMessaging is a message sent to or received from a channel (WebSocket, SSE, MQTT, Kafka, AMQP, NATS, …).
	FamilyMessaging Family = "messaging"
	// FamilyDataset is a query of a data store, answered with rows (SQL).
	FamilyDataset Family = "dataset"
)

// Exchange is which directions a request has: what it sends and what it receives.
type Exchange uint8

// The exchanges.
const (
	// SendOnly sends and does not wait for an answer (a published message).
	SendOnly Exchange = iota + 1
	// ReceiveOnly waits for what arrives and sends nothing (a subscription).
	ReceiveOnly
	// SendReceive sends and receives the answer (an HTTP request, a gRPC call, a query, request and reply).
	SendReceive
)

// String is the exchange as written in documents.
func (e Exchange) String() string {
	switch e {
	case SendOnly:
		return "send"
	case ReceiveOnly:
		return "receive"
	case SendReceive:
		return "send_receive"
	}
	return ""
}

// Sends reports whether the request sends something.
func (e Exchange) Sends() bool { return e == SendOnly || e == SendReceive }

// Receives reports whether the request receives something.
func (e Exchange) Receives() bool { return e == ReceiveOnly || e == SendReceive }

// Family is the family of the version: the block that is set decides, and a version with none is HTTP.
func (v RequestVersion) Family() Family {
	switch {
	case v.Dataset != nil:
		return FamilyDataset
	case v.GRPC != nil:
		return FamilyRPC
	case v.Async != nil:
		return FamilyMessaging
	}
	return FamilyHTTP
}

// Exchange is which directions the version has. Only messaging varies: a published message sends, a subscription
// receives, request and reply does both; every other family sends and receives the answer.
func (v RequestVersion) Exchange() Exchange {
	if v.Family() != FamilyMessaging {
		return SendReceive
	}
	switch v.Async.Action() {
	case AsyncSendOnly:
		return SendOnly
	case AsyncReceiveOnly:
		return ReceiveOnly
	}
	return SendReceive
}
