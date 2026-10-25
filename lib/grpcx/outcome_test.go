// SPDX-License-Identifier: Apache-2.0

package grpcx

import (
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// The call's own deadline and the wait's timer are the same length, so either can win. When the
// call's deadline does, the stream fails with a gRPC DeadlineExceeded status instead of the
// wait's timeout; with nothing to wait for that is the same thing: the time is up.
func TestStreamOutcome_Given_no_count_When_the_call_deadline_ends_the_stream_Then_it_is_a_clean_end(t *testing.T) {
	var res Result

	err := streamOutcome(status.Error(codes.DeadlineExceeded, "context deadline exceeded"), &res, true)

	if err != nil || res.Code != codes.OK {
		t.Fatalf("err %v code %v", err, res.Code)
	}
}

func TestStreamOutcome_Given_a_count_When_the_call_deadline_ends_the_stream_Then_the_code_is_reported(t *testing.T) {
	var res Result

	err := streamOutcome(status.Error(codes.DeadlineExceeded, "context deadline exceeded"), &res, false)

	if err != nil || res.Code != codes.DeadlineExceeded {
		t.Fatalf("err %v code %v", err, res.Code)
	}
}
