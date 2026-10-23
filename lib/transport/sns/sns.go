// SPDX-License-Identifier: Apache-2.0

// Package sns is the AWS SNS transport. SNS only publishes: to read what a
// topic carries, subscribe a queue to it and receive from the queue (sqs).
package sns

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awssns "github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sns/types"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/internal/awscfg"
)

const typeString = "String"

var (
	errNoReceive = errors.New("sns only publishes; subscribe a queue to the topic and receive from the queue (sqs)")
	errNotText   = errors.New("an SNS message body must be text (UTF-8)")
)

// Transport runs SNS operations.
type Transport struct{}

// New returns the SNS transport.
func New() Transport { return Transport{} }

// Run publishes to the topic named by the channel: a topic ARN used as
// given, or a topic name looked up with ListTopics. The server is an
// endpoint override (LocalStack) or empty for AWS. Binding: auth, region,
// profile and the credentials (see package awscfg), subject, and group_id and
// dedup_id for FIFO topics. The message headers become message attributes.
func (Transport) Run(ctx context.Context, op transport.Operation) (transport.Result, error) {
	started := op.Now()
	if op.Receive != nil {
		return transport.Result{}, errNoReceive
	}
	if op.Send != nil && !utf8.Valid(op.Send.Body) {
		return transport.Result{}, errNotText
	}
	cfg, err := awscfg.Load(ctx, op)
	if err != nil {
		return transport.Result{}, err
	}
	client := awssns.NewFromConfig(cfg, func(o *awssns.Options) {
		if op.Server != "" {
			o.BaseEndpoint = aws.String(op.Server)
		}
	})
	res := transport.Result{Connect: op.Now().Sub(started)}
	if op.Send != nil {
		err = publish(ctx, client, op, &res)
	}
	res.Total = op.Now().Sub(started)
	return res, err
}

func publish(ctx context.Context, client *awssns.Client, op transport.Operation, res *transport.Result) error {
	topic, err := resolveTopic(ctx, client, op.Channel)
	if err != nil {
		return err
	}
	in := &awssns.PublishInput{TopicArn: aws.String(topic), Message: aws.String(string(op.Send.Body))}
	if s := op.Binding["subject"]; s != "" {
		in.Subject = aws.String(s)
	}
	if g := op.Binding["group_id"]; g != "" {
		in.MessageGroupId = aws.String(g)
	}
	if d := op.Binding["dedup_id"]; d != "" {
		in.MessageDeduplicationId = aws.String(d)
	}
	if len(op.Send.Headers) > 0 {
		in.MessageAttributes = map[string]types.MessageAttributeValue{}
		for _, h := range op.Send.Headers {
			in.MessageAttributes[h.Name] = types.MessageAttributeValue{DataType: aws.String(typeString), StringValue: aws.String(h.Value)}
		}
	}
	out, err := client.Publish(ctx, in)
	if err != nil {
		return fmt.Errorf("publishing to %s: %w", op.Channel, err)
	}
	m := *op.Send
	m.Direction, m.At = "sent", op.Now()
	m.Headers = append(append([]transport.Header(nil), m.Headers...), transport.Header{Name: "message_id", Value: aws.ToString(out.MessageId)})
	res.Sent = &m
	return nil
}

// resolveTopic returns the channel when it is an ARN, else the ARN of the
// topic with that name (the part after the last colon).
func resolveTopic(ctx context.Context, client *awssns.Client, channel string) (string, error) {
	if strings.HasPrefix(channel, "arn:") {
		return channel, nil
	}
	pages := awssns.NewListTopicsPaginator(client, &awssns.ListTopicsInput{})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return "", fmt.Errorf("listing topics to find %s: %w", channel, err)
		}
		for _, t := range page.Topics {
			arn := aws.ToString(t.TopicArn)
			if arn[strings.LastIndex(arn, ":")+1:] == channel {
				return arn, nil
			}
		}
	}
	return "", fmt.Errorf("no topic named %s (use its ARN to be exact)", channel)
}
