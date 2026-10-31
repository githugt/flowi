// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func deterministicBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i * 31 % 251)
	}
	return b
}

func TestClient_Given_body_over_threshold_Then_streams_to_file_with_hash(t *testing.T) {
	payload := deterministicBytes(2 << 20)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	o := defaultOptions()
	o.StreamThreshold, o.DownloadDir = 64<<10, filepath.Join(t.TempDir(), "downloads")

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || a.Response.Body != nil || a.Response.File == "" {
		t.Fatalf("got %+v", a)
	}
	data, err := os.ReadFile(a.Response.File)
	sum := sha256.Sum256(payload)
	if err != nil || !bytes.Equal(data, payload) || a.Response.SHA256 != hex.EncodeToString(sum[:]) || a.Response.Size != int64(len(payload)) {
		t.Fatalf("file %d bytes, err %v, hash %s", len(data), err, a.Response.SHA256)
	}
	if filepath.Dir(a.Response.File) != o.DownloadDir || filepath.Ext(a.Response.File) != ".pdf" {
		t.Fatalf("file %s", a.Response.File)
	}
}

func TestClient_Given_body_under_threshold_Then_in_memory_and_no_file(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("small")) }))
	defer srv.Close()
	o := defaultOptions()
	o.StreamThreshold, o.DownloadDir = 64<<10, filepath.Join(t.TempDir(), "downloads")

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || string(a.Response.Body) != "small" || a.Response.File != "" {
		t.Fatalf("got %+v", a)
	}
	if _, err := os.Stat(o.DownloadDir); !os.IsNotExist(err) {
		t.Fatalf("download dir created: %v", err)
	}
}

func TestClient_Given_body_exactly_at_threshold_Then_in_memory(t *testing.T) {
	payload := deterministicBytes(1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(payload) }))
	defer srv.Close()
	o := defaultOptions()
	o.StreamThreshold, o.DownloadDir = 1024, t.TempDir()

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || !bytes.Equal(a.Response.Body, payload) || a.Response.File != "" {
		t.Fatalf("got err %v file %q", a.Err, a.Response.File)
	}
}

func TestClient_Given_short_body_vs_content_length_Then_error_and_no_file_left(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()
		writeShortResponse(buf)
	}))
	defer srv.Close()
	o := defaultOptions()
	o.StreamThreshold, o.DownloadDir = 1024, filepath.Join(t.TempDir(), "downloads")

	a := get(t, newClient(t, o), srv.URL)

	if a.Err == nil {
		t.Fatal("want error for truncated body")
	}
	entries, _ := os.ReadDir(o.DownloadDir)
	if len(entries) != 0 {
		t.Fatalf("left files behind: %v", entries)
	}
}

func writeShortResponse(buf *bufio.ReadWriter) {
	_, _ = buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100000\r\nContent-Type: text/plain\r\n\r\n")
	_, _ = buf.WriteString(strings.Repeat("x", 5000))
	_ = buf.Flush()
}
