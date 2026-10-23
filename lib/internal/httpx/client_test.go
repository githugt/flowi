// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/klauspost/compress/zstd"
	"golang.org/x/net/http/httpproxy"
)

func defaultOptions() Options {
	return Options{Version: "auto", FollowRedirects: true, MaxRedirects: 10, Timeout: 5 * time.Second, StreamThreshold: 10 << 20}
}

func newClient(t *testing.T, o Options) *Client {
	t.Helper()
	c, err := NewClient(o)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func get(t *testing.T, c *Client, url string) Attempt {
	t.Helper()
	attempts, err := c.Do(context.Background(), Request{Method: http.MethodGet, URL: url, Header: http.Header{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(attempts) != 1 {
		t.Fatalf("%d attempts", len(attempts))
	}
	return attempts[0]
}

func TestClient_should_record_redirect_chain_when_following(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var n int
		if _, err := fmt.Sscanf(r.URL.Path, "/r/%d", &n); err == nil && n > 0 {
			http.Redirect(w, r, fmt.Sprintf("/r/%d", n-1), http.StatusFound)
			return
		}
		_, _ = io.WriteString(w, "done")
	}))
	defer srv.Close()

	a := get(t, newClient(t, defaultOptions()), srv.URL+"/r/3")

	if a.Err != nil || a.Response.Status != 200 || string(a.Response.Body) != "done" {
		t.Fatalf("got %+v", a)
	}
	if len(a.Response.Redirects) != 3 || a.Response.Redirects[0].Status != 302 ||
		!strings.HasSuffix(a.Response.Redirects[0].URL, "/r/3") || a.Response.Redirects[2].Location != "/r/0" {
		t.Fatalf("redirects %+v", a.Response.Redirects)
	}
}

func TestClient_Given_follow_false_Then_returns_3xx_with_location(t *testing.T) {
	srv := httptest.NewServer(http.RedirectHandler("/elsewhere", http.StatusMovedPermanently))
	defer srv.Close()
	o := defaultOptions()
	o.FollowRedirects = false

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || a.Response.Status != 301 || a.Response.Header.Get("Location") != "/elsewhere" {
		t.Fatalf("got %+v", a)
	}
}

func TestClient_Given_more_redirects_than_max_Then_attempt_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, r.URL.Path+"x", http.StatusFound)
	}))
	defer srv.Close()
	o := defaultOptions()
	o.MaxRedirects = 2

	a := get(t, newClient(t, o), srv.URL+"/")

	if a.Err == nil || !strings.Contains(a.Err.Error(), "2 redirects") {
		t.Fatalf("got %v", a.Err)
	}
}

func TestClient_should_decode_gzip_br_zstd_and_report_raw_and_decoded_size(t *testing.T) {
	plain := bytes.Repeat([]byte("flowi "), 500)
	encoders := map[string]func(io.Writer) io.WriteCloser{
		"gzip": func(w io.Writer) io.WriteCloser { return gzip.NewWriter(w) },
		"br":   func(w io.Writer) io.WriteCloser { return brotli.NewWriter(w) },
		"zstd": func(w io.Writer) io.WriteCloser { z, _ := zstd.NewWriter(w); return z },
	}
	for name, enc := range encoders {
		t.Run(name, func(t *testing.T) {
			var compressed bytes.Buffer
			zw := enc(&compressed)
			_, _ = zw.Write(plain)
			_ = zw.Close()
			var gotAccept string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotAccept = r.Header.Get("Accept-Encoding")
				w.Header().Set("Content-Encoding", name)
				_, _ = w.Write(compressed.Bytes())
			}))
			defer srv.Close()

			a := get(t, newClient(t, defaultOptions()), srv.URL)

			if a.Err != nil || !bytes.Equal(a.Response.Body, plain) || a.Response.Encoding != name {
				t.Fatalf("err %v encoding %q body %d bytes", a.Err, a.Response.Encoding, len(a.Response.Body))
			}
			if a.Response.RawSize != int64(compressed.Len()) || a.Response.Size != int64(len(plain)) {
				t.Fatalf("raw %d decoded %d", a.Response.RawSize, a.Response.Size)
			}
			if gotAccept != "gzip, deflate, br, zstd" {
				t.Fatalf("Accept-Encoding %q", gotAccept)
			}
		})
	}
}

func tlsCAFile(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ca.pem")
	data := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: srv.Certificate().Raw})
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func protoHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, r.Proto) })
}

func TestClient_Given_version_2_Then_uses_HTTP2_over_TLS(t *testing.T) {
	srv := httptest.NewUnstartedServer(protoHandler())
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	o := defaultOptions()
	o.Version, o.CAFiles = "2", []string{tlsCAFile(t, srv)}

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || string(a.Response.Body) != "HTTP/2.0" || a.Response.Proto != "HTTP/2.0" {
		t.Fatalf("got %+v", a)
	}
	if a.Response.Timings.TLS <= 0 {
		t.Fatalf("TLS timing %v", a.Response.Timings.TLS)
	}
}

func TestClient_Given_version_1_1_Then_HTTP1_against_h2_server(t *testing.T) {
	srv := httptest.NewUnstartedServer(protoHandler())
	srv.EnableHTTP2 = true
	srv.StartTLS()
	defer srv.Close()
	o := defaultOptions()
	o.Version, o.CAFiles = "1.1", []string{tlsCAFile(t, srv)}

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || string(a.Response.Body) != "HTTP/1.1" {
		t.Fatalf("got %+v", a)
	}
}

func TestClient_Given_h2c_Then_unencrypted_HTTP2(t *testing.T) {
	srv := httptest.NewUnstartedServer(protoHandler())
	srv.Config.Protocols = new(http.Protocols)
	srv.Config.Protocols.SetHTTP1(true)
	srv.Config.Protocols.SetUnencryptedHTTP2(true)
	srv.Start()
	defer srv.Close()
	o := defaultOptions()
	o.Version = "h2c"

	a := get(t, newClient(t, o), srv.URL)

	if a.Err != nil || string(a.Response.Body) != "HTTP/2.0" {
		t.Fatalf("got %+v", a)
	}
}

func TestClient_Given_custom_CA_file_Then_trusts_test_server(t *testing.T) {
	srv := httptest.NewTLSServer(protoHandler())
	defer srv.Close()
	withCA := defaultOptions()
	withCA.CAFiles = []string{tlsCAFile(t, srv)}

	trusted := get(t, newClient(t, withCA), srv.URL)
	untrusted := get(t, newClient(t, defaultOptions()), srv.URL)

	if trusted.Err != nil || untrusted.Err == nil {
		t.Fatalf("trusted %v, untrusted %v", trusted.Err, untrusted.Err)
	}
}

func TestNewClient_Given_bad_CA_file_Then_error_names_file(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.pem")
	_ = os.WriteFile(bad, []byte("not pem"), 0o600)
	o := defaultOptions()
	o.CAFiles = []string{bad}

	_, err := NewClient(o)

	if err == nil || !strings.Contains(err.Error(), "bad.pem") {
		t.Fatalf("got %v", err)
	}
}

func TestClient_Given_proxy_Then_request_goes_through_proxy(t *testing.T) {
	var hits atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "via proxy for "+r.URL.Host)
	}))
	defer proxy.Close()
	o := defaultOptions()
	o.Proxy = &httpproxy.Config{HTTPProxy: proxy.URL}

	a := get(t, newClient(t, o), "http://api.example/x")

	if a.Err != nil || string(a.Response.Body) != "via proxy for api.example" || hits.Load() != 1 {
		t.Fatalf("got %+v hits %d", a, hits.Load())
	}
}

func TestClient_should_fill_timings(t *testing.T) {
	srv := httptest.NewServer(protoHandler())
	defer srv.Close()

	a := get(t, newClient(t, defaultOptions()), srv.URL)

	if a.Err != nil || a.Response.Timings.TTFB <= 0 || a.Response.Timings.Total < a.Response.Timings.TTFB {
		t.Fatalf("timings %+v err %v", a.Response.Timings, a.Err)
	}
}

func TestClient_Given_slow_server_Then_TimeoutError(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	o := defaultOptions()
	o.Timeout = 100 * time.Millisecond

	a := get(t, newClient(t, o), srv.URL)

	var te *TimeoutError
	if !errors.As(a.Err, &te) || te.After != o.Timeout {
		t.Fatalf("got %v", a.Err)
	}
}

func TestClient_Given_cancelled_context_Then_Do_returns_context_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer srv.Close()
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, err := newClient(t, defaultOptions()).Do(ctx, Request{Method: http.MethodGet, URL: srv.URL, Header: http.Header{}})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}

func TestClient_Given_gzip_request_compression_Then_server_receives_gzip_body(t *testing.T) {
	var got []byte
	var encoding string
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		encoding = r.Header.Get("Content-Encoding")
		zr, err := gzip.NewReader(r.Body)
		if err == nil {
			got, _ = io.ReadAll(zr)
		}
	}))
	defer srv.Close()
	o := defaultOptions()
	o.RequestCompression = "gzip"

	attempts, err := newClient(t, o).Do(context.Background(), Request{
		Method: http.MethodPost, URL: srv.URL, Header: http.Header{}, Body: BytesBody([]byte(`{"a":1}`), "application/json"),
	})

	if err != nil || attempts[0].Err != nil || encoding != "gzip" || string(got) != `{"a":1}` {
		t.Fatalf("err %v %v encoding %q body %q", err, attempts[0].Err, encoding, got)
	}
}

func TestClient_errors_should_not_contain_url_query(t *testing.T) {
	var lc net.ListenConfig
	l, err := lc.Listen(context.Background(), "tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close() // a closed port: connection refused

	a := get(t, newClient(t, defaultOptions()), "http://"+addr+"/x?token=s3cret")

	if a.Err == nil || strings.Contains(a.Err.Error(), "s3cret") {
		t.Fatalf("got %v", a.Err)
	}
}

func TestClient_Given_empty_body_with_content_encoding_Then_empty_response_not_error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", r.URL.Query().Get("enc"))
		switch r.URL.Path {
		case "/204":
			w.WriteHeader(http.StatusNoContent)
		case "/304":
			w.WriteHeader(http.StatusNotModified)
		}
	}))
	defer srv.Close()
	c := newClient(t, defaultOptions())
	for _, tt := range []struct{ method, path, enc string }{
		{http.MethodHead, "/", "gzip"}, {http.MethodGet, "/204", "gzip"}, {http.MethodGet, "/304", "gzip"},
		{http.MethodGet, "/204", "deflate"}, {http.MethodGet, "/204", "zstd"}, {http.MethodGet, "/204", "br"},
	} {
		attempts, err := c.Do(context.Background(), Request{Method: tt.method, URL: srv.URL + tt.path + "?enc=" + tt.enc, Header: http.Header{}})

		if err != nil || attempts[0].Err != nil || attempts[0].Response.Size != 0 {
			t.Errorf("%s %s (%s): err %v attempt %+v", tt.method, tt.path, tt.enc, err, attempts[0])
		}
	}
}

func TestClient_Given_307_and_308_with_body_Then_follows_and_resends_body(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/307":
			http.Redirect(w, r, "/final", http.StatusTemporaryRedirect)
		case "/308":
			http.Redirect(w, r, "/final", http.StatusPermanentRedirect)
		default:
			data, _ := io.ReadAll(r.Body)
			_, _ = w.Write([]byte(r.Method + " " + string(data)))
		}
	}))
	defer srv.Close()
	for _, compression := range []string{"none", "gzip"} {
		o := defaultOptions()
		o.RequestCompression = compression
		for _, path := range []string{"/307", "/308"} {
			attempts, err := newClient(t, o).Do(context.Background(), Request{
				Method: http.MethodPost, URL: srv.URL + path, Header: http.Header{}, Body: BytesBody([]byte(`{"a":1}`), "application/json"),
			})

			a := attempts[0]
			if err != nil || a.Err != nil || a.Response.Status != 200 || len(a.Response.Redirects) != 1 {
				t.Fatalf("%s %s: err %v attempt %+v", compression, path, err, a)
			}
			if compression == "none" && string(a.Response.Body) != `POST {"a":1}` {
				t.Fatalf("%s: body %q", path, a.Response.Body)
			}
		}
	}
}
