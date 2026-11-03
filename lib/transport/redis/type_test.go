// SPDX-License-Identifier: Apache-2.0

package redis

import (
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// A pattern subscription gets messages from many channels; each message's channel is its type.
func TestPubSubMessage_should_take_the_channel_as_the_type(t *testing.T) {
	got := pubSubMessage(&goredis.Message{Channel: "cache.invalidated", Pattern: "cache.*", Payload: "k"}, time.Time{})

	if got.Type != "cache.invalidated" {
		t.Fatalf("type %q", got.Type)
	}
}
