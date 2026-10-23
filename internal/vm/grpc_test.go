// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"errors"
	"slices"
	"testing"

	flowi "github.com/githugt/flowi-lib"
	"github.com/githugt/flowi-lib/grpcx"
	"github.com/githugt/flowi-lib/model"

	"github.com/githugt/flowi/internal/vm"
	"github.com/githugt/flowi/internal/vm/vmtest"
)

func grpcBackend() *vmtest.Backend {
	b := asyncBackend()
	b.Cols[0].Requests[3].Versions[0].Async = nil
	b.Cols[0].Requests[3].Versions[0].GRPC = &model.GRPC{
		Server: "localhost:50051", Method: "shop.Shop/Get", Body: model.Body{JSON: `{"id":"1"}`}, Proto: []string{"protos/shop.proto"}}
	return b
}

func TestVerb_should_be_GRPC_for_a_grpc_version(t *testing.T) {
	if got := vm.Verb(model.RequestVersion{GRPC: &model.GRPC{}}); got != "GRPC" {
		t.Fatalf("verb %q", got)
	}
}

func TestRequestVM_should_edit_the_grpc_block_without_touching_what_is_on_disk(t *testing.T) {
	b := grpcBackend()
	r, _ := asyncRequestVM(t, b)

	r.EditGRPC(func(g *model.GRPC) { g.Method = "shop.Shop/List"; g.Proto = append(g.Proto, "protos/b.proto") })
	edited := r.Dirty() && r.GRPC().Method == "shop.Shop/List"
	r.Revert()
	back := r.GRPC()

	if !r.IsGRPC() || r.IsAsync() || !edited {
		t.Fatalf("grpc %v async %v edited %v", r.IsGRPC(), r.IsAsync(), edited)
	}
	if back.Method != "shop.Shop/Get" || len(back.Proto) != 1 {
		t.Fatalf("revert did not restore the block: %+v", back)
	}
}

func TestRequestVM_Configured_should_mark_the_grpc_sections_that_have_content(t *testing.T) {
	r, _ := asyncRequestVM(t, grpcBackend())

	got := map[vm.Section]bool{vm.SectionMessage: r.Configured(vm.SectionMessage), vm.SectionProto: r.Configured(vm.SectionProto), vm.SectionReceive: r.Configured(vm.SectionReceive)}

	if !got[vm.SectionMessage] || !got[vm.SectionProto] || got[vm.SectionReceive] {
		t.Fatalf("configured %v", got)
	}
}

func TestPipeline_should_show_the_scheme_the_call_will_use_in_the_tooltip_of_a_grpc_request(t *testing.T) {
	r, _ := asyncRequestVM(t, grpcBackend())

	var tip, verb string
	for _, row := range r.Pipeline(nil) {
		if row.Kind == vm.RowAPI {
			tip, verb = row.Tip, row.Method
		}
	}

	if tip != "grpc://localhost:50051/shop.Shop/Get" || verb != "GRPC" {
		t.Fatalf("tooltip %q verb %q", tip, verb)
	}
}

func TestWorkspace_CreateGRPCRequest_should_create_through_the_backend_and_seed_the_hooks(t *testing.T) {
	b := sampleBackend()
	ws, w := loadedWorkspace(t, b)

	ws.CreateGRPCRequest("C1", "orders/get", func(string) {}, func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("NewGRPCRequest orders/get") || !b.Called("SeedRequestHooks new") {
		t.Fatalf("calls %v", b.Calls)
	}
}

func TestMenuFor_should_offer_a_new_grpc_request_where_it_offers_a_new_async_one(t *testing.T) {
	ws, _, _ := shopWorkspace(t)

	for _, id := range []string{"c:C1", "c:C1/requests/auth"} {
		if got := actionsOf(ws.MenuFor([]string{id})); !slices.Contains(got, vm.ActNewGRPCRequest) {
			t.Errorf("%s menu %v lacks a new gRPC request", id, got)
		}
	}
}

func TestRequestVM_LoadMethods_should_list_what_the_backend_finds_and_say_when_it_fails(t *testing.T) {
	b := grpcBackend()
	b.GRPCMethodList = []grpcx.Method{{Service: "shop.Shop", Name: "Get", Input: "shop.Q", Output: "shop.A"}, {Service: "shop.Shop", Name: "Watch", ServerStream: true}}
	ws, w := loadedWorkspace(t, b)
	c := b.Cols[0]
	r := vm.NewRequest(b, w, c, c.Requests[3])
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())

	send.LoadMethods(r)
	w.Wait()
	got := r.Methods()
	b.GRPCMethodsErr = errors.New("the server does not offer reflection")
	send.LoadMethods(r)
	w.Wait()

	if len(got) != 2 || got[1].Full() != "shop.Shop/Watch" || !got[1].ServerStream {
		t.Fatalf("methods %+v", got)
	}
	if r.MethodsError() != "the server does not offer reflection" || len(r.Methods()) != 0 {
		t.Fatalf("after a failure: error %q methods %v", r.MethodsError(), r.Methods())
	}
}

func TestRequestVM_MethodStreams_should_say_whether_the_picked_method_streams_once_methods_are_loaded(t *testing.T) {
	b := grpcBackend()
	b.GRPCMethodList = []grpcx.Method{{Service: "shop.Shop", Name: "Get"}, {Service: "shop.Shop", Name: "Watch", ServerStream: true}}
	ws, w := loadedWorkspace(t, b)
	c := b.Cols[0]
	r := vm.NewRequest(b, w, c, c.Requests[3])
	send := vm.NewSend(b, w, ws, vm.NewRunLog(), vm.NewHistory())
	_, knownBefore := r.MethodStreams()

	send.LoadMethods(r)
	w.Wait()
	unary, known := r.MethodStreams()
	r.EditGRPC(func(g *model.GRPC) { g.Method = "shop.Shop/Watch" })
	streams, _ := r.MethodStreams()
	r.EditGRPC(func(g *model.GRPC) { g.Method = "shop.Shop/Nope" })
	_, knownOther := r.MethodStreams()

	if knownBefore || !known || unary || !streams || knownOther {
		t.Fatalf("before %v; unary %v known %v; streams %v; unknown method known %v", knownBefore, unary, known, streams, knownOther)
	}
}

func TestRequestVM_PickMethod_should_set_the_method_and_fill_an_empty_body_with_its_example(t *testing.T) {
	b := grpcBackend()
	b.Cols[0].Requests[3].Versions[0].GRPC.Body = model.Body{}
	r, _ := asyncRequestVM(t, b)
	get := grpcx.Method{Service: "shop.Shop", Name: "Get", Example: `{"id":""}`}
	watch := grpcx.Method{Service: "shop.Shop", Name: "Watch", Example: `{"filter":""}`}

	r.PickMethod(get)
	first := r.GRPC()
	r.PickMethod(watch)
	second := r.GRPC()

	if first.Method != "shop.Shop/Get" || first.Body.JSON != `{"id":""}` {
		t.Fatalf("after the first pick %+v", first)
	}
	if second.Method != "shop.Shop/Watch" || second.Body.JSON != `{"id":""}` {
		t.Fatalf("a body that has content is kept: %+v", second)
	}
}

func TestRequestVM_MethodsSource_should_say_whether_the_schema_comes_from_files_or_reflection(t *testing.T) {
	r, _ := asyncRequestVM(t, grpcBackend()) // lists a proto file
	byFiles := r.MethodsSource()
	r.EditGRPC(func(g *model.GRPC) { g.Proto = nil })

	if byFiles != vm.SourceFiles || r.MethodsSource() != vm.SourceReflection {
		t.Fatalf("with files %q, without %q", byFiles, r.MethodsSource())
	}
}

func TestWorkspace_GenerateGRPCRequests_should_create_through_the_backend_and_report_what_it_made(t *testing.T) {
	b := sampleBackend()
	b.GeneratedGRPC = flowi.GeneratedGRPC{Created: []flowi.RequestTarget{{Collection: "C1", Request: "a"}}, Skipped: []string{"x: already exists"}}
	ws, w := loadedWorkspace(t, b)
	var got flowi.GeneratedGRPC

	ws.GenerateGRPCRequests("C1", model.GRPC{Server: "localhost:9090"}, []grpcx.Method{{Service: "s.S", Name: "A"}},
		func(g flowi.GeneratedGRPC) { got = g }, func(err error) { t.Fatal(err) })
	w.Wait()

	if !b.Called("GenerateGRPCRequests localhost:9090 1") || len(got.Created) != 1 || len(got.Skipped) != 1 {
		t.Fatalf("calls %v result %+v", b.Calls, got)
	}
}
