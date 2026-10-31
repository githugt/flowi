// SPDX-License-Identifier: Apache-2.0

//nolint:staticcheck // v1alpha is deprecated but many servers still offer only it, so it is the fallback.
package grpcx

import (
	"context"
	"errors"
	"fmt"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	reflectionv1alpha "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
)

type v1alpha struct {
	cancel context.CancelFunc
	stream reflectionv1alpha.ServerReflection_ServerReflectionInfoClient
}

func newV1alpha(ctx context.Context, conn *grpc.ClientConn) (*v1alpha, error) {
	ctx, cancel := context.WithCancel(ctx)
	s, err := reflectionv1alpha.NewServerReflectionClient(conn).ServerReflectionInfo(ctx)
	if err != nil {
		cancel()
		return nil, reflectError(err)
	}
	return &v1alpha{cancel: cancel, stream: s}, nil
}

func (r *v1alpha) close() { r.cancel() }

func (r *v1alpha) ask(req *reflectionv1alpha.ServerReflectionRequest) (*reflectionv1alpha.ServerReflectionResponse, error) {
	if err := r.stream.Send(req); err != nil {
		if errors.Is(err, io.EOF) {
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

func (r *v1alpha) services() ([]string, error) {
	resp, err := r.ask(&reflectionv1alpha.ServerReflectionRequest{MessageRequest: &reflectionv1alpha.ServerReflectionRequest_ListServices{}})
	if err != nil {
		return nil, err
	}
	var out []string
	for _, s := range resp.GetListServicesResponse().GetService() {
		out = append(out, s.GetName())
	}
	return out, nil
}

func (r *v1alpha) fileFor(symbol string) ([][]byte, error) {
	resp, err := r.ask(&reflectionv1alpha.ServerReflectionRequest{MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileContainingSymbol{FileContainingSymbol: symbol}})
	if err != nil {
		return nil, err
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}

func (r *v1alpha) fileNamed(name string) ([][]byte, error) {
	resp, err := r.ask(&reflectionv1alpha.ServerReflectionRequest{MessageRequest: &reflectionv1alpha.ServerReflectionRequest_FileByFilename{FileByFilename: name}})
	if err != nil {
		return nil, &SchemaError{Detail: fmt.Sprintf("the server could not send %s: %v", name, err)}
	}
	return resp.GetFileDescriptorResponse().GetFileDescriptorProto(), nil
}
