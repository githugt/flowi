// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"net/http"
	"testing"
)

func TestCheckSample_should_judge_each_read_kind(t *testing.T) {
	body := map[string]any{
		"data":  map[string]any{"token": "t", "next": nil},
		"items": []any{map[string]any{"id": 1.0}},
		"empty": []any{},
		"n":     3.0,
	}
	headers := http.Header{"X-Request-Id": {"r"}, "Set-Cookie": {"sid=1; Path=/"}}
	full := Sample{Body: body, Headers: headers, HeadersKnown: true}
	cases := []struct {
		name   string
		read   Read
		sample Sample
		want   Status
	}{
		{"present path", body1("data", "token"), full, Compatible},
		{"missing key", body1("data", "gone"), full, Broken},
		{"array index past the end checks element 0", body1("items", "5", "id"), full, Compatible},
		{"missing key in array element", body1("items", "0", "name"), full, Broken},
		{"empty array cannot tell", body1("empty", "0", "id"), full, Unknown},
		{"null cannot tell", body1("data", "next", "x"), full, Unknown},
		{"scalar with steps left", body1("n", "x"), full, Broken},
		{"non-numeric step on array", body1("items", "id"), full, Broken},
		{"text body has no fields", body1("data"), Sample{Text: "ok"}, Broken},
		{"jsonpath match", Read{Kind: ReadJSONPath, Target: "$.items[*].id"}, full, Compatible},
		{"jsonpath miss", Read{Kind: ReadJSONPath, Target: "$.data.sub"}, full, Broken},
		{"invalid jsonpath cannot tell", Read{Kind: ReadJSONPath, Target: "$[?"}, full, Unknown},
		{"header any case", Read{Kind: ReadHeader, Target: "x-request-id"}, full, Compatible},
		{"header missing", Read{Kind: ReadHeader, Target: "X-Other"}, full, Broken},
		{"headers not recorded", Read{Kind: ReadHeader, Target: "X-Request-Id"}, Sample{Body: body}, Unknown},
		{"cookie set", Read{Kind: ReadCookie, Target: "sid"}, full, Compatible},
		{"cookie missing", Read{Kind: ReadCookie, Target: "other"}, full, Broken},
		{"regex over json text", Read{Kind: ReadRegex, Target: `"token":"(\w+)"`}, full, Compatible},
		{"regex miss", Read{Kind: ReadRegex, Target: `nope`}, Sample{Text: "ok"}, Broken},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := checkSample(tc.read, tc.sample)

			if got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}

func body1(steps ...string) Read {
	r, _ := bodyRead(steps)
	return r
}

func TestCheckRead_should_prefer_2xx_samples_and_take_the_best(t *testing.T) {
	ok := Sample{Status: 200, File: "/x/200.json", Body: map[string]any{"token": "t"}, Source: SourceExample}
	notFound := Sample{Status: 404, File: "/x/404.json", Body: map[string]any{"error": "e"}, Source: SourceExample}
	errorOnly := Contract{Source: SourceExample, Samples: []Sample{notFound}}
	both := Contract{Source: SourceExample, Samples: []Sample{notFound, ok}}

	inBoth := checkRead(body1("token"), both)
	errorRead := checkRead(body1("error"), both)
	onlyErrors := checkRead(body1("error"), errorOnly)

	if inBoth.Status != Compatible || errorRead.Status != Broken || onlyErrors.Status != Compatible {
		t.Fatalf("got %s/%s/%s, want compatible/broken/compatible", inBoth.Status, errorRead.Status, onlyErrors.Status)
	}
	if errorRead.Reason != "not in example 200.json" {
		t.Fatalf("reason %q", errorRead.Reason)
	}
}

func TestCheckRead_Given_a_schema_Then_the_reason_names_it(t *testing.T) {
	schema := Sample{Status: 200, File: "/x/getPet.schemas/1-200.json", Source: SourceSchema,
		Schema: map[string]any{"type": "object", "properties": map[string]any{"id": map[string]any{}}}}

	got := checkRead(body1("token"), Contract{Source: SourceSchema, Samples: []Sample{schema}})

	if got.Status != Broken || got.Reason != "not in schema 1-200.json" {
		t.Fatalf("got %+v", got)
	}
}

func TestCheckRead_Given_no_samples_Then_unknown(t *testing.T) {
	got := checkRead(body1("token"), Contract{})

	if got.Status != Unknown || got.Reason != "no contract data" {
		t.Fatalf("got %+v", got)
	}
}

func TestChecked_should_keep_up_front_statuses_and_take_the_worst(t *testing.T) {
	c := Consumer{Reads: []Read{body1("token"), {Kind: ReadInput, Target: "tenant", Status: Broken}}}
	contract := Contract{Source: SourceExample, Samples: []Sample{{Status: 200, Body: map[string]any{"token": "t"}}}}

	got := checked(c, contract)

	if got.Status != Broken || got.Reads[0].Status != Compatible || got.Reads[1].Status != Broken {
		t.Fatalf("got %+v", got)
	}
}

func TestChecked_Given_no_reads_Then_compatible_without_contract(t *testing.T) {
	got := checked(Consumer{}, Contract{})

	if got.Status != Compatible {
		t.Fatalf("got %s", got.Status)
	}
}

func TestCheckSample_against_a_schema(t *testing.T) {
	schema := map[string]any{"allOf": []any{
		map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}}},
		map[string]any{"type": "object", "properties": map[string]any{
			"id":    map[string]any{"type": "integer"},
			"tags":  map[string]any{"type": "array", "items": map[string]any{"type": "object", "properties": map[string]any{"label": map[string]any{"type": "string"}}}},
			"extra": map[string]any{},
		}},
	}}
	s := Sample{Schema: schema}
	cases := []struct {
		name string
		read Read
		want Status
	}{
		{"property from allOf", body1("name"), Compatible},
		{"removed property", body1("token"), Broken},
		{"array items", body1("tags", "0", "label"), Compatible},
		{"missing item property", body1("tags", "3", "colour"), Broken},
		{"anything schema", body1("extra", "deep"), Unknown},
		{"jsonpath", Read{Kind: ReadJSONPath, Target: "$.tags[0].label"}, Compatible},
		{"jsonpath missing", Read{Kind: ReadJSONPath, Target: "$.token"}, Broken},
		{"jsonpath filter", Read{Kind: ReadJSONPath, Target: "$..label"}, Unknown},
		{"header", Read{Kind: ReadHeader, Target: "X-Id"}, Unknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := checkSample(tc.read, s); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
