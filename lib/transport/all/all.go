// SPDX-License-Identifier: Apache-2.0

// Package all wires every protocol into one registry.
package all

import (
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/amqp"
	"github.com/githugt/flowi/lib/transport/amqp10"
	"github.com/githugt/flowi/lib/transport/kafka"
	"github.com/githugt/flowi/lib/transport/mqtt"
	"github.com/githugt/flowi/lib/transport/nats"
	"github.com/githugt/flowi/lib/transport/pubsub"
	"github.com/githugt/flowi/lib/transport/redis"
	"github.com/githugt/flowi/lib/transport/servicebus"
	"github.com/githugt/flowi/lib/transport/sns"
	"github.com/githugt/flowi/lib/transport/sqs"
	"github.com/githugt/flowi/lib/transport/sse"
	"github.com/githugt/flowi/lib/transport/ws"
)

// New returns a registry with every protocol this build speaks.
func New() transport.Registry {
	return transport.Registry{
		model.ProtocolWS:         ws.New(),
		model.ProtocolSSE:        sse.New(),
		model.ProtocolMQTT:       mqtt.New(),
		model.ProtocolKafka:      kafka.New(),
		model.ProtocolAMQP:       amqp.New(),
		model.ProtocolAMQP10:     amqp10.New(),
		model.ProtocolNATS:       nats.New(),
		model.ProtocolRedis:      redis.New(),
		model.ProtocolPubSub:     pubsub.New(),
		model.ProtocolSQS:        sqs.New(),
		model.ProtocolSNS:        sns.New(),
		model.ProtocolServiceBus: servicebus.New(),
	}
}
