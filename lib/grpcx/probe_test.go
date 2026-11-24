// SPDX-License-Identifier: Apache-2.0

package grpcx_test

import (
	"net"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/grpcx"
)

func TestProbe_Given_a_running_server_Then_it_connects_without_calling_a_method(t *testing.T) {
	// Before: a gRPC request had no connection test.
	f := startServer(t, false, nil)

	err := grpcx.Probe(t.Context(), grpcx.Call{Server: "grpc://" + f.Addr, Timeout: 5 * time.Second})

	if err != nil {
		t.Fatalf("a server that does not know the health service is still reached: %v", err)
	}
}

func TestProbe_Given_no_server_Then_it_says_it_could_not_connect(t *testing.T) {
	lis, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := lis.Addr().String()
	_ = lis.Close()

	err = grpcx.Probe(t.Context(), grpcx.Call{Server: "grpc://" + addr, Timeout: 5 * time.Second})

	if err == nil {
		t.Fatal("nothing listens there")
	}
}
