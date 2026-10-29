// SPDX-License-Identifier: Apache-2.0

package replay_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/serve/replay"
)

func TestRecord_Given_a_tape_Then_each_call_is_written_to_a_har_file_in_order_with_credentials_masked(t *testing.T) {
	// Arrange
	up := newUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Set-Cookie", "sid=secret-session")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			_, _ = io.WriteString(w, `{"id": 7, "access_token": "tok-123456"}`)
			return
		}
		_, _ = io.WriteString(w, `{"id": 7}`)
	})
	f := recordingFixture(t, up)
	file := filepath.Join(t.TempDir(), "session.har")
	f.options.Record.Tape = &replay.Tape{Path: file, Version: "9.9"}
	f.start()

	// Act
	f.do("POST", "/v1/login", "Authorization", "Bearer abc")
	f.do("GET", "/v1/users/7")

	// Assert
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var har struct {
		Log struct {
			Entries []struct {
				Request struct {
					Method string `json:"method"`
					URL    string `json:"url"`
				} `json:"request"`
				Response struct {
					Status int `json:"status"`
				} `json:"response"`
			} `json:"entries"`
		} `json:"log"`
	}
	if err := json.Unmarshal(data, &har); err != nil {
		t.Fatalf("not a HAR: %v\n%s", err, data)
	}
	got := har.Log.Entries
	if len(got) != 2 || got[0].Request.Method != "POST" || got[1].Request.Method != "GET" || got[0].Response.Status != 201 {
		t.Fatalf("entries %+v", got)
	}
	if !strings.HasPrefix(got[0].Request.URL, up.srv.URL+"/v1/login") {
		t.Errorf("the URL should be the upstream's, absolute: %s", got[0].Request.URL)
	}
	for _, secret := range []string{"Bearer abc", "secret-session", "tok-123456"} {
		if strings.Contains(string(data), secret) {
			t.Errorf("%q reached the tape:\n%s", secret, data)
		}
	}
}
