// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/githugt/flowi/lib/model"
)

// healthCheck is the standard health service's method: a server that has it answers, and one that has not says it
// does not know the method, which shows it was reached just the same.
const healthCheck = "/grpc.health.v1.Health/Check"

// Probe tests the connection of a call without making it: it compiles the call's .proto files when it names any,
// connects to the server (TCP, TLS, HTTP/2) and sends the call's metadata with a health check, so a refused sign-in
// shows. No method of the service is called and nothing is changed.
//
// Before: a gRPC request had no connection test, so a wrong port or a refused token showed only when the call ran.
func Probe(ctx context.Context, c Call) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout())
	defer cancel()
	if len(c.Protos) > 0 {
		if _, err := compileProtos(ctx, c); err != nil {
			return err
		}
	}
	conn, err := dial(c)
	if err != nil {
		return err
	}
	defer conn.Close() //nolint:errcheck // nothing useful to do if closing fails
	out, err := outgoingContext(ctx, c.Metadata)
	if err != nil {
		return err
	}
	err = conn.Invoke(out, healthCheck, &emptypb.Empty{}, &emptypb.Empty{})
	hostport, _ := model.GRPCTarget(c.Server)
	switch st, _ := status.FromError(err); st.Code() {
	case codes.OK, codes.Unimplemented, codes.NotFound, codes.InvalidArgument, codes.FailedPrecondition, codes.Internal:
		return nil
	case codes.DeadlineExceeded:
		return context.DeadlineExceeded
	case codes.Unauthenticated, codes.PermissionDenied:
		return fmt.Errorf("%s refused the call's metadata (%s): %s", hostport, st.Code(), st.Message())
	default:
		return fmt.Errorf("connecting to %s: %s", hostport, st.Message())
	}
}
