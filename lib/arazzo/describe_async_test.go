// SPDX-License-Identifier: Apache-2.0

package arazzo

import (
	"strings"
	"testing"

	"github.com/goccy/go-yaml"

	"github.com/githugt/flowi/lib/model"
)

type fakeMessages struct {
	request, message map[string]any
}

func (f fakeMessages) RequestSchema(model.Request, string) (map[string]any, bool) {
	return f.request, f.request != nil
}

func (f fakeMessages) MessageSchema(model.Request, string) (map[string]any, bool) {
	return f.message, f.message != nil
}

func asyncCollection() model.Collection {
	asyncReq := func(id, path string, a model.Async) model.Request {
		return model.Request{Meta: model.Meta{ID: model.ID(id), Name: path}, Path: path, Versions: []model.RequestVersion{{Label: "1", Async: &a}}}
	}
	send := &model.AsyncSend{Body: model.Body{JSON: `{"id":1}`}}
	recv := func() *model.AsyncReceive { return &model.AsyncReceive{Count: 2, Timeout: 5e9} }
	return model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Shop"}}, Requests: []model.Request{
		asyncReq("01JA00000000000000000000A1", "orders/placed", model.Async{Protocol: "mqtt", Server: "{{env.broker}}", Channel: "orders/placed", Send: send,
			Binding: []model.KeyValue{{Key: "qos", Value: "1"}, {Key: "password", Value: "{{secret.mqtt_pw}}"}, {Key: "username", Value: "u"}}}),
		asyncReq("01JA00000000000000000000A2", "orders/feed", model.Async{Protocol: "kafka", Server: "kafkas://k1:9093,k2:9093", Channel: "orders", Receive: recv(),
			Binding: []model.KeyValue{{Key: "group", Value: "g"}, {Key: "token", Value: "x"}}}),
		asyncReq("01JA00000000000000000000A3", "rpc/ask", model.Async{Protocol: "ws", Server: "wss://api.example.com", Channel: "rpc/{{kind}}", Send: send, Receive: recv()}),
		{Meta: model.Meta{ID: "01JA00000000000000000000H1", Name: "http"}, Path: "http/get", Versions: []model.RequestVersion{{Label: "1", Method: "GET", URL: "http://x/y"}}},
	}}
}

func describeAsync(t *testing.T, c model.Collection, s AsyncSchemaLookup) (string, map[string]any) {
	t.Helper()
	data, err := DescribeAsync(c, s)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return string(data), doc
}

func TestDescribeAsync_should_declare_servers_channels_and_one_operation_per_version(t *testing.T) {
	text, doc := describeAsync(t, asyncCollection(), nil)

	if doc["asyncapi"] != "3.0.0" {
		t.Fatalf("asyncapi %v", doc["asyncapi"])
	}
	ops := doc["operations"].(map[string]any)
	if len(ops) != 3 {
		t.Fatalf("operations %v (the HTTP request must not appear)", ops)
	}
	placed := ops["orders_placed"].(map[string]any)
	feed := ops["orders_feed"].(map[string]any)
	ask := ops["rpc_ask"].(map[string]any)
	if placed["action"] != "send" || feed["action"] != "receive" || ask["action"] != "send" || ask["reply"] == nil {
		t.Fatalf("actions: %v %v %v", placed["action"], feed["action"], ask["action"])
	}
	if placed["x-flowi-request"] != "01JA00000000000000000000A1" {
		t.Fatalf("x-flowi-request %v", placed["x-flowi-request"])
	}
	channels := doc["channels"].(map[string]any)
	if got := channels["rpc_kind"].(map[string]any)["address"]; got != "rpc/{kind}" {
		t.Fatalf("a templated channel becomes a parameter: %v", got)
	}
	servers := doc["servers"].(map[string]any)
	if len(servers) != 3 {
		t.Fatalf("servers %v", servers)
	}
	for _, want := range []string{"protocol: kafka-secure", "host: k1:9093,k2:9093", "protocol: wss", "host: api.example.com", "{broker}"} {
		if !strings.Contains(text, want) {
			t.Errorf("description lacks %q:\n%s", want, text)
		}
	}
}

func TestDescribeAsync_should_never_write_credentials(t *testing.T) {
	text, _ := describeAsync(t, asyncCollection(), nil)

	for _, leaked := range []string{"mqtt_pw", "secret.", "password", "token", ": u\n"} {
		if strings.Contains(text, leaked) {
			t.Errorf("description leaks %q:\n%s", leaked, text)
		}
	}
	if !strings.Contains(text, "qos") || !strings.Contains(text, "group") {
		t.Errorf("non-secret binding options are kept:\n%s", text)
	}
}

func TestDescribeAsync_should_use_the_learned_schemas_as_payloads(t *testing.T) {
	schemas := fakeMessages{
		request: map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{"type": "integer"}}},
		message: map[string]any{"type": "object", "properties": map[string]any{"ok": map[string]any{"type": "boolean"}}},
	}

	text, _ := describeAsync(t, asyncCollection(), schemas)

	if !strings.Contains(text, "integer") || !strings.Contains(text, "boolean") {
		t.Fatalf("payload schemas missing:\n%s", text)
	}
}

func TestDescribeAsync_should_be_byte_stable(t *testing.T) {
	a, _ := describeAsync(t, asyncCollection(), nil)
	b, _ := describeAsync(t, asyncCollection(), nil)

	if a != b {
		t.Fatal("two runs differ")
	}
}

func TestDescribeAsync_should_be_empty_when_there_is_no_async_request(t *testing.T) {
	data, err := DescribeAsync(describedCollection(), nil)

	if err != nil || data != nil {
		t.Fatalf("data %q err %v", data, err)
	}
}

func TestDescribe_should_leave_async_versions_out_of_the_openapi_description(t *testing.T) {
	text, _ := describe(t, asyncCollection(), nil)

	if strings.Contains(text, "orders_placed") || strings.Contains(text, "rpc_ask") {
		t.Fatalf("async operations in the OpenAPI description:\n%s", text)
	}
	if !strings.Contains(text, "http_get") {
		t.Fatalf("the HTTP request is gone:\n%s", text)
	}
}

func TestSecretBindingName_should_cover_the_credentials_of_every_protocol(t *testing.T) {
	for _, name := range []string{"username", "password", "token", "access_key_id", "secret_access_key", "session_token", "connection_string", "credentials_json"} {
		if !secretBindingName(name) {
			t.Errorf("%s is a credential but would be written to the description", name)
		}
	}
	for _, name := range []string{"region", "qos", "auth", "endpoint"} {
		if secretBindingName(name) {
			t.Errorf("%s is not a credential", name)
		}
	}
}

func managedCollection() model.Collection {
	req := func(id, path string, a model.Async) model.Request {
		return model.Request{Meta: model.Meta{ID: model.ID(id), Name: path}, Path: path, Versions: []model.RequestVersion{{Label: "1", Async: &a}}}
	}
	send := &model.AsyncSend{Body: model.Body{JSON: `{"id":1}`}}
	recv := &model.AsyncReceive{Count: 1, Timeout: 5e9}
	creds := func(kv ...model.KeyValue) []model.KeyValue {
		return append([]model.KeyValue{{Key: "auth", Value: "explicit"}}, kv...)
	}
	return model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Cloud"}}, Requests: []model.Request{
		req("01JA00000000000000000000B1", "queue/jobs", model.Async{Protocol: "sqs", Channel: "jobs", Receive: recv,
			Binding: creds(model.KeyValue{Key: "region", Value: "eu-west-1"}, model.KeyValue{Key: "access_key_id", Value: "AKIDLEAK"}, model.KeyValue{Key: "secret_access_key", Value: "SECRETLEAK"})}),
		req("01JA00000000000000000000B2", "topic/news", model.Async{Protocol: "sns", Server: "http://localhost:4566", Channel: "news", Send: send}),
		req("01JA00000000000000000000B3", "events/feed", model.Async{Protocol: "pubsub", Server: "pubsub://demo", Channel: "feed", Receive: recv,
			Binding: creds(model.KeyValue{Key: "credentials_json", Value: "JSONLEAK"})}),
		req("01JA00000000000000000000B4", "cache/events", model.Async{Protocol: "redis", Server: "redis://app:PWURL@cache:6379", Channel: "events", Send: send,
			Binding: []model.KeyValue{{Key: "password", Value: "PWLEAK"}}}),
		req("01JA00000000000000000000B5", "bus/orders", model.Async{Protocol: "servicebus", Server: "sb://myns.servicebus.windows.net", Channel: "orders", Send: send,
			Binding: creds(model.KeyValue{Key: "connection_string", Value: "CSLEAK"}, model.KeyValue{Key: "subject", Value: "New"})}),
	}}
}

func TestDescribeAsync_should_name_the_managed_protocols_and_never_write_their_credentials(t *testing.T) {
	text, doc := describeAsync(t, managedCollection(), nil)

	for _, want := range []string{"protocol: sqs", "protocol: sns", "protocol: googlepubsub", "protocol: redis", "protocol: amqps", "x-flowi-protocol: servicebus", "host: cache:6379", "host: myns.servicebus.windows.net", "host: demo", "region: eu-west-1", "subject: New"} {
		if !strings.Contains(text, want) {
			t.Errorf("description lacks %q:\n%s", want, text)
		}
	}
	for _, leaked := range []string{"AKIDLEAK", "SECRETLEAK", "JSONLEAK", "PWLEAK", "PWURL", "CSLEAK", "secret_access_key", "connection_string"} {
		if strings.Contains(text, leaked) {
			t.Errorf("description leaks %q:\n%s", leaked, text)
		}
	}
	if ops := doc["operations"].(map[string]any); len(ops) != 5 {
		t.Fatalf("operations %v, want one per request", ops)
	}
}

func TestDescribe_should_leave_grpc_versions_out_of_the_openapi_description(t *testing.T) {
	c := model.Collection{Manifest: model.Manifest{Meta: model.Meta{Name: "Shop"}}, Requests: []model.Request{
		{Meta: model.Meta{ID: "01JA00000000000000000000G1", Name: "get"}, Path: "orders/get",
			Versions: []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://h:1", Method: "a.B/C"}}}},
	}}

	data, err := Describe(c, nil)

	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "orders") {
		t.Fatalf("a gRPC call appears in the OpenAPI description:\n%s", data)
	}
}
