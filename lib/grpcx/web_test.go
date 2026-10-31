// SPDX-License-Identifier: Apache-2.0

package grpcx_test

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/transport"
)

func webFrame(flag byte, payload []byte) []byte {
	out := []byte{flag, 0, 0, 0, 0}
	binary.BigEndian.PutUint32(out[1:], uint32(len(payload)))
	return append(out, payload...)
}

// webGateway is what a gRPC-Web proxy does: it takes the POST, calls the
// gRPC server behind it, and writes the answer as frames with the status in the last.
// Each request it sees is kept in seen.
type webGateway struct {
	*httptest.Server
	seen chan http.Header
}

func startWebGateway(t *testing.T, f *fake) *webGateway {
	t.Helper()
	files := compileTestFiles(t)
	conn, err := grpc.NewClient(f.Addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	g := &webGateway{seen: make(chan http.Header, 8)}
	g.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		g.seen <- r.Header.Clone()
		serviceDesc, _ := files.FindDescriptorByName("acme.orders.v1.Orders")
		name := strings.TrimPrefix(r.URL.Path, "/acme.orders.v1.Orders/")
		md := serviceDesc.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(name))
		if md == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		body, _ := io.ReadAll(r.Body)
		in := dynamicpb.NewMessage(md.Input())
		if err := proto.Unmarshal(body[5:], in); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/grpc-web+proto")
		w.WriteHeader(http.StatusOK)
		stream, err := conn.NewStream(r.Context(), &grpc.StreamDesc{ServerStreams: md.IsStreamingServer()}, "/acme.orders.v1.Orders/"+name)
		if err == nil {
			err = stream.SendMsg(in)
		}
		if err == nil {
			err = stream.CloseSend()
		}
		for err == nil {
			out := dynamicpb.NewMessage(md.Output())
			if err = stream.RecvMsg(out); err != nil {
				break
			}
			payload, _ := proto.Marshal(out)
			_, _ = w.Write(webFrame(0, payload))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		}
		st := status.New(codes.OK, "")
		if !errors.Is(err, io.EOF) {
			st, _ = status.FromError(err)
		}
		trailers := "grpc-status: " + itoa(int(st.Code())) + "\r\ngrpc-message: " + url.PathEscape(st.Message()) + "\r\nx-trail: done\r\n"
		_, _ = w.Write(webFrame(0x80, []byte(trailers)))
	}))
	t.Cleanup(g.Close)
	return g
}

func itoa(n int) string { return strconv.Itoa(n) }

func webCall(g *webGateway, method, body, root string) grpcx.Call {
	return grpcx.Call{Server: g.URL, Method: method, JSON: []byte(body), Web: true, Timeout: 5 * time.Second,
		Root: root, Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}}
}

func TestRun_Web_should_make_a_unary_call_through_a_gRPC_Web_proxy(t *testing.T) {
	// Arrange
	f := startServer(t, false, nil)
	g := startWebGateway(t, f)
	c := webCall(g, "acme.orders.v1.Orders/Get", `{"id": "7"}`, writeProtos(t))
	c.Metadata = []transport.Header{{Name: "X-Token", Value: "abc"}}

	// Act
	res, err := grpcx.Run(t.Context(), c)

	// Assert
	if err != nil || res.Code != codes.OK || len(res.Messages) != 2 {
		t.Fatalf("err %v code %v messages %d", err, res.Code, len(res.Messages))
	}
	if got := string(res.Messages[1].Body); !strings.Contains(got, `"name":"order-7"`) {
		t.Errorf("response %s", got)
	}
	seen := <-g.seen
	if seen.Get("Content-Type") != "application/grpc-web+proto" || seen.Get("X-Grpc-Web") != "1" || seen.Get("X-Token") != "abc" || seen.Get("Grpc-Timeout") == "" {
		t.Errorf("the proxy saw headers %v", seen)
	}
	if header(res.Trailer, "x-trail") != "done" {
		t.Errorf("trailers %+v", res.Trailer)
	}
}

func TestRun_Web_should_keep_the_messages_of_a_server_stream_and_stop_at_the_count(t *testing.T) {
	f := startServer(t, false, nil)
	g := startWebGateway(t, f)
	c := webCall(g, "acme.orders.v1.Orders/List", `{"count": 5}`, writeProtos(t))
	c.Receive = &transport.Receive{Count: 2, Timeout: 3 * time.Second}

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || res.Code != codes.OK || len(res.Messages) != 3 {
		t.Fatalf("err %v code %v messages %d, want the request and 2 of the 5 orders", err, res.Code, len(res.Messages))
	}
}

func TestRun_Web_should_read_a_whole_stream_when_no_count_is_given(t *testing.T) {
	f := startServer(t, false, nil)
	g := startWebGateway(t, f)

	res, err := grpcx.Run(t.Context(), webCall(g, "acme.orders.v1.Orders/List", `{"count": 3}`, writeProtos(t)))

	if err != nil || res.Code != codes.OK || len(res.Messages) != 4 {
		t.Fatalf("err %v code %v messages %d", err, res.Code, len(res.Messages))
	}
}

func TestRun_Web_should_report_the_status_in_the_trailers(t *testing.T) {
	f := startServer(t, false, nil)
	g := startWebGateway(t, f)

	res, err := grpcx.Run(t.Context(), webCall(g, "acme.orders.v1.Orders/Fail", `{"id": "1"}`, writeProtos(t)))

	if err != nil || res.Code != codes.NotFound || res.Message != "no order 1" {
		t.Fatalf("err %v code %v message %q", err, res.Code, res.Message)
	}
}

func TestRun_Web_Given_a_body_that_ends_without_trailers_Then_the_call_has_no_trusted_status(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/grpc-web+proto")
		_, _ = w.Write(webFrame(0, nil))
	}))
	defer srv.Close()
	c := grpcx.Call{Server: srv.URL, Method: "acme.orders.v1.Orders/Get", JSON: []byte(`{}`), Web: true, Timeout: 3 * time.Second,
		Root: writeProtos(t), Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}}

	// Act
	res, err := grpcx.Run(t.Context(), c)

	// Assert
	if err != nil || res.Code != codes.Unknown || !strings.Contains(res.Message, "without a gRPC status") {
		t.Fatalf("err %v code %v message %q", err, res.Code, res.Message)
	}
}

func TestRun_Web_should_map_an_http_error_from_the_proxy_to_a_status(t *testing.T) {
	cases := map[int]codes.Code{401: codes.Unauthenticated, 403: codes.PermissionDenied, 404: codes.Unimplemented, 503: codes.Unavailable, 418: codes.Unknown}
	for status, want := range cases {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(status) }))
		c := grpcx.Call{Server: srv.URL, Method: "acme.orders.v1.Orders/Get", JSON: []byte(`{}`), Web: true, Timeout: 3 * time.Second,
			Root: writeProtos(t), Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}}

		res, err := grpcx.Run(t.Context(), c)
		srv.Close()

		if err != nil || res.Code != want {
			t.Errorf("HTTP %d: err %v code %v, want %v", status, err, res.Code, want)
		}
	}
}

func TestRun_Web_should_refuse_what_gRPC_Web_cannot_do_before_sending(t *testing.T) {
	root := writeProtos(t)
	cases := map[string]grpcx.Call{
		"no proto files":      {Server: "http://x", Method: "acme.orders.v1.Orders/Get", Web: true},
		"client stream":       {Server: "http://x", Method: "acme.orders.v1.Orders/Upload", Web: true, Root: root, Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}},
		"not an http address": {Server: "ftp://x", Method: "acme.orders.v1.Orders/Get", Web: true, Root: root, Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := grpcx.Run(context.Background(), c)

			if err == nil {
				t.Fatal("the call must be refused")
			}
		})
	}
}

func TestRun_Web_Given_the_proxy_is_unreachable_Then_the_status_is_unavailable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.URL
	srv.Close()
	c := grpcx.Call{Server: addr, Method: "acme.orders.v1.Orders/Get", JSON: []byte(`{}`), Web: true, Timeout: 3 * time.Second,
		Root: writeProtos(t), Protos: []string{"protos/orders.proto"}, Imports: []string{"protos"}}

	res, err := grpcx.Run(t.Context(), c)

	if err != nil || res.Code != codes.Unavailable {
		t.Fatalf("err %v code %v", err, res.Code)
	}
	if strings.Contains(res.Message, addr) {
		t.Errorf("the message quotes the address: %q", res.Message)
	}
}
