// SPDX-License-Identifier: Apache-2.0

// Package pubsub is the Google Cloud Pub/Sub transport: publish to a topic,
// pull from a subscription.
//
// A message kept is acknowledged. A message the filter skips (or that was
// pulled but not needed) is nacked when the run ends so another consumer can
// have it.
package pubsub

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ps "cloud.google.com/go/pubsub/v2"
	vkit "cloud.google.com/go/pubsub/v2/apiv1"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/transport"
	shared "github.com/githugt/flowi-lib/transport/internal/pull"
)

const (
	connectTimeout = 10 * time.Second
	settleTimeout  = 10 * time.Second
	pullBatch      = 10 // messages asked for per pull
)

var errBadServer = errors.New("server must be pubsub://<project>")

// Transport runs Pub/Sub operations.
type Transport struct{}

// New returns the Pub/Sub transport.
func New() Transport { return Transport{} }

// Run publishes to the topic named by the channel and/or pulls from the
// subscription of that name. Binding: auth (default: Application Default
// Credentials; explicit: credentials_json, the JSON itself or a path to it),
// endpoint (host:port of an emulator, no credentials), ordering_key.
//
// Goroutine ownership: a receive starts one pull goroutine; Run cancels it
// and waits for it before it settles the messages and returns.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	projectID, err := projectOf(op.Server)
	if err != nil {
		return transport.Result{}, err
	}
	opts, err := clientOptions(op.Binding)
	if err != nil {
		return transport.Result{}, err
	}
	client, err := ps.NewClient(ctx, projectID, opts...)
	if err != nil {
		return transport.Result{}, fmt.Errorf("connecting to Pub/Sub project %s: %w", projectID, err)
	}
	defer client.Close() //nolint:errcheck // nothing useful to do if closing fails
	res := transport.Result{Connect: op.Now().Sub(started)}
	err = run(ctx, client, op, &res)
	res.Total = op.Now().Sub(started)
	return res, err
}

func run(ctx context.Context, client *ps.Client, op transport.Operation, res *transport.Result) error {
	var ownID string
	if op.Send != nil {
		id, err := publish(ctx, client, op)
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
	res.Received, err = pull(ctx, client, op, ownID)
	return err
}

func projectOf(server string) (string, error) {
	project, found := strings.CutPrefix(server, "pubsub://")
	project = strings.Trim(project, "/")
	if !found || project == "" {
		return "", errBadServer
	}
	return project, nil
}

func publish(ctx context.Context, client *ps.Client, op transport.Operation) (string, error) {
	pub := client.Publisher(op.Channel)
	defer pub.Stop()
	key := op.Binding["ordering_key"]
	pub.EnableMessageOrdering = key != ""
	msg := &ps.Message{Data: op.Send.Body, OrderingKey: key}
	if len(op.Send.Headers) > 0 {
		msg.Attributes = map[string]string{}
		for _, h := range op.Send.Headers {
			msg.Attributes[h.Name] = h.Value
		}
	}
	id, err := pub.Publish(ctx, msg).Get(ctx)
	if err != nil {
		return "", fmt.Errorf("publishing to %s: %w", op.Channel, err)
	}
	return id, nil
}

func sentMessage(op transport.Operation, id string) *transport.Message {
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	m.Headers = append(append([]transport.Header(nil), m.Headers...), transport.Header{Name: "message_id", Value: id})
	return &m
}

// pull reads the subscription with unary Pull calls until the receive is
// satisfied. The messages kept are acknowledged when the receive ends; every
// other message pulled (skipped, our own, pulled but not needed) is nacked
// then, so another consumer can have it (see package pull).
//
// Unary calls, not the library's streaming Receive: acknowledgements there
// are sent in the background and are lost when the client is closed.
func pull(ctx context.Context, client *ps.Client, op transport.Operation, ownID string) ([]transport.Message, error) {
	api := client.SubscriptionAdminClient
	name := subscriptionName(client.Project(), op.Channel)
	pump := func(ctx context.Context, book *shared.Book[string], in chan<- transport.Message) error {
		return pullLoop(ctx, api, name, ownID, op.Now, book, in)
	}
	settle := func(ctx context.Context, kept, others []string) error {
		return settleAcks(ctx, api, name, kept, others)
	}
	return shared.Collect(ctx, *op.Receive, func(m transport.Message) string { return headerValue(m, "message_id") }, pump, settle)
}

func subscriptionName(project, channel string) string {
	if strings.HasPrefix(channel, "projects/") {
		return channel
	}
	return "projects/" + project + "/subscriptions/" + channel
}

// pullLoop pulls until ctx ends, holding each message's ack id; the error is
// a failed pull, never the end of the run.
func pullLoop(ctx context.Context, api *vkit.SubscriptionAdminClient, name, ownID string, now func() time.Time,
	book *shared.Book[string], in chan<- transport.Message) error {
	for ctx.Err() == nil {
		resp, err := api.Pull(ctx, &pubsubpb.PullRequest{Subscription: name, MaxMessages: pullBatch})
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("pulling from %s: %w", name, err)
		}
		for _, rm := range resp.GetReceivedMessages() {
			m := rm.GetMessage()
			book.Hold(m.GetMessageId(), rm.GetAckId())
			if m.GetMessageId() == ownID {
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

// settleAcks acknowledges the ack ids kept and nacks (a zero ack deadline)
// all the others.
func settleAcks(ctx context.Context, api *vkit.SubscriptionAdminClient, name string, kept, others []string) error {
	var errs []error
	if len(kept) > 0 {
		errs = append(errs, api.Acknowledge(ctx, &pubsubpb.AcknowledgeRequest{Subscription: name, AckIds: kept}))
	}
	if len(others) > 0 {
		errs = append(errs, api.ModifyAckDeadline(ctx, &pubsubpb.ModifyAckDeadlineRequest{Subscription: name, AckIds: others, AckDeadlineSeconds: 0}))
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("settling messages on %s: %w", name, err)
	}
	return nil
}

func received(m *pubsubpb.PubsubMessage, at time.Time) transport.Message {
	headers := []transport.Header{
		{Name: "message_id", Value: m.GetMessageId()},
		{Name: "publish_time", Value: m.GetPublishTime().AsTime().UTC().Format(time.RFC3339Nano)},
	}
	if m.GetOrderingKey() != "" {
		headers = append(headers, transport.Header{Name: "ordering_key", Value: m.GetOrderingKey()})
	}
	attrs := m.GetAttributes()
	for _, name := range sortedKeys(attrs) {
		headers = append(headers, transport.Header{Name: name, Value: attrs[name]})
	}
	return transport.Message{Direction: "received", At: at, Headers: headers, Body: m.GetData()}
}

func headerValue(m transport.Message, name string) string {
	for _, h := range m.Headers {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

// clientOptions turns the binding into client options: an emulator endpoint
// (no credentials), or the explicit credentials, or none (Application
// Default Credentials).
func clientOptions(binding map[string]string) ([]option.ClientOption, error) {
	var creds option.ClientOption
	if binding["auth"] == model.AuthExplicit {
		var err error
		if creds, err = explicitCredentials(binding["credentials_json"]); err != nil {
			return nil, err
		}
	}
	if endpoint := binding["endpoint"]; endpoint != "" {
		return []option.ClientOption{
			option.WithEndpoint(endpoint), option.WithoutAuthentication(),
			option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())),
		}, nil
	}
	if creds == nil {
		return nil, nil
	}
	return []option.ClientOption{creds}, nil
}
