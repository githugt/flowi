// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
)

func harOf(t *testing.T, records []Record) map[string]any {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteHAR(&buf, records, "9.9"); err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, buf.String())
	}
	return out["log"].(map[string]any)
}

func TestWriteHAR_Given_an_http_record_Then_the_entry_carries_request_response_and_timings(t *testing.T) {
	// Arrange
	rec := Record{
		At: time.Date(2026, 10, 28, 9, 0, 0, 0, time.UTC), Proto: "HTTP/2.0", Status: 201,
		Request: SentRequest{
			Method: "POST", URL: "https://api.test/users?team=a%20b&x=1",
			Header: []Header{{Name: "Authorization", Value: "***"}},
			Body:   &Body{Text: `{"name":"ada"}`, ContentType: "application/json", Size: 14},
		},
		Header:  []Header{{Name: "Content-Type", Value: "application/json"}},
		Body:    Body{Text: `{"id":7}`, ContentType: "application/json", Size: 8},
		Timings: Timings{DNS: 2 * time.Millisecond, Connect: 3 * time.Millisecond, TLS: 4 * time.Millisecond, TTFB: 20 * time.Millisecond, Total: 30 * time.Millisecond},
	}

	// Act
	log := harOf(t, []Record{rec})

	// Assert
	entries := log["entries"].([]any)
	e := entries[0].(map[string]any)
	req, resp := e["request"].(map[string]any), e["response"].(map[string]any)
	if log["version"] != "1.2" || len(entries) != 1 || e["time"].(float64) != 30 {
		t.Fatalf("log %+v", log)
	}
	if req["method"] != "POST" || req["postData"].(map[string]any)["text"] != `{"name":"ada"}` {
		t.Errorf("request %+v", req)
	}
	query := req["queryString"].([]any)
	if len(query) != 2 || query[0].(map[string]any)["value"] != "a b" {
		t.Errorf("query %+v", query)
	}
	if resp["status"].(float64) != 201 || resp["content"].(map[string]any)["text"] != `{"id":7}` {
		t.Errorf("response %+v", resp)
	}
}

func TestWriteHAR_Given_a_message_record_Then_it_is_left_out_and_the_file_is_still_valid(t *testing.T) {
	// Arrange
	records := []Record{{Kind: KindAsync, Request: SentRequest{Method: "mqtt", URL: "mqtt://broker/t"}}}

	// Act
	log := harOf(t, records)

	// Assert
	if entries := log["entries"].([]any); len(entries) != 0 {
		t.Fatalf("entries %+v", entries)
	}
}

func TestWriteHAR_Given_a_binary_body_Then_it_is_written_as_base64(t *testing.T) {
	// Arrange
	rec := Record{Status: 200, Request: SentRequest{Method: "GET", URL: "https://api.test/f"}, Body: Body{Base64: "AAEC", ContentType: "application/octet-stream", Size: 3}}

	// Act
	log := harOf(t, []Record{rec})

	// Assert
	content := log["entries"].([]any)[0].(map[string]any)["response"].(map[string]any)["content"].(map[string]any)
	if content["encoding"] != "base64" || content["text"] != "AAEC" {
		t.Fatalf("content %+v", content)
	}
}
