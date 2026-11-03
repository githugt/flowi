// SPDX-License-Identifier: Apache-2.0

package kafka

import (
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestReceived_should_take_the_topic_as_the_type(t *testing.T) {
	got := received(&kgo.Record{Topic: "orders.created", Value: []byte("x")})

	if got.Type != "orders.created" {
		t.Fatalf("type %q", got.Type)
	}
}
