// SPDX-License-Identifier: Apache-2.0

package grpcx_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"

	"github.com/bufbuild/protocompile"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha" //nolint:staticcheck // see startServerWith
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"
)

// ordersProto is the service the tests call. It imports a second file so
// import roots are exercised too.
const ordersProto = `syntax = "proto3";
package acme.orders.v1;
import "common.proto";
import "google/protobuf/timestamp.proto";

service Orders {
  rpc Get(GetRequest) returns (Order);
  rpc Fail(GetRequest) returns (Order);
  rpc Slow(GetRequest) returns (Order);
  rpc Echo(GetRequest) returns (Order);
  rpc List(ListRequest) returns (stream Order);
  rpc Trickle(ListRequest) returns (stream Order);
  rpc Upload(stream GetRequest) returns (Order);
  rpc Create(CreateRequest) returns (Order);
}

message GetRequest { string id = 1; }
enum Color { RED = 0; BLUE = 1; }
message CreateRequest {
  string name = 1;
  int32 qty = 2;
  bool gift = 3;
  repeated string tags = 4;
  map<string, string> attrs = 5;
  Color color = 6;
  acme.common.Money price = 7;
  oneof where { string pickup = 8; string ship = 9; }
  google.protobuf.Timestamp at = 10;
}
message ListRequest { int32 count = 1; }
message Order {
  string id = 1;
  string name = 2;
  acme.common.Money total = 3;
  int32 seq = 4;
}
`

const commonProto = `syntax = "proto3";
package acme.common;
message Money { string currency = 1; int64 units = 2; }
`

// fake is a running gRPC server over a real socket, with the methods of
// ordersProto implemented through dynamicpb.
type fake struct {
	Addr  string
	files *protoregistry.Files
}

// startServer runs the server; reflect says whether it offers reflection,
// creds (nil for plaintext) secures it.
func startServer(t *testing.T, withReflection bool, creds credentials.TransportCredentials) *fake {
	return startServerWith(t, reflectV1, withReflection, creds)
}

// reflectVersion says which reflection service a test server offers.
type reflectVersion int

const (
	reflectV1 reflectVersion = iota
	reflectV1Alpha
)

func startServerWith(t *testing.T, version reflectVersion, withReflection bool, creds credentials.TransportCredentials) *fake {
	t.Helper()
	files := compileTestFiles(t)
	f := &fake{files: files}
	opts := []grpc.ServerOption{grpc.UnknownServiceHandler(f.handle)}
	if creds != nil {
		opts = append(opts, grpc.Creds(creds))
	}
	srv := grpc.NewServer(opts...)
	if withReflection {
		options := reflection.ServerOptions{Services: serviceLister{"acme.orders.v1.Orders"}, DescriptorResolver: files, ExtensionResolver: protoregistry.GlobalTypes}
		if version == reflectV1Alpha {
			reflectionv1alpha.RegisterServerReflectionServer(srv, reflection.NewServer(options)) //nolint:staticcheck // the point is to test the old version
		} else {
			reflectionv1.RegisterServerReflectionServer(srv, reflection.NewServerV1(options))
		}
	}
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(srv.Stop)
	f.Addr = lis.Addr().String()
	return f
}

type serviceLister []string

func (s serviceLister) GetServiceInfo() map[string]grpc.ServiceInfo {
	out := map[string]grpc.ServiceInfo{}
	for _, name := range s {
		out[name] = grpc.ServiceInfo{}
	}
	return out
}

func compileTestFiles(t *testing.T) *protoregistry.Files {
	t.Helper()
	c := protocompile.Compiler{Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
		Accessor: protocompile.SourceAccessorFromMap(map[string]string{"orders.proto": ordersProto, "common.proto": commonProto})})}
	compiled, err := c.Compile(t.Context(), "orders.proto")
	if err != nil {
		t.Fatal(err)
	}
	files := &protoregistry.Files{}
	for _, f := range []protoreflect.FileDescriptor{compiled[0].Imports().Get(0).FileDescriptor, compiled[0]} {
		if err := files.RegisterFile(f); err != nil {
			t.Fatal(err)
		}
	}
	return files
}

// handle answers every method: the name picks the behaviour.
func (f *fake) handle(_ any, stream grpc.ServerStream) error {
	full, _ := grpc.MethodFromServerStream(stream)
	d, err := f.files.FindDescriptorByName("acme.orders.v1.Orders")
	if err != nil {
		return status.Error(codes.Unimplemented, "no such service")
	}
	method := d.(protoreflect.ServiceDescriptor).Methods().ByName(protoreflect.Name(full[len("/acme.orders.v1.Orders/"):]))
	if method == nil {
		return status.Error(codes.Unimplemented, "no such method "+full)
	}
	in := dynamicpb.NewMessage(method.Input())
	if err := stream.RecvMsg(in); err != nil {
		return err
	}
	return f.answer(stream, string(method.Name()), method, in)
}

func (f *fake) answer(stream grpc.ServerStream, name string, method protoreflect.MethodDescriptor, in *dynamicpb.Message) error {
	id := ""
	if fd := in.Descriptor().Fields().ByName("id"); fd != nil {
		id = in.Get(fd).String()
	}
	order := func(seq int) *dynamicpb.Message {
		out := dynamicpb.NewMessage(method.Output())
		set := func(field string, v protoreflect.Value) {
			out.Set(out.Descriptor().Fields().ByName(protoreflect.Name(field)), v)
		}
		set("id", protoreflect.ValueOfString(id))
		set("name", protoreflect.ValueOfString("order-"+id))
		set("seq", protoreflect.ValueOfInt32(int32(seq))) //nolint:gosec // test counts are tiny
		return out
	}
	switch name {
	case "Get":
		return stream.SendMsg(order(0))
	case "Fail":
		return status.Error(codes.NotFound, "no order "+id)
	case "Slow":
		<-stream.Context().Done()
		return status.FromContextError(stream.Context().Err()).Err()
	case "Echo":
		md, _ := metadata.FromIncomingContext(stream.Context())
		_ = stream.SendHeader(metadata.Pairs("x-seen", fmt.Sprint(md.Get("x-token"))))
		stream.SetTrailer(metadata.Pairs("x-trail", "done"))
		return stream.SendMsg(order(0))
	case "List", "Trickle":
		count := int(in.Get(in.Descriptor().Fields().ByName("count")).Int())
		for i := 1; i <= count; i++ {
			id = fmt.Sprint(i)
			if err := stream.SendMsg(order(i)); err != nil {
				return err
			}
		}
		if name == "Trickle" { // keeps the stream open after its messages
			<-stream.Context().Done()
		}
		return nil
	}
	return status.Error(codes.Unimplemented, name)
}

// selfSignedCreds makes a certificate for 127.0.0.1 and the credentials and
// client TLS config that trust it.
func selfSignedCreds(t *testing.T) (credentials.TransportCredentials, *tls.Config) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "flowi-test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, IsCA: true, BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	server := credentials.NewTLS(&tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}, MinVersion: tls.VersionTLS12})
	return server, &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
}

var _ = protojson.MarshalOptions{}
var _ = context.Background
