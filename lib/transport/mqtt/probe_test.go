// SPDX-License-Identifier: Apache-2.0

package mqtt_test

import (
	"testing"

	"github.com/mochi-mqtt/server/v2/hooks/auth"

	"github.com/githugt/flowi/lib/transport"
	"github.com/githugt/flowi/lib/transport/mqtt"
)

func TestProbe_should_sign_in_without_publishing_for_both_protocol_levels(t *testing.T) {
	// Before: a wrong user or password showed only after a full send.
	_, server := broker(t, auth.AuthRules{{Username: "u", Password: "p", Allow: true}})
	for _, version := range []string{"5", "3.1.1"} {
		op := transport.Operation{Server: server, Channel: "t", Send: &transport.Message{Body: []byte("x")}, Binding: map[string]string{"version": version}}

		anonymous := transport.Probe(t.Context(), mqtt.New(), op)
		op.Binding["username"], op.Binding["password"] = "u", "p"
		signedIn := transport.Probe(t.Context(), mqtt.New(), op)

		if anonymous == nil || signedIn != nil {
			t.Errorf("version %s: anonymous %v, signed in %v", version, anonymous, signedIn)
		}
	}
}
