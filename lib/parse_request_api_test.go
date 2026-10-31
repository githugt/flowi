// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"testing"
	"time"
)

const pastedAMQPRequest = `id: 01M63D04BXES8ME7EM4FSWYBEJ
name: Listen
versions:
  "1":
    async:
      protocol: amqp
      server: "amqp://127.0.0.1:5672"
      channel: "flowi.test"
      receive:
        count: 1
        timeout: "30s"
      binding:
        username: "flowi"
        password: "{{secret.broker_password}}"
        queue: "flowi.test"
`

func TestParseRequest_Given_a_request_file_Then_its_default_version_is_read_with_the_message_block(t *testing.T) {
	// Act
	req, diags := ParseRequest([]byte(pastedAMQPRequest), "pasted request")

	// Assert
	if diags.HasErrors() || len(req.Versions) != 1 {
		t.Fatalf("diags %v versions %d", diags, len(req.Versions))
	}
	a := req.Versions[0].Async
	if a == nil || a.Protocol != "amqp" || a.Server != "amqp://127.0.0.1:5672" || a.Channel != "flowi.test" {
		t.Fatalf("async %+v", a)
	}
	if a.Receive == nil || a.Receive.Count != 1 || a.Receive.Timeout != 30*time.Second || len(a.Binding) != 3 || a.Binding[1].Value != "{{secret.broker_password}}" {
		t.Fatalf("receive %+v binding %+v", a.Receive, a.Binding)
	}
}

func TestParseRequest_Given_text_that_is_not_a_request_Then_it_is_an_error_that_says_so(t *testing.T) {
	// Act
	_, diags := ParseRequest([]byte("just: some\nother: yaml\n"), "pasted request")

	// Assert
	if !diags.HasErrors() {
		t.Fatalf("diags %v", diags)
	}
}
