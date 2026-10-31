// SPDX-License-Identifier: Apache-2.0

package tapeplayer

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/githugt/flowi/lib/engine"
)

func record(method, url string, status int, body string) engine.Record {
	return engine.Record{
		Status: status, Proto: "HTTP/1.1", Request: engine.SentRequest{Method: method, URL: url},
		Header: []engine.Header{{Name: "Content-Type", Value: "application/json"}, {Name: "Content-Encoding", Value: "gzip"}},
		Body:   engine.Body{Text: body, Size: int64(len(body))},
	}
}

type answer struct {
	status int
	header http.Header
}

func get(t *testing.T, p *Player, method, url string) (answer, string, error) {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, url, nil)
	resp, err := p.RoundTrip(req)
	if err != nil {
		return answer{}, "", err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	return answer{status: resp.StatusCode, header: resp.Header}, string(data), nil
}

func TestPlayer_Given_the_same_call_twice_Then_the_recorded_answers_come_back_in_order(t *testing.T) {
	// Arrange
	p := New([]engine.Record{record("GET", "https://api.test/poll", 202, `{"n":1}`), record("GET", "https://api.test/poll", 200, `{"n":2}`)})

	// Act
	first, body1, err1 := get(t, p, "GET", "https://api.test/poll")
	second, body2, err2 := get(t, p, "GET", "https://api.test/poll")

	// Assert
	if err1 != nil || err2 != nil || first.status != 202 || second.status != 200 || body1 != `{"n":1}` || body2 != `{"n":2}` {
		t.Fatalf("%d %q %v / %d %q %v", first.status, body1, err1, second.status, body2, err2)
	}
}

func TestPlayer_Given_a_response_Then_headers_describe_the_decoded_body(t *testing.T) {
	// Arrange
	p := New([]engine.Record{record("GET", "https://api.test/a", 200, `hello`)})

	// Act
	resp, _, err := get(t, p, "GET", "https://api.test/a")

	// Assert
	if err != nil || resp.header.Get("Content-Encoding") != "" || resp.header.Get("Content-Length") != "5" || resp.header.Get("Content-Type") != "application/json" {
		t.Fatalf("%v %v", err, resp.header)
	}
}

func TestPlayer_Given_a_call_with_another_query_Then_the_same_path_still_answers_but_a_new_path_does_not(t *testing.T) {
	// Arrange: a timestamp in the query is the usual reason a replayed URL differs.
	p := New([]engine.Record{record("GET", "https://api.test/items?since=1", 200, `[]`)})

	// Act
	_, _, sameErr := get(t, p, "GET", "https://api.test/items?since=2")
	_, _, otherErr := get(t, p, "GET", "https://api.test/other")

	// Assert
	if sameErr != nil {
		t.Errorf("same path: %v", sameErr)
	}
	if !errors.Is(otherErr, ErrNotRecorded) {
		t.Errorf("other path: %v", otherErr)
	}
}

func TestPlayer_Given_a_json_and_a_binary_body_Then_both_are_served_as_bytes(t *testing.T) {
	// Arrange
	jsonRec := engine.Record{Status: 200, Request: engine.SentRequest{Method: "GET", URL: "https://api.test/j"}, Body: engine.Body{JSON: map[string]any{"a": 1.0}}}
	binRec := engine.Record{Status: 200, Request: engine.SentRequest{Method: "GET", URL: "https://api.test/b"}, Body: engine.Body{Base64: "AAEC"}}
	p := New([]engine.Record{jsonRec, binRec})

	// Act
	_, j, _ := get(t, p, "GET", "https://api.test/j")
	_, b, _ := get(t, p, "GET", "https://api.test/b")

	// Assert
	if j != `{"a":1}` || b != "\x00\x01\x02" {
		t.Fatalf("json %q binary %q", j, b)
	}
}
