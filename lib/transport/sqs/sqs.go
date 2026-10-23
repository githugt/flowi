// SPDX-License-Identifier: Apache-2.0

// Package sqs is the AWS SQS transport: send to a queue, long-poll a queue.
//
// A message kept is deleted from the queue (unless binding delete is false).
// Every other message received (skipped by the filter, our own, or not
// needed) is returned to the queue at the end of the run by setting its
// visibility to zero, so another consumer has it at once instead of after the
// visibility timeout. See package pull for why that happens at the end.
package sqs

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssqs "github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/internal/awscfg"
	shared "github.com/githugt/flowi-lib/transport/internal/pull"
)

const (
	maxBatch     = 10 // SQS returns at most 10 messages per call
	maxWait      = 20 // longest long-poll SQS allows, in seconds
	attrAll      = "All"
	typeString   = "String"
	receiveCount = "ApproximateReceiveCount"
	groupID      = "MessageGroupId"
)

// Transport runs SQS operations.
type Transport struct{}

// New returns the SQS transport.
func New() Transport { return Transport{} }

// Run sends to and/or receives from the queue named by the channel (a queue
// name, looked up with GetQueueUrl, or a queue URL used as given). The
// server is an endpoint override (LocalStack) or empty for AWS. Binding:
// auth, region, profile and the credentials (see package awscfg); for a send
// group_id and dedup_id (FIFO queues) and delay (seconds); for a receive
// visibility_timeout (seconds), wait (long-poll seconds, 1 to 20, default
// 20) and delete (true by default).
//
// Goroutine ownership: a receive starts one polling goroutine; Run stops it
// and waits for it before it returns.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	settings, err := settingsOf(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	cfg, err := awscfg.Load(ctx, op)
	if err != nil {
		return transport.Result{}, err
	}
	client := awssqs.NewFromConfig(cfg, func(o *awssqs.Options) {
		if op.Server != "" {
			o.BaseEndpoint = aws.String(op.Server)
		}
	})
	queueURL, err := resolveQueue(ctx, client, op.Channel)
	if err != nil {
		return transport.Result{}, err
	}
	res := transport.Result{Connect: op.Now().Sub(started)}
	err = run(ctx, client, queueURL, op, settings, &res)
	res.Total = op.Now().Sub(started)
	return res, err
}

func run(ctx context.Context, client *awssqs.Client, queueURL string, op transport.Operation, s settings, res *transport.Result) error {
	var ownID string
	if op.Send != nil {
		id, err := send(ctx, client, queueURL, op, s)
		if err != nil {
			return err
		}
		ownID = id
		res.Sent = sentMessage(op, id)
	}
	if op.Receive == nil {
		return nil
	}
	var err error
	res.Received, err = receive(ctx, client, queueURL, op, s, ownID)
	return err
}

// settings are the binding options that are numbers or flags, parsed once so
// a bad value is reported before anything is sent.
type settings struct {
	delay, visibility int32
	hasVisibility     bool
	wait              int32
	deleteKept        bool
	group, dedup      string
}

func settingsOf(binding map[string]string) (settings, error) {
	s := settings{wait: maxWait, deleteKept: true, group: binding["group_id"], dedup: binding["dedup_id"]}
	var err error
	if s.delay, _, err = optionalSeconds(binding, "delay", 0, 900); err != nil {
		return s, err
	}
	if s.visibility, s.hasVisibility, err = optionalSeconds(binding, "visibility_timeout", 0, 43200); err != nil {
		return s, err
	}
	if w, has, err := optionalSeconds(binding, "wait", 1, maxWait); err != nil {
		return s, err
	} else if has {
		s.wait = w
	}
	if raw := binding["delete"]; raw != "" {
		if s.deleteKept, err = strconv.ParseBool(raw); err != nil {
			return s, fmt.Errorf("binding delete %q must be true or false", raw)
		}
	}
	return s, nil
}

// optionalSeconds reads a whole number of seconds within [lo, hi]; has is
// false when the binding is absent.
func optionalSeconds(binding map[string]string, name string, lo, hi int) (value int32, has bool, err error) {
	raw := binding[name]
	if raw == "" {
		return 0, false, nil
	}
	n, perr := strconv.Atoi(raw)
	if perr != nil || n < lo || n > hi {
		return 0, false, fmt.Errorf("binding %s %q must be a whole number from %d to %d", name, raw, lo, hi)
	}
	return int32(n), true, nil //nolint:gosec // bounded by hi above
}

func resolveQueue(ctx context.Context, client *awssqs.Client, channel string) (string, error) {
	if strings.Contains(channel, "://") {
		return channel, nil
	}
	out, err := client.GetQueueUrl(ctx, &awssqs.GetQueueUrlInput{QueueName: aws.String(channel)})
	if err != nil {
		return "", fmt.Errorf("finding queue %s: %w", channel, err)
	}
	return aws.ToString(out.QueueUrl), nil
}

func send(ctx context.Context, client *awssqs.Client, queueURL string, op transport.Operation, s settings) (string, error) {
	if !utf8.Valid(op.Send.Body) {
		return "", errors.New("an SQS message body must be text (UTF-8)")
	}
	in := &awssqs.SendMessageInput{QueueUrl: aws.String(queueURL), MessageBody: aws.String(string(op.Send.Body)), DelaySeconds: s.delay}
	if len(op.Send.Headers) > 0 {
		in.MessageAttributes = map[string]types.MessageAttributeValue{}
		for _, h := range op.Send.Headers {
			in.MessageAttributes[h.Name] = types.MessageAttributeValue{DataType: aws.String(typeString), StringValue: aws.String(h.Value)}
		}
	}
	if s.group != "" {
		in.MessageGroupId = aws.String(s.group)
	}
	if s.dedup != "" {
		in.MessageDeduplicationId = aws.String(s.dedup)
	}
	out, err := client.SendMessage(ctx, in)
	if err != nil {
		return "", fmt.Errorf("sending to %s: %w", op.Channel, err)
	}
	return aws.ToString(out.MessageId), nil
}

func sentMessage(op transport.Operation, id string) *transport.Message {
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	m.Headers = append(append([]transport.Header(nil), m.Headers...), transport.Header{Name: "message_id", Value: id})
	return &m
}

// receive long-polls until the receive is satisfied, then settles.
func receive(ctx context.Context, client *awssqs.Client, queueURL string, op transport.Operation, s settings, ownID string) ([]transport.Message, error) {
	pump := func(ctx context.Context, book *shared.Book[string], in chan<- transport.Message) error {
		return poll(ctx, client, queueURL, pollPlan{settings: s, ownID: ownID, now: op.Now}, book, in)
	}
	settle := func(ctx context.Context, kept, others []string) error {
		return settleHandles(ctx, client, queueURL, s.deleteKept, kept, others)
	}
	return shared.Collect(ctx, *op.Receive, func(m transport.Message) string { return headerValue(m, "message_id") }, pump, settle)
}

type pollPlan struct {
	settings
	ownID string
	now   func() time.Time
}

// poll receives batches until ctx ends, holding each receipt handle; the
// error is a failed call, never the end of the run.
func poll(ctx context.Context, client *awssqs.Client, queueURL string, p pollPlan, book *shared.Book[string], in chan<- transport.Message) error {
	for ctx.Err() == nil {
		input := &awssqs.ReceiveMessageInput{
			QueueUrl: aws.String(queueURL), MaxNumberOfMessages: maxBatch, WaitTimeSeconds: p.wait,
			MessageAttributeNames: []string{attrAll}, MessageSystemAttributeNames: []types.MessageSystemAttributeName{attrAll},
		}
		if p.hasVisibility {
			input.VisibilityTimeout = p.visibility
		}
		out, err := client.ReceiveMessage(ctx, input)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("receiving from %s: %w", queueURL, err)
		}
		for _, m := range out.Messages {
			book.Hold(aws.ToString(m.MessageId), aws.ToString(m.ReceiptHandle))
			if aws.ToString(m.MessageId) == p.ownID {
				continue
			}
			select {
			case in <- received(m, p.now()):
			case <-ctx.Done():
				return nil
			}
		}
	}
	return nil
}

// settleHandles deletes the kept messages (or returns them when delete is
// false) and returns all the others to the queue.
func settleHandles(ctx context.Context, client *awssqs.Client, queueURL string, deleteKept bool, kept, others []string) error {
	var errs []error
	for _, handle := range kept {
		if deleteKept {
			_, err := client.DeleteMessage(ctx, &awssqs.DeleteMessageInput{QueueUrl: aws.String(queueURL), ReceiptHandle: aws.String(handle)})
			errs = append(errs, err)
			continue
		}
		others = append(others, handle)
	}
	for _, handle := range others {
		_, err := client.ChangeMessageVisibility(ctx, &awssqs.ChangeMessageVisibilityInput{
			QueueUrl: aws.String(queueURL), ReceiptHandle: aws.String(handle), VisibilityTimeout: 0})
		errs = append(errs, err)
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("settling messages on %s: %w", queueURL, err)
	}
	return nil
}

// received turns a message into headers (message_id, receive_count,
// group_id, then the message attributes in name order) and a body.
func received(m types.Message, at time.Time) transport.Message {
	headers := []transport.Header{{Name: "message_id", Value: aws.ToString(m.MessageId)}}
	if n := m.Attributes[receiveCount]; n != "" {
		headers = append(headers, transport.Header{Name: "receive_count", Value: n})
	}
	if g := m.Attributes[groupID]; g != "" {
		headers = append(headers, transport.Header{Name: "group_id", Value: g})
	}
	names := make([]string, 0, len(m.MessageAttributes))
	for name := range m.MessageAttributes {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		headers = append(headers, transport.Header{Name: name, Value: aws.ToString(m.MessageAttributes[name].StringValue)})
	}
	return transport.Message{Direction: "received", At: at, Headers: headers, Body: []byte(aws.ToString(m.Body))}
}

func headerValue(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}
