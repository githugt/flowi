// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/vault"
)

func sampleRecord(secret string) Record {
	return Record{
		ID: "01JA00000000000000000000R1", RunID: "01JA00000000000000000000N1",
		CollectionID: "01JA0000000000000000000001", RequestID: "01JA0000000000000000000010",
		RequestPath: "auth/login", Version: "2", VersionSource: "preferred",
		At: time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC),
		Request: SentRequest{
			Method: "POST", URL: "https://api.example/v2/login?token=" + url.QueryEscape(secret),
			Header: []Header{{Name: "X-Key", Value: secret}},
			Body:   &Body{Text: `{"password":"` + secret + `"}`, SHA256: "aa", Size: 10},
		},
		Status: 302, Proto: "HTTP/2.0",
		Header:    []Header{{Name: "Content-Type", Value: "application/json"}},
		Body:      Body{Text: `{"echo":"` + secret + `"}`, JSON: map[string]any{"echo": secret, "list": []any{secret}}, SHA256: "bb", Size: 20, RawSize: 18, ContentType: "application/json"},
		Redirects: []Hop{{Status: 302, URL: "https://api.example/a", Location: "/b?s=" + secret, Header: []Header{{Name: "Location", Value: "/b?s=" + secret}}}},
		Timings:   Timings{DNS: time.Millisecond, Connect: 2 * time.Millisecond, TLS: 3 * time.Millisecond, TTFB: 4 * time.Millisecond, Total: 5 * time.Millisecond},
		Attempts:  []AttemptRecord{{Error: "dial tcp: " + secret, Duration: time.Millisecond, Wait: 200 * time.Millisecond}, {Status: 302, Duration: 5 * time.Millisecond}},
		Outputs:   map[string]any{"token": secret},
	}
}

func TestRecord_Masked_should_mask_every_string_field(t *testing.T) {
	const secret = `s3"cr/et`
	m := vault.NewMasker(secret)

	masked := sampleRecord(secret).Masked(m)

	data, err := json.Marshal(masked)
	if err != nil {
		t.Fatal(err)
	}
	escaped, _ := json.Marshal(secret)
	for _, form := range []string{secret, string(escaped[1 : len(escaped)-1]), url.QueryEscape(secret), base64.StdEncoding.EncodeToString([]byte(secret))} {
		if bytes.Contains(data, []byte(form)) {
			t.Fatalf("form %q present in %s", form, data)
		}
	}
	if !strings.Contains(string(data), vault.Mask) {
		t.Fatal("nothing masked")
	}
}

func TestRecord_Masked_should_not_modify_original(t *testing.T) {
	r := sampleRecord("secret")

	_ = r.Masked(vault.NewMasker("secret"))

	if r.Request.Header[0].Value != "secret" || r.Body.JSON.(map[string]any)["echo"] != "secret" {
		t.Fatal("original changed")
	}
}

func TestRecord_should_round_trip_through_JSON(t *testing.T) {
	golden, err := os.ReadFile("testdata/record.golden.json")
	if err != nil {
		t.Fatal(err)
	}
	r := sampleRecord("plain")

	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	var back Record
	if err := json.Unmarshal(golden, &back); err != nil {
		t.Fatal(err)
	}

	if string(data)+"\n" != string(golden) {
		t.Fatalf("encoding differs from golden:\n%s", data)
	}
	if !reflect.DeepEqual(back, r) {
		t.Fatalf("decoded golden differs:\n%+v\n%+v", back, r)
	}
}
