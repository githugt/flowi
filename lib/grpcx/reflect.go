// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionv1 "google.golang.org/grpc/reflection/grpc_reflection_v1"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

// reflector asks a server about its own schema. There are two wire versions
// of the reflection service (v1, and the older v1alpha that many servers still
// offer); one implementation of each.
type reflector interface {
	services() ([]string, error)
	fileFor(symbol string) ([][]byte, error)
	fileNamed(name string) ([][]byte, error)
	close()
}

// openReflector tries v1, then v1alpha; a server with neither gives a
// SchemaError that suggests .proto files.
func openReflector(ctx context.Context, conn *grpc.ClientConn) (reflector, error) {
	r1, err := newV1(ctx, conn)
	if err != nil {
		return nil, err
	}
	if _, err := r1.services(); err == nil {
		return r1, nil
	} else if !isUnimplemented(err) {
		r1.close()
		return nil, reflectError(err)
	}
	r1.close()
	ra, err := newV1alpha(ctx, conn)
	if err != nil {
		return nil, err
	}
	if _, err := ra.services(); err != nil {
		ra.close()
		return nil, reflectError(err)
	}
	return ra, nil
}

func isUnimplemented(err error) bool { return status.Code(err) == codes.Unimplemented }

func reflectError(err error) error {
	if isUnimplemented(err) {
		return &SchemaError{Detail: noReflection}
	}
	return &SchemaError{Detail: "asking the server for its schema: " + err.Error()}
}

// reflectService fetches the file of one service (and what it imports).
func reflectService(ctx context.Context, conn *grpc.ClientConn, service string) (*protoregistry.Files, error) {
	r, err := openReflector(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer r.close()
	raw, err := r.fileFor(service)
	if err != nil {
		return nil, &SchemaError{Detail: fmt.Sprintf("the server has no service %s: %v", service, err)}
	}
	return buildFiles(raw, r.fileNamed)
}

// reflectAll fetches every service the server lists.
func reflectAll(ctx context.Context, conn *grpc.ClientConn) (*protoregistry.Files, error) {
	r, err := openReflector(ctx, conn)
	if err != nil {
		return nil, err
	}
	defer r.close()
	names, err := r.services()
	if err != nil {
		return nil, reflectError(err)
	}
	if len(names) == 0 {
		return nil, &SchemaError{Detail: errNoServices.Error()}
	}
	var raw [][]byte
	for _, name := range names {
		if name == "grpc.reflection.v1.ServerReflection" || name == "grpc.reflection.v1alpha.ServerReflection" {
			continue
		}
		files, err := r.fileFor(name)
		if err != nil {
			return nil, &SchemaError{Detail: fmt.Sprintf("the server has no schema for %s: %v", name, err)}
		}
		raw = append(raw, files...)
	}
	return buildFiles(raw, r.fileNamed)
}

func unmarshalFile(b []byte, fdp *descriptorpb.FileDescriptorProto) error {
	if err := proto.Unmarshal(b, fdp); err != nil {
		return &SchemaError{Detail: "the server sent a descriptor that does not parse: " + err.Error()}
	}
	return nil
}

// ---- v1 ----

type v1 struct {
	cancel context.CancelFunc
	stream reflectionv1.ServerReflection_ServerReflectionInfoClient
}

func newV1(ctx context.Context, conn *grpc.ClientConn) (*v1, error) {
	ctx, cancel := context.WithCancel(ctx)
	s, err := reflectionv1.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		cancel()
		return nil, reflectError(err)
	}
	return &v1{cancel: cancel, stream: s}, nil
}

func (r *v1) close() { r.cancel() }

func (r *v1) ask(req *reflectionv1.ServerReflectionRequest) (*reflectionv1.ServerReflectionResponse, error) {
	if err := r.stream.Send(req); err != nil {
		if errors.Is(err, io.EOF) { // the real status comes from Recv
			_, rerr := r.stream.Recv()
			return nil, rerr
		}
		return nil, err
	}
	resp, err := r.stream.Recv()
	if err != nil {
		return nil, err
	}
	if e := resp.GetErrorResponse(); e != nil {
		return nil, status.Error(codes.Code(e.GetErrorCode()), e.GetErrorMessage()) //nolint:gosec // an error code from the wire, small
	}
	return resp, nil
}

func (r *v1) services() ([]string, error) {
	resp, err := r.ask(&reflectionv1.ServerReflectionRequest{MessageRequest: &reflectionv1.ServerReflectionRequest_ListServices{}})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range resp.GetListServicesResponse().GetService() {
		out = append(out, s.GetName())
	}
	return out, nil
}

func (r *v1) fileFor(symbol string) ([][]byte, error) {
	resp, err := r.ask(&reflectionv1.ServerReflectionRequest{MessageRequest: &reflectionv1.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol}})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (r *v1) fileNamed(name string) ([][]byte, error) {
	resp, err := r.ask(&reflectionv1.ServerReflectionRequest{MessageRequest: &reflectionv1.ServerReflectionRequest_FileByFilename{FileByFilename: name}})
	if err != nil {
		return nil, &SchemaError{Detail: fmt.Sprintf("the server could not send %s: %v", name, err)}
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}
