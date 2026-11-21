// SPDX-License-Identifier: Apache-2.0

package redis_test

import (
	"testing"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/redis"
)

func TestProbe_should_connect_sign_in_and_touch_nothing(t *testing.T) {
	// Before: a wrong address or password showed only after a full send.
	m := startRedis(t)
	m.RequireAuth("pw")
	op := transport.Operation{Server: server(m), Channel: "orders", Send: body("x"), Receive: recv(1, 0)}

	noPassword := transport.Probe(t.Context(), redis.New(), op)
	op.Binding = map[string]string{"password": "pw"}
	signedIn := transport.Probe(t.Context(), redis.New(), op)

	if noPassword == nil || signedIn != nil {
		t.Fatalf("without the password %v, with it %v", noPassword, signedIn)
	}
	if len(m.Keys()) != 0 {
		t.Fatalf("the probe wrote %v", m.Keys())
	}
}

func TestProbe_Given_nothing_listening_Then_it_says_it_could_not_connect(t *testing.T) {
	m := startRedis(t)
	url := server(m)
	m.Close()

	err := transport.Probe(t.Context(), redis.New(), transport.Operation{Server: url})

	if err == nil {
		t.Fatal("want an error")
	}
}
