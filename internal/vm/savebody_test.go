// SPDX-License-Identifier: AGPL-3.0-or-later

package vm_test

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/engine"

	"github.com/githugt/flowi/internal/vm"
)

func TestResponseBodyFile_should_give_the_bytes_of_the_response_and_a_name_to_save_it_under(t *testing.T) {
	download := filepath.Join(t.TempDir(), "spill.bin")
	if err := os.WriteFile(download, []byte("streamed"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		rec      engine.Record
		wantData string
		wantName string
	}{
		{"text", engine.Record{Request: engine.SentRequest{URL: "https://api.example.com/v1/pets"}, Body: engine.Body{Text: "hello", ContentType: "text/plain; charset=utf-8"}}, "hello", "pets.txt"},
		{"JSON keeps the name's own extension", engine.Record{Request: engine.SentRequest{URL: "https://example.com/data/export.json?x=1"}, Body: engine.Body{Text: `{"a":1}`, ContentType: "application/json"}}, `{"a":1}`, "export.json"},
		{"JSON without text", engine.Record{Request: engine.SentRequest{URL: "https://example.com/"}, Body: engine.Body{JSON: map[string]any{"a": 1}, ContentType: "application/json"}}, "{\n  \"a\": 1\n}", "response.json"},
		{"binary", engine.Record{Request: engine.SentRequest{URL: "https://example.com/logo"}, Body: engine.Body{Base64: base64.StdEncoding.EncodeToString([]byte{0, 1, 2, 255}), ContentType: "image/png"}}, "\x00\x01\x02\xff", "logo.png"},
		{"streamed download", engine.Record{Request: engine.SentRequest{URL: "https://example.com/files/big.zip"}, Body: engine.Body{File: download, ContentType: "application/zip"}}, "streamed", "big.zip"},
		{"unknown type", engine.Record{Request: engine.SentRequest{URL: "https://example.com/"}, Body: engine.Body{Text: "x"}}, "x", "response"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := vm.ResponseBodyFile(tc.rec)

			if err != nil {
				t.Fatal(err)
			}
			if string(got.Data) != tc.wantData || got.Name != tc.wantName {
				t.Errorf("data %q name %q, want %q and %q", got.Data, got.Name, tc.wantData, tc.wantName)
			}
		})
	}
}

func TestResponseBodyFile_should_refuse_a_response_with_no_body(t *testing.T) {
	_, err := vm.ResponseBodyFile(engine.Record{Request: engine.SentRequest{URL: "https://example.com/"}})

	if err == nil {
		t.Fatal("want an error: there is nothing to save")
	}
}

func TestResponseBodyFile_should_offer_only_a_plain_name_whatever_the_URL_says(t *testing.T) {
	for _, raw := range []string{"https://example.com/a/..%2F..%2Fetc%2Fpasswd", "https://example.com/..", "https://example.com/a%5Cb", "https://example.com/c:%5Cwin"} {
		got, err := vm.ResponseBodyFile(engine.Record{Request: engine.SentRequest{URL: raw}, Body: engine.Body{Text: "x"}})

		if err != nil || strings.ContainsAny(got.Name, `/\:`) || got.Name == ".." || got.Name == "" {
			t.Errorf("%s: name %q, err %v", raw, got.Name, err)
		}
	}
}
