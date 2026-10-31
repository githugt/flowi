// SPDX-License-Identifier: Apache-2.0

package asyncapi

import (
	"os"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func newID() func() model.ID {
	n := 0
	return func() model.ID { n++; return model.ID(strings.Repeat("0", 25) + string(rune('A'+n))) }
}

func load(t *testing.T, name string) Result {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	res, diags := Import(data, Options{NewID: newID()})
	if diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
	return res
}

func byOperation(res Result, op string) model.Request {
	for _, r := range res.Requests {
		if r.Operation == op {
			return r
		}
	}
	return model.Request{}
}

func TestImport_v3_should_map_operations_to_async_requests(t *testing.T) {
	res := load(t, "streetlights3.yaml")

	recv := byOperation(res, "receiveLightMeasurement").Versions[0].Async
	send := byOperation(res, "turnOn").Versions[0].Async

	if len(res.Requests) != 2 || recv == nil || send == nil {
		t.Fatalf("requests %+v", res.Requests)
	}
	if recv.Protocol != "mqtt" || recv.Server != "mqtt://{{env.broker}}:1883" || recv.Receive == nil || recv.Send != nil {
		t.Fatalf("receive %+v", recv)
	}
	if recv.Channel != "smartylighting/streetlights/1/0/event/{{streetlightId}}/lighting/measured" {
		t.Fatalf("channel %q", recv.Channel)
	}
	if send.Send == nil || !strings.Contains(send.Send.Body.JSON, `"command": ""`) || !strings.Contains(send.Send.Body.JSON, `"level": 0`) {
		t.Fatalf("send %+v", send)
	}
	if send.Server != "mqtt://{{env.broker}}:1883" && send.Server != "mqtts://mqtt.example.com" {
		t.Fatalf("server %q", send.Server)
	}
}

func TestImport_v2_should_read_publish_as_send_and_subscribe_as_receive(t *testing.T) {
	res := load(t, "chat2.yaml")

	send := byOperation(res, "sendMessage").Versions[0].Async
	recv := byOperation(res, "onMessage").Versions[0].Async

	if send.Send == nil || send.Protocol != "ws" || send.Server != "wss://chat.example.com/socket" || recv.Receive == nil {
		t.Fatalf("send %+v receive %+v", send, recv)
	}
}

func TestImport_should_report_an_unsupported_protocol_and_not_a_non_asyncapi_document(t *testing.T) {
	doc := "asyncapi: 3.0.0\nservers: {s: {host: h, protocol: stomp}}\nchannels: {c: {address: a}}\noperations: {o: {action: send, channel: {$ref: '#/channels/c'}}}\n"

	res, diags := Import([]byte(doc), Options{NewID: newID()})
	_, bad := Import([]byte("openapi: 3.0.0\n"), Options{NewID: newID()})

	if len(res.Requests) != 0 || len(diags) != 1 || diags[0].Code != diag.CodeImportUnmapped {
		t.Fatalf("requests %d diags %v", len(res.Requests), diags)
	}
	if !bad.HasErrors() || bad[0].Code != diag.CodeImportInvalid {
		t.Fatalf("diags %v", bad)
	}
}

func bindingOf(a *model.Async, name string) string {
	for _, kv := range a.Binding {
		if kv.Key == name {
			return kv.Value
		}
	}
	return ""
}

func TestImport_should_map_the_protocol_bindings_it_understands_and_report_the_rest(t *testing.T) {
	doc := `asyncapi: 3.0.0
info: {title: t, version: 1.0.0}
servers:
  broker:
    host: broker:1883
    protocol: mqtt
    bindings:
      mqtt: {clientId: flowi-client, keepAlive: 60}
channels:
  readings:
    address: sensors/{id}/readings
    bindings:
      mqtt: {}
operations:
  publishReading:
    action: send
    channel: {$ref: '#/channels/readings'}
    bindings:
      mqtt: {qos: 1, retain: true, bindingVersion: 0.2.0}
`
	kafka := `asyncapi: 3.0.0
info: {title: t, version: 1.0.0}
servers: {k: {host: 'kafka:9092', protocol: kafka}}
channels:
  orders:
    address: orders-address
    bindings:
      kafka: {topic: orders.v1, partitions: 3}
    messages:
      order:
        payload: {type: object}
        bindings:
          kafka: {key: {type: string, enum: [order-key]}}
operations:
  readOrders:
    action: receive
    channel: {$ref: '#/channels/orders'}
    bindings:
      kafka: {groupId: {type: string, enum: [billing]}, clientId: {type: string}}
`
	amqp := `asyncapi: 2.6.0
info: {title: t, version: 1.0.0}
servers: {a: {url: 'amqp://rabbit:5672', protocol: amqp}}
channels:
  user.signedup:
    bindings:
      amqp:
        is: routingKey
        exchange: {name: events, type: topic}
    publish:
      operationId: announce
      bindings:
        amqp: {deliveryMode: 2}
      message: {payload: {type: object}}
`
	nats := `asyncapi: 2.6.0
info: {title: t, version: 1.0.0}
servers: {n: {url: 'nats://nats:4222', protocol: nats}}
channels:
  jobs:
    subscribe:
      operationId: work
      bindings:
        nats: {queue: workers}
      message: {payload: {type: object}}
`

	m, mDiags := Import([]byte(doc), Options{NewID: newID()})
	k, kDiags := Import([]byte(kafka), Options{NewID: newID()})
	a, aDiags := Import([]byte(amqp), Options{NewID: newID()})
	n, nDiags := Import([]byte(nats), Options{NewID: newID()})

	mq := m.Requests[0].Versions[0].Async
	if bindingOf(mq, "qos") != "1" || bindingOf(mq, "retain") != "true" || bindingOf(mq, "client_id") != "flowi-client" {
		t.Errorf("mqtt binding %+v", mq.Binding)
	}
	if len(mDiags) != 1 || !strings.Contains(mDiags[0].Args["what"], "mqtt.keepAlive") {
		t.Errorf("mqtt: want the keepAlive reported, got %v", mDiags)
	}
	kf := k.Requests[0].Versions[0].Async
	if kf.Channel != "orders.v1" || bindingOf(kf, "group") != "billing" || bindingOf(kf, "key") != "order-key" {
		t.Errorf("kafka channel %q binding %+v", kf.Channel, kf.Binding)
	}
	if len(kDiags) != 2 {
		t.Errorf("kafka: want partitions and clientId reported, got %v", kDiags)
	}
	am := a.Requests[0].Versions[0].Async
	if bindingOf(am, "exchange") != "events" || len(aDiags) != 1 {
		t.Errorf("amqp binding %+v diags %v", am.Binding, aDiags)
	}
	nt := n.Requests[0].Versions[0].Async
	if bindingOf(nt, "queue_group") != "workers" || len(nDiags) != 0 {
		t.Errorf("nats binding %+v diags %v", nt.Binding, nDiags)
	}
}

func TestImport_should_ignore_the_bindings_of_other_protocols(t *testing.T) {
	doc := `asyncapi: 3.0.0
info: {title: t, version: 1.0.0}
servers: {n: {host: 'nats:4222', protocol: nats}}
channels:
  jobs:
    address: jobs
    bindings:
      kafka: {topic: elsewhere}
operations:
  work:
    action: receive
    channel: {$ref: '#/channels/jobs'}
`

	res, diags := Import([]byte(doc), Options{NewID: newID()})

	if len(diags) != 0 || res.Requests[0].Versions[0].Async.Channel != "jobs" {
		t.Fatalf("channel %q diags %v", res.Requests[0].Versions[0].Async.Channel, diags)
	}
}

// AsyncAPI 3.1.0 only adds a ROS 2 binding to 3.0.0: a 3.1 document with an operation must import like a 3.0 one, and the
// ROS 2 binding on a channel (a protocol Flowi has no transport for) must not stop it or change the request.
func TestImport_Given_an_asyncapi_3_1_document_Then_it_imports_like_3_0(t *testing.T) {
	// Arrange
	doc := `asyncapi: 3.1.0
info: {title: t, version: 1.0.0}
servers:
  broker: {host: "broker:1883", protocol: mqtt}
channels:
  readings:
    address: sensors/readings
    bindings:
      ros2: {node: sensor_node}
operations:
  publishReading:
    action: send
    channel: {$ref: '#/channels/readings'}
`

	// Act
	got, diags := Import([]byte(doc), Options{NewID: func() model.ID { return "ID000000000000000000000001" }})

	// Assert
	if diags.HasErrors() || len(got.Requests) != 1 || got.Requests[0].Versions[0].Async == nil || got.Requests[0].Versions[0].Async.Protocol != "mqtt" {
		t.Fatalf("diags %v requests %+v", diags, got.Requests)
	}
}
