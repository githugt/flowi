// SPDX-License-Identifier: Apache-2.0

package asyncapi

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

// bindingSources are the `bindings` objects of the places an AsyncAPI document
// can put a protocol's options, keyed by protocol name: the server, the
// channel, the operation and the message.
type bindingSources struct {
	server, channel, operation, message map[string]any
}

// bindingsOf reads the bindings of an operation's protocol the way Flowi
// understands them. It returns the bindings in file order, the channel when a
// binding names a different one than the channel's address (a Kafka topic),
// and what it could not map. A protocol's binding fields Flowi has no
// equivalent for are reported, not guessed; the bindings of other protocols
// are ignored.
func bindingsOf(protocol string, src bindingSources) (binding []model.KeyValue, channel string, left []string) {
	switch protocol {
	case model.ProtocolMQTT:
		binding, left = mqttBindings(src)
	case model.ProtocolKafka:
		binding, channel, left = kafkaBindings(src)
	case model.ProtocolAMQP:
		binding, left = amqpBindings(src)
	case model.ProtocolNATS:
		binding, left = natsBindings(src)
	}
	return binding, channel, left
}

// protocolBinding is the object of one protocol inside a `bindings` map.
func protocolBinding(bindings map[string]any, name string) map[string]any {
	m, _ := bindings[name].(map[string]any)
	return m
}

// scalar is the one value a binding's schema allows: a plain value, or the
// const, the only enum value or the default of a schema. It is "" when the
// schema leaves the value open.
func scalar(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int, int64, uint64:
		return fmt.Sprint(x)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case map[string]any:
		if c, ok := x["const"]; ok {
			return scalar(c)
		}
		if enum, _ := x["enum"].([]any); len(enum) == 1 {
			return scalar(enum[0])
		}
		if d, ok := x["default"]; ok {
			return scalar(d)
		}
	}
	return ""
}

// unread lists the fields of a protocol binding that were not used, as
// "mqtt.lastWill", leaving out the version of the binding.
func unread(protocol string, b map[string]any, used ...string) []string {
	var out []string
	for _, k := range slices.Sorted(maps.Keys(b)) {
		if k != "bindingVersion" && !slices.Contains(used, k) {
			out = append(out, protocol+"."+k)
		}
	}
	return out
}

func add(binding []model.KeyValue, name, value string) []model.KeyValue {
	if value == "" || strings.Contains(value, "{") {
		return binding
	}
	return append(binding, model.KeyValue{Key: name, Value: value})
}

func mqttBindings(src bindingSources) (binding []model.KeyValue, left []string) {
	server, op := protocolBinding(src.server, "mqtt"), protocolBinding(src.operation, "mqtt")
	binding = add(binding, "qos", scalar(op["qos"]))
	binding = add(binding, "retain", scalar(op["retain"]))
	binding = add(binding, "client_id", scalar(server["clientId"]))
	left = append(left, unread("mqtt", server, "clientId")...)
	left = append(left, unread("mqtt", op, "qos", "retain")...)
	left = append(left, unread("mqtt", protocolBinding(src.channel, "mqtt"))...)
	left = append(left, unread("mqtt", protocolBinding(src.message, "mqtt"))...)
	return binding, left
}

func kafkaBindings(src bindingSources) (binding []model.KeyValue, topic string, left []string) {
	ch, op, msg := protocolBinding(src.channel, "kafka"), protocolBinding(src.operation, "kafka"), protocolBinding(src.message, "kafka")
	topic = scalar(ch["topic"])
	binding = add(binding, "group", scalar(op["groupId"]))
	binding = add(binding, "key", scalar(msg["key"]))
	left = append(left, unread("kafka", ch, "topic")...)
	left = append(left, unread("kafka", op, "groupId")...)
	left = append(left, unread("kafka", msg, "key")...)
	left = append(left, unread("kafka", protocolBinding(src.server, "kafka"))...)
	return binding, strings.TrimSpace(topic), left
}

func amqpBindings(src bindingSources) (binding []model.KeyValue, left []string) {
	ch := protocolBinding(src.channel, "amqp")
	if exchange, _ := ch["exchange"].(map[string]any); exchange != nil {
		binding = add(binding, "exchange", scalar(exchange["name"]))
	}
	if queue, _ := ch["queue"].(map[string]any); queue != nil {
		binding = add(binding, "queue", scalar(queue["name"]))
	}
	left = append(left, unread("amqp", ch, "is", "exchange", "queue")...)
	left = append(left, unread("amqp", protocolBinding(src.operation, "amqp"))...)
	left = append(left, unread("amqp", protocolBinding(src.message, "amqp"))...)
	return binding, left
}

func natsBindings(src bindingSources) (binding []model.KeyValue, left []string) {
	op := protocolBinding(src.operation, "nats")
	binding = add(binding, "queue_group", scalar(op["queue"]))
	left = append(left, unread("nats", op, "queue")...)
	left = append(left, unread("nats", protocolBinding(src.channel, "nats"))...)
	return binding, left
}

// unmappedBindings reports the binding fields left out for one operation.
func unmappedBindings(operation string, left []string) diag.List {
	var out diag.List
	for _, name := range left {
		out = append(out, diag.New(diag.CodeImportUnmapped, diag.Location{}, map[string]string{
			"what": "binding " + name + " of operation " + operation, "how": "left out",
			"detail": "Flowi has no equivalent of this protocol binding field"}))
	}
	return out
}
