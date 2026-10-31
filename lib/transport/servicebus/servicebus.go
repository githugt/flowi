// SPDX-License-Identifier: Apache-2.0

// Package servicebus is the Azure Service Bus transport: send to a queue or
// topic, receive from a queue or a topic subscription.
//
// By default (binding read lock) it receives in peek-lock mode: a message kept
// is completed (removed), and every other message received (skipped by the
// filter, our own, or not needed) is abandoned at the end of the run so
// another consumer can lock it; see package pull. Each abandon counts as a
// delivery, and a message delivered too often is dead-lettered. With binding
// read peek it only looks at the messages: nothing is locked, removed or
// counted, which is what a test of a live queue wants.
package servicebus

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/transport"
	shared "github.com/githugt/flowi-lib/transport/internal/pull"
)

const (
	closeTimeout = 10 * time.Second
	receiveBatch = 10
)

// Transport runs Service Bus operations.
type Transport struct{}

// New returns the Service Bus transport.
func New() Transport { return Transport{} }

// Run sends to and/or receives from the entity the channel names (a queue or
// topic; for a receive also topic/subscription). The server is the
// namespace. Binding: auth (default: Azure's DefaultAzureCredential;
// explicit: connection_string), read (lock or peek, for a receive), and for a
// send session_id, subject, message_id and ttl.
//
// Goroutine ownership: a receive starts one goroutine; Run stops it and waits
// for it before it settles the messages and closes the client.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	props, err := sendPropertiesOf(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	if _, err := readModeOf(op.Binding); err != nil {
		return transport.Result{}, err
	}
	client, err := newClient(op)
	if err != nil {
		return transport.Result{}, err
	}
	defer closeClient(ctx, client)
	res := transport.Result{Connect: op.Now().Sub(started)}
	err = run(ctx, client, op, props, &res)
	res.Total = op.Now().Sub(started)
	return res, err
}

func run(ctx context.Context, client *azservicebus.Client, op transport.Operation, props sendProperties, res *transport.Result) error {
	if op.Send != nil {
		if err := send(ctx, client, op, props); err != nil {
			return err
		}
		res.Sent = sentMessage(op, props)
	}
	if op.Receive == nil {
		return nil
	}
	var err error
	res.Received, err = receive(ctx, client, op, props.messageID)
	return err
}

// newClient signs in as the binding says: the default Azure credential
// against the namespace of the server, or an explicit connection string.
func newClient(op transport.Operation) (*azservicebus.Client, error) {
	switch mode := op.Binding["auth"]; mode {
	case "", model.AuthDefault:
		namespace, err := namespaceOf(op.Server)
		if err != nil {
			return nil, err
		}
		cred, err := azidentity.NewDefaultAzureCredential(nil)
		if err != nil {
			return nil, fmt.Errorf("finding Azure credentials (az login, environment or managed identity): %w", err)
		}
		return newNamespaceClient(namespace, cred)
	case model.AuthExplicit:
		cs, err := explicitConnection(op.Binding)
		if err != nil {
			return nil, err
		}
		// The SDK's own error can quote the string, so it is replaced.
		client, cerr := azservicebus.NewClientFromConnectionString(cs, nil)
		if cerr != nil {
			return nil, errors.New("binding connection_string could not be used to connect")
		}
		return client, nil
	default:
		return nil, fmt.Errorf("binding auth %q must be %s or %s", mode, model.AuthDefault, model.AuthExplicit)
	}
}

func newNamespaceClient(namespace string, cred *azidentity.DefaultAzureCredential) (*azservicebus.Client, error) {
	client, err := azservicebus.NewClient(namespace, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", namespace, err)
	}
	return client, nil
}

func closeClient(ctx context.Context, client *azservicebus.Client) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	_ = client.Close(closeCtx) // the run is over; a close failure changes nothing for the caller
}

func send(ctx context.Context, client *azservicebus.Client, op transport.Operation, props sendProperties) error {
	target, err := sendTarget(op.Channel)
	if err != nil {
		return err
	}
	sender, err := client.NewSender(target, nil)
	if err != nil {
		return fmt.Errorf("opening a sender for %s: %w", target, err)
	}
	defer closeWithin(ctx, sender.Close)
	if err := sender.SendMessage(ctx, outgoing(op, props), nil); err != nil {
		return fmt.Errorf("sending to %s: %w", target, err)
	}
	return nil
}

func outgoing(op transport.Operation, props sendProperties) *azservicebus.Message {
	msg := &azservicebus.Message{Body: op.Send.Body, MessageID: &props.messageID}
	if len(op.Send.Headers) > 0 {
		msg.ApplicationProperties = map[string]any{}
		for _, h := range op.Send.Headers {
			msg.ApplicationProperties[h.Name] = h.Value
		}
	}
	if props.sessionID != "" {
		msg.SessionID = &props.sessionID
	}
	if props.subject != "" {
		msg.Subject = &props.subject
	}
	if props.ttl > 0 {
		msg.TimeToLive = &props.ttl
	}
	return msg
}

func sentMessage(op transport.Operation, props sendProperties) *transport.Message {
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	m.Headers = append(append([]transport.Header(nil), m.Headers...), transport.Header{Name: "message_id", Value: props.messageID})
	return &m
}

func closeWithin(ctx context.Context, closer func(context.Context) error) {
	closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), closeTimeout)
	defer cancel()
	_ = closer(closeCtx) // see closeClient
}

// receive opens a peek-lock receiver, pulls until the receive is satisfied,
// then completes the kept messages and abandons the rest.
func receive(ctx context.Context, client *azservicebus.Client, op transport.Operation, ownID string) ([]transport.Message, error) {
	target, err := receiveTarget(op.Channel)
	if err != nil {
		return nil, err
	}
	receiver, err := openReceiver(client, target)
	if err != nil {
		return nil, err
	}
	defer closeWithin(ctx, receiver.Close)
	mode, _ := readModeOf(op.Binding) // Run checked it before connecting
	pump := func(ctx context.Context, book *shared.Book[*azservicebus.ReceivedMessage], in chan<- transport.Message) error {
		if mode == readPeek {
			return peekLoop(ctx, receiver, ownID, op.Now, book, in)
		}
		return pullLoop(ctx, receiver, ownID, op.Now, book, in)
	}
	settle := func(ctx context.Context, kept, others []*azservicebus.ReceivedMessage) error {
		if mode == readPeek {
			return nil // nothing was locked, so nothing is completed or given back
		}
		return settleLocks(ctx, receiver, kept, others)
	}
	return shared.Collect(ctx, *op.Receive, func(m transport.Message) string { return headerValue(m, "message_id") }, pump, settle)
}

func openReceiver(client *azservicebus.Client, target entity) (*azservicebus.Receiver, error) {
	opts := &azservicebus.ReceiverOptions{ReceiveMode: azservicebus.ReceiveModePeekLock}
	if target.queue != "" {
		r, err := client.NewReceiverForQueue(target.queue, opts)
		if err != nil {
			return nil, fmt.Errorf("opening a receiver for queue %s: %w", target.queue, err)
		}
		return r, nil
	}
	r, err := client.NewReceiverForSubscription(target.topic, target.subscription, opts)
	if err != nil {
		return nil, fmt.Errorf("opening a receiver for %s/%s: %w", target.topic, target.subscription, err)
	}
	return r, nil
}

// pullLoop receives batches until ctx ends, holding each locked message; the
// error is a failed receive, never the end of the run.
func pullLoop(ctx context.Context, receiver *azservicebus.Receiver, ownID string, now func() time.Time,
	book *shared.Book[*azservicebus.ReceivedMessage], in chan<- transport.Message) error {
	for ctx.Err() == nil {
		batch, err := receiver.ReceiveMessages(ctx, receiveBatch, nil)
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("receiving: %w", err)
		}
		for _, m := range batch {
			book.Hold(m.MessageID, m)
			if m.MessageID == ownID {
				continue
			}
			select {
			case in <- received(m, now()):
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

// peekPoll is how long peek waits before it looks again at an empty queue.
const peekPoll = time.Second

// peekLoop looks at the messages from the oldest on, a batch at a time, and
// goes on after the last sequence number it saw, until ctx ends. It locks
// nothing.
func peekLoop(ctx context.Context, receiver *azservicebus.Receiver, ownID string, now func() time.Time,
	book *shared.Book[*azservicebus.ReceivedMessage], in chan<- transport.Message) error {
	var next *int64
	for ctx.Err() == nil {
		batch, err := receiver.PeekMessages(ctx, receiveBatch, &azservicebus.PeekMessagesOptions{FromSequenceNumber: next})
		if err != nil && ctx.Err() == nil {
			return fmt.Errorf("peeking: %w", err)
		}
		if len(batch) == 0 {
			select {
			case <-time.After(peekPoll):
			case <-ctx.Done():
			}
			continue
		}
		for _, m := range batch {
			if m.SequenceNumber != nil {
				after := *m.SequenceNumber + 1
				next = &after
			}
			book.Hold(m.MessageID, m)
			if m.MessageID == ownID {
				continue
			}
			select {
			case in <- received(m, now()):
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

// settleLocks completes the kept messages and abandons all the others.
func settleLocks(ctx context.Context, receiver *azservicebus.Receiver, kept, others []*azservicebus.ReceivedMessage) error {
	var errs []error
	for _, m := range kept {
		errs = append(errs, receiver.CompleteMessage(ctx, m, nil))
	}
	for _, m := range others {
		errs = append(errs, receiver.AbandonMessage(ctx, m, nil))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("settling messages: %w", err)
	}
	return nil
}

// received turns a message into a body and headers: the service's own fields
// first, then the application properties in name order.
func received(m *azservicebus.ReceivedMessage, at time.Time) transport.Message {
	headers := []transport.Header{{Name: "message_id", Value: m.MessageID}}
	add := func(name string, value *string) {
		if value != nil && *value != "" {
			headers = append(headers, transport.Header{Name: name, Value: *value})
		}
	}
	add("subject", m.Subject)
	add("session_id", m.SessionID)
	add("correlation_id", m.CorrelationID)
	add("content_type", m.ContentType)
	if m.DeliveryCount > 0 {
		headers = append(headers, transport.Header{Name: "delivery_count", Value: fmt.Sprint(m.DeliveryCount)})
	}
	if m.EnqueuedTime != nil {
		headers = append(headers, transport.Header{Name: "enqueued_time", Value: m.EnqueuedTime.UTC().Format(time.RFC3339Nano)})
	}
	names := make([]string, 0, len(m.ApplicationProperties))
	for name := range m.ApplicationProperties {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		headers = append(headers, transport.Header{Name: name, Value: fmt.Sprint(m.ApplicationProperties[name])})
	}
	return transport.Message{Direction: "received", At: at, Headers: headers, Body: m.Body}
}

func headerValue(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}
