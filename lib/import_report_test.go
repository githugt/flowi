// SPDX-License-Identifier: Apache-2.0

package flowi

import "testing"

func TestVerbOfRequestFile_Given_a_message_request_Then_it_is_the_protocol_verb(t *testing.T) {
	cases := map[string]struct{ yaml, want string }{
		"sqs":  {"id: 01M817V5DJAFNAH69DHXEKDFBP\nname: Receive\nversions:\n  \"1\":\n    async:\n      protocol: sqs\n      server: http://localhost:4566\n      channel: q\n      receive: {}\n", "SQS"},
		"http": {"id: 01M817V5DJAFNAH69DHXEKDFBQ\nname: Get\nversions:\n  \"1\":\n    method: GET\n    url: http://localhost/x\n", "GET"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := verbOfRequestFile("requests/x.req.yaml", []byte(c.yaml))

			if got != c.want {
				t.Fatalf("verb %q, want %q", got, c.want)
			}
		})
	}
	if got := verbOfRequestFile("requests/x.req.yaml", nil); got != "" {
		t.Fatalf("verb of no data %q", got)
	}
}
