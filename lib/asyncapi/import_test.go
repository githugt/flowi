// SPDX-License-Identifier: Apache-2.0

package asyncapi

import (
	"os"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
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
