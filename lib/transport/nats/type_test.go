// SPDX-License-Identifier: Apache-2.0

package nats

import (
	"testing"
	"time"

	natsgo "github.com/nats-io/nats.go"
)

// A subscription to a wildcard gets messages from many subjects; each message's subject is its type.
func TestCoreMessage_should_take_the_subject_as_the_type(t *testing.T) {
	got := coreMessage(&natsgo.Msg{Subject: "orders.created", Data: []byte("x")}, time.Time{})

	if got.Type != "orders.created" {
		t.Fatalf("type %q", got.Type)
	}
}
