// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func TestAsync_should_report_what_is_wrong_with_a_version(t *testing.T) {
	send := &model.AsyncSend{}
	recv := &model.AsyncReceive{Count: 1}
	tests := []struct {
		name string
		a    model.Async
		want diag.Code // "" → no diagnostic
		path string
	}{
		{name: "valid", a: model.Async{Protocol: "mqtt", Server: "mqtt://h", Send: send}},
		{name: "templated_server_is_skipped", a: model.Async{Protocol: "kafka", Server: "{{env.k}}", Send: send}},
		{name: "unknown_protocol", a: model.Async{Protocol: "carrier-pigeon", Send: send}, want: diag.CodeAsyncProtocol, path: "$.versions.1.async.protocol"},
		{name: "neither_side", a: model.Async{Protocol: "ws"}, want: diag.CodeAsyncAction, path: "$.versions.1.async"},
		{name: "sse_cannot_send", a: model.Async{Protocol: "sse", Server: "http://h", Send: send}, want: diag.CodeAsyncAction, path: "$.versions.1.async.send"},
		{name: "multipart_body", a: model.Async{Protocol: "ws", Server: "ws://h", Send: &model.AsyncSend{Body: model.Body{Multipart: []model.Field{{Key: "a", Value: "b"}}}}}, want: diag.CodeAsyncAction, path: "$.versions.1.async.send.body"},
		{name: "bad_count", a: model.Async{Protocol: "ws", Receive: &model.AsyncReceive{Count: -1}}, want: diag.CodeAsyncAction, path: "$.versions.1.async.receive.count"},
		{name: "bad_timeout", a: model.Async{Protocol: "ws", Receive: &model.AsyncReceive{Timeout: -time.Second}}, want: diag.CodeAsyncAction, path: "$.versions.1.async.receive.timeout"},
		{name: "unknown_binding", a: model.Async{Protocol: "ws", Send: send, Binding: []model.KeyValue{{Key: "qos", Value: "1"}}}, want: diag.CodeAsyncBinding, path: "$.versions.1.async.binding.qos"},
		{name: "known_binding", a: model.Async{Protocol: "mqtt", Receive: recv, Binding: []model.KeyValue{{Key: "qos", Value: "1"}}}},
		{name: "nats_host_and_port", a: model.Async{Protocol: "nats", Server: "localhost:4222", Send: send}},
		{name: "kafka_host_and_port", a: model.Async{Protocol: "kafka", Server: "localhost:9092", Send: send}},
		{name: "mqtt_needs_a_scheme", a: model.Async{Protocol: "mqtt", Server: "localhost:1883", Send: send}, want: diag.CodeAsyncServer, path: "$.versions.1.async.server"},
		{name: "wrong_scheme", a: model.Async{Protocol: "nats", Server: "http://h", Send: send}, want: diag.CodeAsyncServer, path: "$.versions.1.async.server"},
		{name: "sqs_without_server", a: model.Async{Protocol: "sqs", Channel: "q", Send: send}},
		{name: "sns_publish", a: model.Async{Protocol: "sns", Server: "http://localhost:4566", Channel: "t", Send: send}},
		{name: "sns_cannot_receive", a: model.Async{Protocol: "sns", Channel: "t", Receive: recv}, want: diag.CodeAsyncAction, path: "$.versions.1.async.receive"},
		{name: "redis_pubsub_cannot_send_and_receive", a: model.Async{Protocol: "redis", Server: "redis://h", Send: send, Receive: recv}, want: diag.CodeAsyncAction, path: "$.versions.1.async"},
		{name: "redis_stream_can_send_and_receive", a: model.Async{Protocol: "redis", Server: "redis://h", Send: send, Receive: recv, Binding: []model.KeyValue{{Key: "mode", Value: "stream"}}}},
		{name: "redis_wrong_scheme", a: model.Async{Protocol: "redis", Server: "http://h", Send: send}, want: diag.CodeAsyncServer, path: "$.versions.1.async.server"},
		{name: "servicebus_namespace_without_scheme", a: model.Async{Protocol: "servicebus", Server: "myns.servicebus.windows.net", Send: send}},
		{name: "servicebus_sb_scheme", a: model.Async{Protocol: "servicebus", Server: "sb://myns.servicebus.windows.net", Send: send}},
		{name: "pubsub_project", a: model.Async{Protocol: "pubsub", Server: "pubsub://my-project", Send: send}},
		{name: "auth_explicit", a: model.Async{Protocol: "sqs", Send: send, Binding: []model.KeyValue{{Key: "auth", Value: "explicit"}, {Key: "access_key_id", Value: "{{secret.id}}"}}}},
		{name: "auth_templated", a: model.Async{Protocol: "sqs", Send: send, Binding: []model.KeyValue{{Key: "auth", Value: "{{var.mode}}"}}}},
		{name: "auth_unknown_mode", a: model.Async{Protocol: "sqs", Send: send, Binding: []model.KeyValue{{Key: "auth", Value: "maybe"}}}, want: diag.CodeAsyncAction, path: "$.versions.1.async.binding.auth"},
		{name: "auth_not_for_open_protocols", a: model.Async{Protocol: "mqtt", Server: "mqtt://h", Send: send, Binding: []model.KeyValue{{Key: "auth", Value: "default"}}}, want: diag.CodeAsyncBinding, path: "$.versions.1.async.binding.auth"},
		{name: "jetstream_bindings", a: model.Async{Protocol: "nats", Server: "nats://h", Receive: recv, Binding: []model.KeyValue{{Key: "stream", Value: "S"}, {Key: "deliver", Value: "all"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Requests = append(c.Requests, model.Request{
				Meta: model.Meta{ID: "01J9ZA3M4N5P6Q7R8S9T0VAAAA", Name: "r"}, Path: "r", File: "r.req.yaml",
				Versions: []model.RequestVersion{{Label: "1", Async: &tt.a}},
			})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want == "" {
				for _, d := range got {
					if len(d.Code) > 9 && d.Code[:9] == "FLW-ASYNC" {
						t.Fatalf("unexpected %v", d)
					}
				}
				return
			}
			if d := only(t, got, tt.want); d.Path != tt.path {
				t.Fatalf("path %q, want %q", d.Path, tt.path)
			}
		})
	}
}

func TestAsync_should_check_the_templates_and_expressions_of_a_version(t *testing.T) {
	good := model.Async{Protocol: "ws", Server: "ws://{{env.host}}", Channel: "c/{{id}}", Send: &model.AsyncSend{Body: model.Body{JSON: `{"a":"{{id}}"}`}},
		Receive: &model.AsyncReceive{Match: "body.ok and received >= 0"}}
	bad := good
	bad.Channel = "c/{{"

	for name, tt := range map[string]struct {
		a        model.Async
		assert   string
		wantCode diag.Code
	}{
		"good":       {a: good, assert: "received == 1 and body.ok"},
		"bad_syntax": {a: bad, assert: "received == 1", wantCode: diag.CodeTemplateSyntax},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			a := tt.a
			c.Requests = append(c.Requests, model.Request{
				Meta: model.Meta{ID: "01J9ZA3M4N5P6Q7R8S9T0VAAAA", Name: "r"}, Path: "r", File: "r.req.yaml",
				Versions: []model.RequestVersion{{Label: "1", Async: &a, Assert: []string{tt.assert}}},
			})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.wantCode == "" {
				if len(got) != 0 {
					t.Fatalf("unexpected %v", got)
				}
				return
			}
			only(t, got, tt.wantCode)
		})
	}
}

func grpcVersion(g model.GRPC, async *model.Async) model.RequestVersion {
	return model.RequestVersion{Label: "1", GRPC: &g, Async: async}
}

func TestGRPC_should_report_what_is_wrong_with_a_version(t *testing.T) {
	ok := model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Body: model.Body{JSON: `{}`}}
	tests := []struct {
		name  string
		v     model.RequestVersion
		want  diag.Code // "" → no diagnostic
		path  string
		other model.GRPC
	}{
		{name: "valid", v: grpcVersion(ok, nil)},
		{name: "templated_server_and_method", v: grpcVersion(model.GRPC{Server: "{{env.api}}", Method: "{{var.m}}"}, nil)},
		{name: "bare_host_and_port", v: grpcVersion(model.GRPC{Server: "localhost:50051", Method: "a.B/C"}, nil)},
		{name: "both_grpc_and_async", v: grpcVersion(ok, &model.Async{Protocol: "ws"}), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc"},
		{name: "bad_method", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "NoSlash"}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.method"},
		{name: "bad_scheme", v: grpcVersion(model.GRPC{Server: "http://h:1", Method: "a.B/C"}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.server"},
		{name: "web_over_http", v: grpcVersion(model.GRPC{Server: "https://proxy.example/api", Method: "a.B/C", Proto: []string{"p.proto"}, Web: true}, nil)},
		{name: "web_without_proto_files", v: grpcVersion(model.GRPC{Server: "https://h", Method: "a.B/C", Web: true}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.proto"},
		{name: "web_over_grpc_scheme", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Proto: []string{"p.proto"}, Web: true}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.server"},
		{name: "negative_count", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Receive: &model.AsyncReceive{Count: -1}}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.receive.count"},
		{name: "negative_timeout", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Receive: &model.AsyncReceive{Timeout: -time.Second}}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.receive.timeout"},
		{name: "multipart_body", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Body: model.Body{Multipart: []model.Field{{Key: "a", Value: "b"}}}}, nil), want: diag.CodeGRPCRequest, path: "$.versions.1.grpc.body"},
		{name: "proto_inside_the_collection", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Proto: []string{"protos/a.proto"}, Import: []string{"protos"}}, nil)},
		{name: "proto_outside_the_collection", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Proto: []string{"../a.proto"}}, nil), want: diag.CodeGRPCProto, path: "$.versions.1.grpc.proto[0]"},
		{name: "absolute_proto", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Proto: []string{"/etc/a.proto"}}, nil), want: diag.CodeGRPCProto, path: "$.versions.1.grpc.proto[0]"},
		{name: "import_outside_the_collection", v: grpcVersion(model.GRPC{Server: "grpc://h:1", Method: "a.B/C", Import: []string{"../x"}}, nil), want: diag.CodeGRPCProto, path: "$.versions.1.grpc.import[0]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixture(t)
			c := f.collection("c")
			c.Requests = append(c.Requests, model.Request{
				Meta: model.Meta{ID: "01J9ZA3M4N5P6Q7R8S9T0VAAAA", Name: "r"}, Path: "r", File: "r.req.yaml", Versions: []model.RequestVersion{tt.v},
			})

			got := Validate(Target{Collections: []model.Collection{c}})

			if tt.want == "" {
				for _, d := range got { // a gRPC version has no URL or method to complain about either
					t.Fatalf("unexpected %v", d)
				}
				return
			}
			if d := only(t, got, tt.want); d.Path != tt.path {
				t.Fatalf("path %q, want %q", d.Path, tt.path)
			}
		})
	}
}

func TestValidate_should_report_misuse_of_connections_kept_open_between_steps(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("c")
	chat := f.request(&c, "chat", "1")
	c.Requests[0].Versions[0].Async = &model.Async{Protocol: "ws", Server: "ws://h", Receive: &model.AsyncReceive{Count: 1}}
	plain := f.request(&c, "plain", "1")
	both, closeOnly, onHTTP, orphan, fine := requestNode("both", chat), requestNode("closeOnly", chat), requestNode("onHTTP", plain), requestNode("orphan", chat), requestNode("fine", chat)
	both.Request.Open, both.Request.Over = "a", "a"
	closeOnly.Request.Close = true
	onHTTP.Request.Open = "h"
	orphan.Request.Over = "nobody"
	fine.Request.Over = "a"
	f.flow(&c, "x", both, closeOnly, onHTTP, orphan, fine)

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	paths := map[string]diag.Code{}
	for _, d := range got {
		if d.Code == diag.CodeAsyncConn || d.Code == diag.CodeAsyncConnName {
			paths[d.Path] = d.Code
		}
	}
	want := map[string]diag.Code{
		"$.nodes[0].over":  diag.CodeAsyncConn,
		"$.nodes[1].close": diag.CodeAsyncConn,
		"$.nodes[2].open":  diag.CodeAsyncConn,
		"$.nodes[3].over":  diag.CodeAsyncConnName,
	}
	for p, code := range want {
		if paths[p] != code {
			t.Errorf("missing %s at %s; got %v", code, p, paths)
		}
	}
	if len(paths) != len(want) {
		t.Errorf("a step over a connection that another step of the flow opens must pass: %v", paths)
	}
}
