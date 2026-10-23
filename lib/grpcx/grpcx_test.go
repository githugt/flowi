// SPDX-License-Identifier: Apache-2.0

package grpcx_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/transport"
)

func call(f *fake, method, body string) grpcx.Call {
	return grpcx.Call{Server: "grpc://" + f.Addr, Method: method, JSON: []byte(body), Timeout: 5 * time.Second}
}

// writeProtos writes the test protos under a collection directory: orders in
// protos/, its import next to it.
func writeProtos(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "protos"), 0o755); err != nil {
		t.Fatal(err)
	}
	for name, src := range map[string]string{"orders.proto": ordersProto, "common.proto": commonProto} {
		if err := os.WriteFile(filepath.Join(root, "protos", name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func withProtos(c grpcx.Call, root string) grpcx.Call {
	c.Root, c.Protos, c.Imports = root, []string{"protos/orders.proto"}, []string{"protos"}
	return c
}

func header(hs []transport.Header, name string) string {
	for _, h := range hs {
		if h.Name == name {
			return h.Value
		}
	}
	return ""
}

func TestRun_should_call_a_unary_method_through_reflection(t *testing.T) {
	f := startServer(t, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Get", `{"id": "7"}`))

	if err != nil {
		t.Fatal(err)
	}
	if res.Code != codes.OK || len(res.Messages) != 2 {
		t.Fatalf("code %v messages %d, want OK with the request and the response", res.Code, len(res.Messages))
	}
	sent, got := res.Messages[0], res.Messages[1]
	if sent.Direction != "sent" || got.Direction != "received" || !strings.Contains(string(got.Body), `"name":"order-7"`) {
		t.Fatalf("sent %s received %s", sent.Body, got.Body)
	}
}

func TestRun_should_fall_back_to_the_older_reflection_service(t *testing.T) {
	f := startServerWith(t, reflectV1Alpha, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Get", `{"id": "5"}`))

	if err != nil || res.Code != codes.OK || !strings.Contains(string(res.Messages[1].Body), `"id":"5"`) {
		t.Fatalf("err %v result %+v", err, res)
	}
}

func TestRun_should_call_a_method_from_proto_files_with_an_import_root(t *testing.T) {
	f := startServer(t, false, nil) // no reflection: the files are the only schema
	root := writeProtos(t)

	res, err := grpcx.Run(t.Context(), withProtos(call(f, "acme.orders.v1.Orders/Get", `{"id": "9"}`), root))

	if err != nil || res.Code != codes.OK || !strings.Contains(string(res.Messages[1].Body), `"id":"9"`) {
		t.Fatalf("err %v result %+v", err, res)
	}
}

func TestRun_should_report_a_non_ok_status_as_a_result_not_an_error(t *testing.T) {
	f := startServer(t, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Fail", `{"id": "1"}`))

	if err != nil {
		t.Fatalf("a status is not a Go error: %v", err)
	}
	if res.Code != codes.NotFound || res.Message != "no order 1" || len(res.Messages) != 1 {
		t.Fatalf("code %v message %q messages %d", res.Code, res.Message, len(res.Messages))
	}
}

func TestRun_should_send_metadata_and_return_the_response_header_and_trailer(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/Echo", `{"id": "1"}`)
	c.Metadata = []transport.Header{{Name: "X-Token", Value: "abc"}}

	res, err := grpcx.Run(t.Context(), c)

	if err != nil {
		t.Fatal(err)
	}
	if got := header(res.Header, "x-seen"); got != "[abc]" || header(res.Trailer, "x-trail") != "done" {
		t.Fatalf("header %+v trailer %+v", res.Header, res.Trailer)
	}
}

func TestRun_should_refuse_binary_metadata(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/Get", `{"id": "1"}`)
	c.Metadata = []transport.Header{{Name: "trace-bin", Value: "x"}}

	_, err := grpcx.Run(t.Context(), c)

	if err == nil || !strings.Contains(err.Error(), "-bin") {
		t.Fatalf("err %v, want one naming -bin metadata", err)
	}
}

func TestRun_should_report_deadline_exceeded_as_the_status(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/Slow", `{"id": "1"}`)
	c.Timeout = 300 * time.Millisecond

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || res.Code != codes.DeadlineExceeded {
		t.Fatalf("err %v code %v", err, res.Code)
	}
}

func TestRun_should_collect_a_server_stream_up_to_count(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/List", `{"count": 5}`)
	c.Receive = &transport.Receive{Count: 3, Timeout: 5 * time.Second}
	var live []string
	c.Receive.OnMessage = func(m transport.Message) { live = append(live, string(m.Body)) }

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || len(res.Messages) != 4 || len(live) != 3 { // the request and 3 responses
		t.Fatalf("err %v messages %d live %d", err, len(res.Messages), len(live))
	}
}

func TestRun_should_end_a_stream_with_no_receive_options_at_the_timeout_without_an_error(t *testing.T) {
	f := startServer(t, true, nil) // Trickle sends 2 messages and then holds the stream open, like a watch
	c := call(f, "acme.orders.v1.Orders/Trickle", `{"count": 2}`)
	c.Timeout = 400 * time.Millisecond

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || len(res.Messages) != 3 || res.Code != codes.OK {
		t.Fatalf("err %v messages %d code %v: with no count to wait for, the timeout just ends the stream", err, len(res.Messages), res.Code)
	}
}

func TestRun_should_keep_the_messages_received_when_the_context_is_cancelled(t *testing.T) {
	f := startServer(t, true, nil)
	ctx, cancel := context.WithCancel(t.Context())
	c := call(f, "acme.orders.v1.Orders/Trickle", `{"count": 2}`)
	c.Receive = &transport.Receive{Timeout: time.Minute}
	c.Receive.OnMessage = func(m transport.Message) {
		if strings.Contains(string(m.Body), `"seq":2`) {
			cancel() // the user stops the watch after the second message
		}
	}

	res, _ := grpcx.Run(ctx, c)

	if len(res.Messages) != 3 || res.Code != codes.Canceled {
		t.Fatalf("messages %d code %v, want the request and 2 messages kept, and the code Canceled", len(res.Messages), res.Code)
	}
}

func TestRun_should_collect_a_stream_until_it_ends_when_no_receive_options_are_given(t *testing.T) {
	f := startServer(t, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/List", `{"count": 4}`))

	if err != nil || len(res.Messages) != 5 || res.Code != codes.OK {
		t.Fatalf("err %v messages %d code %v, want the request and all 4 messages", err, len(res.Messages), res.Code)
	}
}

func TestRun_should_keep_only_the_messages_the_filter_accepts(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/List", `{"count": 6}`)
	c.Receive = &transport.Receive{Count: 2, Timeout: 5 * time.Second, Match: func(m transport.Message) (bool, error) {
		return strings.Contains(string(m.Body), `"seq":4`) || strings.Contains(string(m.Body), `"seq":6`), nil
	}}

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || len(res.Messages) != 3 || !strings.Contains(string(res.Messages[1].Body), `"seq":4`) || !strings.Contains(string(res.Messages[2].Body), `"seq":6`) {
		t.Fatalf("err %v messages %d", err, len(res.Messages))
	}
}

func TestRun_should_treat_a_stream_that_ends_early_as_done_not_as_a_timeout(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/List", `{"count": 2}`)
	c.Receive = &transport.Receive{Count: 5, Timeout: 5 * time.Second}

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || res.Code != codes.OK || len(res.Messages) != 3 {
		t.Fatalf("err %v code %v messages %d, want the 2 messages and no error: nothing more will come", err, res.Code, len(res.Messages))
	}
}

func TestRun_should_return_the_partial_stream_with_a_timeout(t *testing.T) {
	f := startServer(t, true, nil)
	c := call(f, "acme.orders.v1.Orders/Trickle", `{"count": 2}`)
	c.Receive = &transport.Receive{Count: 5, Timeout: 400 * time.Millisecond}

	res, err := grpcx.Run(t.Context(), c)

	if !errors.Is(err, transport.ErrTimeout) || len(res.Messages) != 3 {
		t.Fatalf("err %v messages %d, want ErrTimeout with the request and 2 messages", err, len(res.Messages))
	}
}

func TestRun_should_use_tls_for_a_grpcs_server_with_the_given_tls_settings(t *testing.T) {
	creds, clientTLS := selfSignedCreds(t)
	f := startServer(t, true, creds)
	c := call(f, "acme.orders.v1.Orders/Get", `{"id": "3"}`)
	c.Server = "grpcs://" + f.Addr
	c.TLS = clientTLS

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || res.Code != codes.OK {
		t.Fatalf("err %v code %v", err, res.Code)
	}
}

func TestRun_should_refuse_a_body_that_does_not_fit_the_message_and_send_nothing(t *testing.T) {
	f := startServer(t, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Get", `{"id": 7}`))

	var bad *grpcx.BadBodyError
	if !errors.As(err, &bad) || !strings.Contains(err.Error(), "id") || len(res.Messages) != 0 {
		t.Fatalf("err %v messages %d, want a bad-body error naming the field and no message sent", err, len(res.Messages))
	}
}

func TestRun_should_send_without_a_field_the_message_does_not_have_and_warn_about_it(t *testing.T) {
	f := startServer(t, true, nil)

	res, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Get", `{"id": "7", "extra": 1}`))

	if err != nil || res.Code != codes.OK {
		t.Fatalf("an unknown field must not stop the call: err %v code %v", err, res.Code)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "extra") || strings.Contains(string(res.Messages[0].Body), "extra") {
		t.Fatalf("warnings %v sent %s, want one naming the field, and the field not sent", res.Warnings, res.Messages[0].Body)
	}
}

func TestRun_should_say_when_the_method_is_not_in_the_schema(t *testing.T) {
	f := startServer(t, true, nil)

	_, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Nope", `{}`))

	var schema *grpcx.SchemaError
	if !errors.As(err, &schema) || !strings.Contains(err.Error(), "Nope") {
		t.Fatalf("err %v, want a schema error naming the method", err)
	}
}

func TestRun_should_suggest_proto_files_when_the_server_has_no_reflection(t *testing.T) {
	f := startServer(t, false, nil)

	_, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Get", `{}`))

	var schema *grpcx.SchemaError
	if !errors.As(err, &schema) || !strings.Contains(err.Error(), ".proto") {
		t.Fatalf("err %v, want a schema error that suggests .proto files", err)
	}
}

func TestRun_should_name_a_missing_import(t *testing.T) {
	f := startServer(t, false, nil)
	root := writeProtos(t)
	if err := os.Remove(filepath.Join(root, "protos", "common.proto")); err != nil {
		t.Fatal(err)
	}

	_, err := grpcx.Run(t.Context(), withProtos(call(f, "acme.orders.v1.Orders/Get", `{}`), root))

	var schema *grpcx.SchemaError
	if !errors.As(err, &schema) || !strings.Contains(err.Error(), "common.proto") {
		t.Fatalf("err %v, want the missing import named", err)
	}
}

func TestRun_should_refuse_a_client_streaming_method(t *testing.T) {
	f := startServer(t, true, nil)

	_, err := grpcx.Run(t.Context(), call(f, "acme.orders.v1.Orders/Upload", `{}`))

	if err == nil || !strings.Contains(err.Error(), "streaming") {
		t.Fatalf("err %v, want a refusal that names client streaming", err)
	}
}

func TestRun_should_return_promptly_when_the_context_ends(t *testing.T) {
	f := startServer(t, true, nil)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(150*time.Millisecond, cancel)
	c := call(f, "acme.orders.v1.Orders/Slow", `{"id": "1"}`)
	c.Timeout = time.Minute
	started := time.Now()

	res, _ := grpcx.Run(ctx, c)

	if time.Since(started) > 5*time.Second || res.Code == codes.OK {
		t.Fatalf("returned after %v with code %v", time.Since(started), res.Code)
	}
}

func TestMethods_should_list_the_methods_from_files_and_from_reflection(t *testing.T) {
	f := startServer(t, true, nil)
	root := writeProtos(t)

	fromFiles, err1 := grpcx.Methods(t.Context(), withProtos(call(f, "", ""), root))
	fromServer, err2 := grpcx.Methods(t.Context(), call(f, "", ""))

	if err1 != nil || err2 != nil {
		t.Fatalf("files: %v, reflection: %v", err1, err2)
	}
	for name, got := range map[string][]grpcx.Method{"files": fromFiles, "reflection": fromServer} {
		var list, upload grpcx.Method
		for _, m := range got {
			switch m.Name {
			case "List":
				list = m
			case "Upload":
				upload = m
			}
		}
		if len(got) != 8 || !list.ServerStream || list.ClientStream || !upload.ClientStream || list.Service != "acme.orders.v1.Orders" || list.Output != "acme.orders.v1.Order" {
			t.Errorf("%s: %d methods, List %+v, Upload %+v", name, len(got), list, upload)
		}
	}
}

func TestMethods_should_write_an_example_message_for_each_request_type(t *testing.T) {
	f := startServer(t, true, nil)

	methods, err := grpcx.Methods(t.Context(), call(f, "", ""))

	if err != nil {
		t.Fatal(err)
	}
	example := map[string]string{}
	for _, m := range methods {
		example[m.Name] = m.Example
	}
	if example["Get"] != `{"id":""}` {
		t.Errorf("Get example %s", example["Get"])
	}
	want := `{"name":"","qty":0,"gift":false,"tags":[],"attrs":{},"color":"RED","price":{"currency":"","units":0},"pickup":"","at":"1970-01-01T00:00:00Z"}`
	if example["Create"] != want {
		t.Errorf("Create example\n got %s\nwant %s", example["Create"], want)
	}
}
