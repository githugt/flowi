// SPDX-License-Identifier: Apache-2.0

package model

import (
	"slices"
	"strings"
)

// The async protocols a request can speak.
const (
	ProtocolWS    = "ws"
	ProtocolSSE   = "sse"
	ProtocolMQTT  = "mqtt"
	ProtocolKafka = "kafka"
	ProtocolAMQP  = "amqp" // AMQP 0-9-1 (RabbitMQ and compatible)
	ProtocolNATS  = "nats"

	// ProtocolAMQP10 is AMQP 1.0 (ActiveMQ Artemis, Qpid, IBM MQ, RabbitMQ 4 ...), a different protocol from AMQP 0-9-1.
	ProtocolAMQP10 = "amqp10"

	// Managed messaging and Redis (ADR 0019).
	ProtocolRedis      = "redis"
	ProtocolSQS        = "sqs"
	ProtocolSNS        = "sns"
	ProtocolServiceBus = "servicebus"
	ProtocolPubSub     = "pubsub"
)

// Values of the `auth` binding of the cloud protocols.
const (
	AuthDefault  = "default"  // the provider's own credential lookup
	AuthExplicit = "explicit" // the credential bindings of the request
)

// asyncProtocol is what is fixed about one protocol: the URL schemes its
// server may use and the binding names it understands.
type asyncProtocol struct {
	schemes  []string
	bindings []string
	// credentials are the binding names that hold secrets; the GUI edits
	// them in the Authentication section and descriptions never write them.
	credentials []string
	cloud       bool // has the `auth` mode (default | explicit)
}

// Credential name sets shared by several protocols (functions, not
// variables, so nothing global can be changed).
func userPass() []string      { return []string{"username", "password"} }
func userPassToken() []string { return []string{"username", "password", "token"} }
func awsKeys() []string       { return []string{"access_key_id", "secret_access_key", "session_token"} }

// protocolOf returns what is fixed about protocol p (zero for an unknown one).
func protocolOf(p string) asyncProtocol {
	switch p {
	case ProtocolWS:
		return asyncProtocol{schemes: []string{"ws", "wss"}, bindings: []string{"subprotocol"}}
	case ProtocolSSE:
		return asyncProtocol{schemes: []string{"http", "https"}, bindings: []string{"last_event_id"}}
	case ProtocolMQTT:
		return asyncProtocol{schemes: []string{"mqtt", "mqtts", "ws", "wss"}, credentials: userPassToken(),
			bindings: []string{"version", "qos", "retain", "client_id", "username", "password"}}
	case ProtocolKafka:
		return asyncProtocol{schemes: []string{"kafka", "kafkas"}, credentials: userPassToken(),
			bindings: []string{"key", "partition", "group", "offset", "sasl.mechanism", "username", "password"}}
	case ProtocolAMQP:
		return asyncProtocol{schemes: []string{"amqp", "amqps"}, credentials: userPassToken(),
			bindings: []string{"exchange", "routing_key", "queue", "username", "password"}}
	case ProtocolAMQP10:
		return asyncProtocol{schemes: []string{"amqp", "amqps"}, credentials: userPass(),
			bindings: []string{"subject", "content_type", "durable", "ttl", "username", "password"}}
	case ProtocolNATS:
		return asyncProtocol{schemes: []string{"nats", "tls"}, credentials: userPassToken(), bindings: []string{
			"queue_group", "reply_to", "token", "username", "password", "stream", "consumer", "deliver", "ack"}}
	}
	return managedProtocolOf(p)
}

// managedProtocolOf is protocolOf for Redis and the managed messaging services.
func managedProtocolOf(p string) asyncProtocol {
	switch p {
	case ProtocolRedis:
		return asyncProtocol{schemes: []string{"redis", "rediss"}, credentials: userPass(),
			bindings: []string{"mode", "pattern", "start", "group", "consumer", "maxlen", "username", "password"}}
	case ProtocolSQS:
		return asyncProtocol{schemes: []string{"http", "https"}, credentials: awsKeys(), cloud: true, bindings: []string{
			"region", "profile", "auth", "group_id", "dedup_id", "delay", "visibility_timeout", "wait", "delete",
			"access_key_id", "secret_access_key", "session_token"}}
	case ProtocolSNS:
		return asyncProtocol{schemes: []string{"http", "https"}, credentials: awsKeys(), cloud: true, bindings: []string{
			"region", "profile", "auth", "subject", "group_id", "dedup_id",
			"access_key_id", "secret_access_key", "session_token"}}
	case ProtocolServiceBus:
		return asyncProtocol{schemes: []string{"sb"}, credentials: []string{"connection_string"}, cloud: true,
			bindings: []string{"auth", "read", "session_id", "subject", "message_id", "ttl", "connection_string"}}
	case ProtocolPubSub:
		return asyncProtocol{schemes: []string{"pubsub"}, credentials: []string{"credentials_json"}, cloud: true,
			bindings: []string{"endpoint", "auth", "ordering_key", "credentials_json"}}
	}
	return asyncProtocol{}
}

// AsyncProtocols lists the protocols in catalogue order.
func AsyncProtocols() []string {
	return []string{ProtocolWS, ProtocolSSE, ProtocolMQTT, ProtocolKafka, ProtocolAMQP, ProtocolAMQP10, ProtocolNATS,
		ProtocolRedis, ProtocolSQS, ProtocolSNS, ProtocolServiceBus, ProtocolPubSub}
}

// KnownProtocol reports whether p is an async protocol.
func KnownProtocol(p string) bool { return slices.Contains(AsyncProtocols(), p) }

// AsyncSchemes are the URL schemes a server of protocol p may use.
func AsyncSchemes(p string) []string { return protocolOf(p).schemes }

// AsyncBindings are the binding names protocol p understands.
func AsyncBindings(p string) []string { return protocolOf(p).bindings }

// AsyncCredentials are the binding names of protocol p that hold credentials.
func AsyncCredentials(p string) []string { return protocolOf(p).credentials }

// CloudProtocol reports whether p is a cloud service with an `auth` mode:
// the provider's default login, or the explicit credential bindings.
func CloudProtocol(p string) bool { return protocolOf(p).cloud }

// NormalizeServer writes out the scheme of a server that left it off: a NATS
// or Kafka server is often written as host:port, a Service Bus one as the
// namespace host. A templated server, one with
// a scheme and one of a protocol with several schemes are returned as given.
func NormalizeServer(protocol, server string) string {
	if server == "" || strings.Contains(server, "://") || strings.HasPrefix(strings.TrimSpace(server), "{{") {
		return server
	}
	switch protocol {
	case ProtocolNATS, ProtocolKafka:
		return protocol + "://" + server
	case ProtocolServiceBus:
		return "sb://" + server
	}
	return server
}

// JoinURL puts a channel after a server URL: "wss://h" and "events" give
// "wss://h/events"; an empty channel leaves the server alone.
func JoinURL(server, channel string) string {
	if server == "" { // SQS and SNS with no server reach AWS itself: the channel alone
		return channel
	}
	if channel == "" {
		return server
	}
	return strings.TrimRight(server, "/") + "/" + strings.TrimLeft(channel, "/")
}

// TargetURL is where an async request goes, for display and for the
// protocols that connect to a path: the server and channel joined. For SSE the
// channel names the event to keep, not a path, so the server alone is the URL.
func TargetURL(protocol, server, channel string) string {
	if protocol == ProtocolSSE {
		return server
	}
	return JoinURL(server, channel)
}

// RedactedTarget hides the password of user:password@ in a URL-like target,
// for anything recorded or shown. A user name alone is left as it is.
func RedactedTarget(target string) string {
	scheme, rest, found := strings.Cut(target, "://")
	if !found {
		return target
	}
	authority, path, hasPath := strings.Cut(rest, "/")
	at := strings.LastIndex(authority, "@")
	if at < 0 {
		return target
	}
	user, _, hasPassword := strings.Cut(authority[:at], ":")
	if !hasPassword {
		return target
	}
	out := scheme + "://" + user + ":***@" + authority[at+1:]
	if hasPath {
		out += "/" + path
	}
	return out
}

// ProtocolVerb is the short upper-case name a protocol shows where an HTTP
// request shows its method: MQTT, WS, … BUS for Service Bus and AMQP1 for AMQP 1.0.
func ProtocolVerb(protocol string) string {
	switch protocol {
	case ProtocolServiceBus:
		return "BUS"
	case ProtocolAMQP10:
		return "AMQP1"
	}
	return strings.ToUpper(protocol)
}

// ProtocolOfVerb is the protocol a verb stands for, or "" when it is not one
// (an HTTP method).
func ProtocolOfVerb(verb string) string {
	for _, p := range AsyncProtocols() {
		if strings.EqualFold(ProtocolVerb(p), verb) {
			return p
		}
	}
	return ""
}

// ProtocolVerbGRPC is what a gRPC request shows where an HTTP request shows
// its method.
const ProtocolVerbGRPC = "GRPC"
