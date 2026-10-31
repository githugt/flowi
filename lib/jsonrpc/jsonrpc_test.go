// SPDX-License-Identifier: Apache-2.0

package jsonrpc

import "testing"

func TestEnvelope_should_write_a_request_with_a_numeric_id_as_a_number(t *testing.T) {
	got := Envelope(Call{Method: "subtract", Params: `[42, 23]`, ID: "1"})

	want := "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"subtract\",\n  \"params\": [\n    42,\n    23\n  ],\n  \"id\": 1\n}"
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestEnvelope_should_quote_an_id_that_is_not_a_number_and_leave_out_params_and_id_when_empty(t *testing.T) {
	cases := map[string]struct {
		call Call
		want string
	}{
		"text id":         {Call{Method: "m", ID: "req-7"}, "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"m\",\n  \"id\": \"req-7\"\n}"},
		"notification":    {Call{Method: "m", Params: `{"a":1}`}, "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"m\",\n  \"params\": {\n    \"a\": 1\n  }\n}"},
		"template params": {Call{Method: "m", Params: `{"id": "{{id}}"}`, ID: "1"}, "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"m\",\n  \"params\": {\n    \"id\": \"{{id}}\"\n  },\n  \"id\": 1\n}"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Envelope(c.call); got != c.want {
				t.Fatalf("got\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

func TestEnvelope_Given_params_that_are_not_JSON_yet_Then_keeps_them_as_typed(t *testing.T) {
	got := Envelope(Call{Method: "m", Params: `{"id": {{id}}}`, ID: "1"})

	if want := "{\n  \"jsonrpc\": \"2.0\",\n  \"method\": \"m\",\n  \"params\": {\"id\": {{id}}},\n  \"id\": 1\n}"; got != want {
		t.Fatalf("got\n%s", got)
	}
}

func TestParse_should_read_back_what_Envelope_writes(t *testing.T) {
	for _, c := range []Call{
		{Method: "subtract", Params: "[\n  42,\n  23\n]", ID: "1"},
		{Method: "notify"},
		{Method: "m", Params: "{\n  \"a\": 1\n}", ID: "abc"},
	} {
		got, ok := Parse(Envelope(c))

		if !ok || got.Method != c.Method || got.ID != c.ID || compact(got.Params) != compact(c.Params) {
			t.Errorf("%+v came back as %+v (%v)", c, got, ok)
		}
	}
}

func TestParse_should_refuse_what_is_not_a_JSON_RPC_2_request(t *testing.T) {
	for _, text := range []string{``, `not json`, `[]`, `{"method":"m"}`, `{"jsonrpc":"1.0","method":"m"}`, `{"jsonrpc":"2.0"}`, `{"jsonrpc":"2.0","method":5}`, `{"jsonrpc":"2.0","method":"m","id":{"a":1}}`} {
		if _, ok := Parse(text); ok {
			t.Errorf("%q was taken for a request", text)
		}
	}
}

func compact(s string) string {
	out := ""
	for _, r := range s {
		if r != ' ' && r != '\n' {
			out += string(r)
		}
	}
	return out
}
