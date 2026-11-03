// SPDX-License-Identifier: Apache-2.0

package mqtt

import (
	"testing"
	"time"

	"github.com/eclipse/paho.golang/paho"
)

// A wildcard subscription gets messages from many topics; each message's topic is its type.
func TestReceived_should_take_the_topic_as_the_type(t *testing.T) {
	got := received(&paho.Publish{Topic: "lights/kitchen", Payload: []byte("on")}, time.Time{})

	if got.Type != "lights/kitchen" {
		t.Fatalf("type %q", got.Type)
	}
}
