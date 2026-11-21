// SPDX-License-Identifier: Apache-2.0

package model

// EndpointHint is what a message request's two address boxes say for a protocol: an example server, and what the
// second box is called and holds. Before, every protocol showed {{env.broker}} and "topic, subject, queue…", and for
// SSE the second box is an event-name filter, not a path.
type EndpointHint struct {
	// Server is an example of the first box.
	Server string
	// Channel is the second box's name: Topic, Subject, Queue, Event...
	Channel string
	// ChannelHint is an example or a short note for the second box's placeholder.
	ChannelHint string
}

// AsyncEndpointHint returns the hint for a message protocol. An unknown protocol gets the general one.
func AsyncEndpointHint(protocol string) EndpointHint {
	switch protocol {
	case ProtocolWS:
		return EndpointHint{"wss://host/socket", "Path", "chat (added to the address)"}
	case ProtocolSSE:
		return EndpointHint{"https://host/stream", "Event", "order.created (only this event name; empty for all)"}
	case ProtocolMQTT:
		return EndpointHint{"mqtt://host:1883", "Topic", "orders/created (+ and # match)"}
	case ProtocolKafka:
		return EndpointHint{"kafka://host:9092", "Topic", "orders"}
	case ProtocolAMQP:
		return EndpointHint{"amqp://user@host:5672", "Queue", "orders (or exchange/routing.key)"}
	case ProtocolAMQP10:
		return EndpointHint{"amqp://user@host:5672", "Address", "orders (a queue or topic address)"}
	case ProtocolNATS:
		return EndpointHint{"nats://host:4222", "Subject", "orders.new"}
	case ProtocolRedis:
		return EndpointHint{"redis://host:6379", "Channel", "orders"}
	case ProtocolSQS:
		return EndpointHint{"empty for AWS, or http://localhost:4566", "Queue", "jobs"}
	case ProtocolSNS:
		return EndpointHint{"empty for AWS, or http://localhost:4566", "Topic", "orders"}
	case ProtocolServiceBus:
		return EndpointHint{"sb://myns.servicebus.windows.net", "Queue or topic", "orders"}
	case ProtocolPubSub:
		return EndpointHint{"pubsub://project-id", "Topic", "orders"}
	}
	return EndpointHint{"{{env.broker}}", "Channel", "topic, subject, queue…"}
}

// DatasetServerExample returns an example of the server box for a dataset driver.
func DatasetServerExample(driver string) string {
	switch driver {
	case DriverPostgres:
		return "postgres://user:password@host:5432/database"
	case DriverMySQL:
		return "mysql://user:password@host:3306/database"
	case DriverMariaDB:
		return "mariadb://user:password@host:3306/database"
	}
	return "{{env.database_url}}"
}
