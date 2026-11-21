// SPDX-License-Identifier: Apache-2.0

package nats_test

import (
	"testing"

	natsserver "github.com/nats-io/nats-server/v2/server"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/nats"
)

func TestProbe_should_connect_and_refuse_a_wrong_token(t *testing.T) {
	// Before: a wrong token showed only after a full send.
	url := startServer(t, func(o *natsserver.Options) { o.Authorization = "secret" })
	op := transport.Operation{Server: url, Channel: "orders", Send: &transport.Message{Body: []byte("x")}, Binding: map[string]string{"stream": "missing"}}

	noToken := transport.Probe(t.Context(), nats.New(), op)
	op.Binding["token"] = "wrong"
	wrong := transport.Probe(t.Context(), nats.New(), op)
	op.Binding["token"] = "secret"
	right := transport.Probe(t.Context(), nats.New(), op)

	if noToken == nil || wrong == nil || right != nil {
		t.Fatalf("none %v, wrong %v, right %v (a stream that does not exist is not looked up)", noToken, wrong, right)
	}
}
