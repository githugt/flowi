// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/bufbuild/protocompile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"
)

type serviceNames []string

func (s serviceNames) GetServiceInfo() map[string]grpc.ServiceInfo {
	out := map[string]grpc.ServiceInfo{}
	for _, n := range s {
		out[n] = grpc.ServiceInfo{}
	}
	return out
}

// startReflectingServer runs a gRPC server that offers reflection for one
// service, Shop/Get, and answers nothing else.
func startReflectingServer(t *testing.T) string {
	t.Helper()
	c := protocompile.Compiler{Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
		Accessor: protocompile.SourceAccessorFromMap(map[string]string{"shop.proto": `syntax = "proto3";
package shop; message Q { string id = 1; } message A { string name = 1; } service Shop { rpc Get(Q) returns (A); }`})})}
	compiled, err := c.Compile(t.Context(), "shop.proto")
	if err != nil {
		t.Fatal(err)
	}
	files := &protoregistry.Files{}
	if err := files.RegisterFile(compiled[0]); err != nil {
		t.Fatal(err)
	}
	srv := grpc.NewServer()
	reflectionv1.RegisterServerReflectionServer(srv, reflection.NewServerV1(reflection.ServerOptions{
		Services: serviceNames{"shop.Shop"}, DescriptorResolver: files, ExtensionResolver: protoregistry.GlobalTypes}))
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	return lis.Addr().String()
}

func TestNewGRPCRequest_should_create_a_grpc_version_that_loads_back(t *testing.T) {
	w := demoWorkspace(t)

	target, err := w.NewGRPCRequest(w.Collections()[0].Manifest.ID, "orders/get")

	if err != nil {
		t.Fatal(err)
	}
	_, r, err := w.requestFor(target)
	if err != nil || r.Versions[0].GRPC == nil || r.Versions[0].Async != nil {
		t.Fatalf("loaded back as %+v, err %v", r.Versions, err)
	}
}

func TestGRPCMethods_should_list_the_methods_of_the_server_the_request_points_at(t *testing.T) {
	w := demoWorkspace(t)
	addr := startReflectingServer(t)
	target, err := w.NewGRPCRequest(w.Collections()[0].Manifest.ID, "shop/get")
	if err != nil {
		t.Fatal(err)
	}
	v := model.RequestVersion{Label: "1", GRPC: &model.GRPC{Server: "grpc://" + addr, Method: "shop.Shop/Get"}}

	methods, err := w.GRPCMethods(t.Context(), target, v, SendOptions{})

	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, m := range methods {
		names = append(names, m.Full())
	}
	if !slices.Contains(names, "shop.Shop/Get") {
		t.Fatalf("methods %v", names)
	}
}

func TestGRPCMethods_should_refuse_a_request_that_is_not_grpc(t *testing.T) {
	w := demoWorkspace(t)
	target, err := w.NewRequest(w.Collections()[0].Manifest.ID, "plain/http")
	if err != nil {
		t.Fatal(err)
	}
	_, r, _ := w.requestFor(target)

	_, err = w.GRPCMethods(t.Context(), target, r.Versions[0], SendOptions{})

	var le *diag.ListError
	if !errors.As(err, &le) || le.List[0].Code != diag.CodeGRPCRequest {
		t.Fatalf("err %v", err)
	}
}

func TestGenerateGRPCRequests_should_create_a_request_per_method_with_an_example_message_and_hooks(t *testing.T) {
	w := demoWorkspace(t)
	col := w.Collections()[0].Manifest.ID
	methods := []grpcx.Method{
		{Service: "shop.v1.Shop", Name: "Get", Example: `{"id":""}`},
		{Service: "shop.v1.Shop", Name: "Watch", ServerStream: true, Example: `{}`},
		{Service: "shop.v1.Shop", Name: "Upload", ClientStream: true, Example: `{}`},
	}
	base := model.GRPC{Server: "grpc://localhost:9090", Proto: []string{"protos/shop.proto"}, Import: []string{"protos"}}

	got, err := w.GenerateGRPCRequests(col, base, methods)

	if err != nil {
		t.Fatal(err)
	}
	if len(got.Created) != 2 || len(got.Skipped) != 1 || !strings.Contains(got.Skipped[0], "Upload") {
		t.Fatalf("created %d skipped %v, want 2 created and the client-streaming method skipped", len(got.Created), got.Skipped)
	}
	_, r, err := w.requestFor(got.Created[0])
	if err != nil {
		t.Fatal(err)
	}
	g := r.Versions[0].GRPC
	if r.Path != "grpc/Shop/Get" || g == nil || g.Server != "grpc://localhost:9090" || g.Method != "shop.v1.Shop/Get" || g.Body.JSON != `{"id":""}` || len(g.Proto) != 1 || len(g.Import) != 1 {
		t.Fatalf("request %q grpc %+v", r.Path, g)
	}
	if len(r.Hooks.Post) != 1 {
		t.Fatalf("hooks %+v, want the seeded gRPC hooks", r.Hooks)
	}
	if diags := w.Validate(); diags.HasErrors() {
		t.Fatalf("errors after generating: %v", diags)
	}
}

func TestGenerateGRPCRequests_should_skip_a_request_that_already_exists_instead_of_failing(t *testing.T) {
	w := demoWorkspace(t)
	col := w.Collections()[0].Manifest.ID
	methods := []grpcx.Method{{Service: "shop.v1.Shop", Name: "Get", Example: `{}`}}
	base := model.GRPC{Server: "localhost:9090"}
	if _, err := w.GenerateGRPCRequests(col, base, methods); err != nil {
		t.Fatal(err)
	}

	again, err := w.GenerateGRPCRequests(col, base, methods)

	if err != nil || len(again.Created) != 0 || len(again.Skipped) != 1 || !strings.Contains(again.Skipped[0], "already exists") {
		t.Fatalf("created %d skipped %v err %v", len(again.Created), again.Skipped, err)
	}
}
