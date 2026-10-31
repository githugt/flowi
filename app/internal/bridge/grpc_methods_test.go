// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"testing"

	flowi "github.com/githugt/flowi/lib"
	"github.com/githugt/flowi/lib/grpcx"
	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openRPC opens a gRPC request whose server lists methods, returning the
// requests service, the request's node and what the window was sent.
func openRPC(t *testing.T, methods []grpcx.Method) (*bridge.RequestService, string, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R9", Name: "Rpc"}, Path: "rpc", File: "/c/requests/rpc.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", GRPC: &model.GRPC{Server: "grpc://localhost:9"}}}})
	b := &vmtest.Backend{Cols: []model.Collection{c}, GRPCMethodList: methods,
		GeneratedGRPC: flowi.GeneratedGRPC{Created: make([]flowi.RequestTarget, len(methods))}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == "Rpc" {
			if _, err := app.Requests().Open(n.ID); err != nil {
				t.Fatal(err)
			}
			return app.Requests(), n.ID, pub
		}
	}
	t.Fatal("no Rpc request")
	return nil, "", nil
}

func sampleMethods() []grpcx.Method {
	return []grpcx.Method{
		{Service: "myexampleapp.v1.ExampleService", Name: "GetExample", Input: "myexampleapp.v1.GetExampleRequest", Output: "myexampleapp.v1.Example"},
		{Service: "myexampleapp.v1.ExampleService", Name: "ListExamples", ServerStream: true},
		{Service: "myexampleapp.v1.ExampleService", Name: "Upload", ClientStream: true},
	}
}

// The picker showed only "package.Service/" for long names: the window wraps
// a datalist entry at the slash. It now draws its own list from each
// method's service, name and kind.
func TestRequestService_LoadMethods_should_list_each_method_with_its_service_name_and_kind(t *testing.T) {
	reqs, node, pub := openRPC(t, sampleMethods())

	reqs.LoadMethods(node)

	got := pub.last["request"].(bridge.RequestView).GRPC.Methods
	if len(got) != 3 {
		t.Fatalf("methods %+v", got)
	}
	first, stream, upload := got[0], got[1], got[2]
	if first.Service != "myexampleapp.v1.ExampleService" || first.Name != "GetExample" || first.Full != "myexampleapp.v1.ExampleService/GetExample" ||
		first.Input != "myexampleapp.v1.GetExampleRequest" || first.ServerStream || first.ClientStream {
		t.Fatalf("unary %+v", first)
	}
	if !stream.ServerStream || stream.ClientStream || !upload.ClientStream || upload.ServerStream {
		t.Fatalf("stream %+v upload %+v", stream, upload)
	}
}

func TestRequestService_GenerateRequests_should_make_a_request_per_loaded_method(t *testing.T) {
	tests := []struct {
		name        string
		load        bool
		wantCreated int
		wantErr     bool
	}{
		{name: "methods loaded", load: true, wantCreated: 3},
		{name: "nothing loaded yet", load: false, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reqs, node, _ := openRPC(t, sampleMethods())
			if tt.load {
				reqs.LoadMethods(node)
			}

			got, err := reqs.GenerateRequests(node)

			if (err != nil) != tt.wantErr || got.Created != tt.wantCreated {
				t.Fatalf("got %+v err %v, want %d created (error %v)", got, err, tt.wantCreated, tt.wantErr)
			}
		})
	}
}
