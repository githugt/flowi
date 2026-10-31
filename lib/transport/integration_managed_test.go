// SPDX-License-Identifier: Apache-2.0

//go:build integration

// Redis, SQS, SNS and Pub/Sub against the emulators of
// testdata/compose/docker-compose.yml (just test-integration). Addresses can
// be changed with FLOWI_TEST_REDIS, _LOCALSTACK and _PUBSUB. Azure Service
// Bus has an emulator behind a compose profile; its test runs only when
// FLOWI_TEST_SERVICEBUS holds a connection string.

package transport_test

import (
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/githugt/flowi/lib/transport"
	pubsubtransport "github.com/githugt/flowi/lib/transport/pubsub"
	"github.com/githugt/flowi/lib/transport/redis"
	"github.com/githugt/flowi/lib/transport/servicebus"
	snstransport "github.com/githugt/flowi/lib/transport/sns"
	sqstransport "github.com/githugt/flowi/lib/transport/sqs"
)

func TestRedis_against_redis_pubsub_and_streams(t *testing.T) {
	server := address("FLOWI_TEST_REDIS", "redis://127.0.0.1:6379")
	waitForPort(t, strings.Replace(server, "redis://", "tcp://", 1))

	t.Run("stream: add, then read from the start", func(t *testing.T) {
		stream := unique("flowi-it")
		binding := map[string]string{"mode": "stream", "start": "0"}
		_, err := redis.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: stream, Binding: binding, Send: &transport.Message{Body: []byte("one"), Headers: []transport.Header{{Name: "kind", Value: "a"}}}})
		got, err2 := redis.New().Run(t.Context(), transport.Operation{
			Server: server, Channel: stream, Binding: binding, Receive: &transport.Receive{Timeout: 10 * time.Second}})

		if err != nil || err2 != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "one" {
			t.Fatalf("send %v receive %v received %v", err, err2, bodies(got.Received))
		}
	})

	t.Run("pub/sub: receive what another client publishes", func(t *testing.T) {
		channel := unique("flowi-it")
		done := make(chan error, 1)
		var got transport.Result
		go func() {
			var err error
			got, err = redis.New().Run(t.Context(), transport.Operation{
				Server: server, Channel: channel, Receive: &transport.Receive{Timeout: 15 * time.Second}})
			done <- err
		}()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) { // publish until the subscriber is there to hear it
			_, _ = redis.New().Run(t.Context(), transport.Operation{Server: server, Channel: channel, Send: &transport.Message{Body: []byte("hi")}})
			select {
			case err := <-done:
				if err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "hi" {
					t.Fatalf("err %v received %v", err, bodies(got.Received))
				}
				return
			case <-time.After(200 * time.Millisecond):
			}
		}
		t.Fatal("the subscriber heard nothing")
	})
}

func localstackBinding() map[string]string {
	return map[string]string{"auth": "explicit", "access_key_id": "test", "secret_access_key": "test", "region": "us-east-1", "wait": "2"}
}

func TestSNS_to_SQS_against_localstack(t *testing.T) {
	endpoint := address("FLOWI_TEST_LOCALSTACK", "http://127.0.0.1:4566")
	waitForPort(t, strings.Replace(endpoint, "http://", "tcp://", 1))
	cfg, err := config.LoadDefaultConfig(t.Context(), config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")))
	if err != nil {
		t.Fatal(err)
	}
	queues := sqs.NewFromConfig(cfg, func(o *sqs.Options) { o.BaseEndpoint = aws.String(endpoint) })
	topics := sns.NewFromConfig(cfg, func(o *sns.Options) { o.BaseEndpoint = aws.String(endpoint) })
	name := unique("flowi-it")
	retry(t, func() error {
		_, err := queues.CreateQueue(t.Context(), &sqs.CreateQueueInput{QueueName: aws.String(name)})
		return err
	})
	attrs, err := queues.GetQueueAttributes(t.Context(), &sqs.GetQueueAttributesInput{
		QueueUrl: aws.String(endpoint + "/000000000000/" + name), AttributeNames: []sqstypes.QueueAttributeName{sqstypes.QueueAttributeNameQueueArn}})
	if err != nil {
		t.Fatal(err)
	}
	topic, err := topics.CreateTopic(t.Context(), &sns.CreateTopicInput{Name: aws.String(name)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := topics.Subscribe(t.Context(), &sns.SubscribeInput{
		TopicArn: topic.TopicArn, Protocol: aws.String("sqs"), Endpoint: aws.String(attrs.Attributes["QueueArn"]),
		Attributes: map[string]string{"RawMessageDelivery": "true"}}); err != nil {
		t.Fatal(err)
	}

	_, pubErr := snstransport.New().Run(t.Context(), transport.Operation{
		Server: endpoint, Channel: name, Binding: localstackBinding(), Send: &transport.Message{Body: []byte("news")}})
	got, err := sqstransport.New().Run(t.Context(), transport.Operation{
		Server: endpoint, Channel: name, Binding: localstackBinding(), Receive: &transport.Receive{Timeout: 15 * time.Second}})

	if pubErr != nil || err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "news" {
		t.Fatalf("publish %v receive %v received %v", pubErr, err, bodies(got.Received))
	}
}

func TestPubSub_against_the_emulator(t *testing.T) {
	host := address("FLOWI_TEST_PUBSUB", "127.0.0.1:8085")
	waitForPort(t, "tcp://"+host)
	const project = "flowi-it"
	client, err := pubsub.NewClient(t.Context(), project, option.WithEndpoint(host), option.WithoutAuthentication(),
		option.WithGRPCDialOption(grpc.WithTransportCredentials(insecure.NewCredentials())))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Close() })
	name := unique("flowi-it")
	topicName := "projects/" + project + "/topics/" + name
	if _, err := client.TopicAdminClient.CreateTopic(t.Context(), &pubsubpb.Topic{Name: topicName}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.SubscriptionAdminClient.CreateSubscription(t.Context(), &pubsubpb.Subscription{
		Name: "projects/" + project + "/subscriptions/" + name, Topic: topicName, AckDeadlineSeconds: 10}); err != nil {
		t.Fatal(err)
	}
	binding := map[string]string{"endpoint": host}

	_, pubErr := pubsubtransport.New().Run(t.Context(), transport.Operation{
		Server: "pubsub://" + project, Channel: name, Binding: binding, Send: &transport.Message{Body: []byte("event")}})
	got, err := pubsubtransport.New().Run(t.Context(), transport.Operation{
		Server: "pubsub://" + project, Channel: name, Binding: binding, Receive: &transport.Receive{Timeout: 15 * time.Second}})

	if pubErr != nil || err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "event" {
		t.Fatalf("publish %v receive %v received %v", pubErr, err, bodies(got.Received))
	}
}

func TestServiceBus_against_the_emulator(t *testing.T) {
	connection := address("FLOWI_TEST_SERVICEBUS", "")
	if connection == "" {
		t.Skip("set FLOWI_TEST_SERVICEBUS to the emulator's connection string (see the servicebus profile of the compose file)")
	}
	binding := map[string]string{"auth": "explicit", "connection_string": connection}

	_, sendErr := servicebus.New().Run(t.Context(), transport.Operation{
		Channel: "flowi-it", Binding: binding, Send: &transport.Message{Body: []byte("one")}})
	got, err := servicebus.New().Run(t.Context(), transport.Operation{
		Channel: "flowi-it", Binding: binding, Receive: &transport.Receive{Timeout: 20 * time.Second}})

	if sendErr != nil || err != nil || len(got.Received) != 1 || string(got.Received[0].Body) != "one" {
		t.Fatalf("send %v receive %v received %v", sendErr, err, bodies(got.Received))
	}
}
