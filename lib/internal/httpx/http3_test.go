// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
	"time"

	"github.com/quic-go/quic-go/http3"
	"golang.org/x/net/http/httpproxy"
)

// startHTTP3 serves handler over QUIC on a loopback UDP port with the
// certificate of a throwaway TLS test server, and returns the https URL and
// that server (for its CA).
func startHTTP3(t *testing.T, handler http.Handler) (string, *httptest.Server) {
	t.Helper()
	tlsSrv := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(tlsSrv.Close)
	pc, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	cfg := tlsSrv.TLS.Clone()
	cfg.NextProtos = []string{http3.NextProtoH3}
	srv := &http3.Server{Handler: handler, TLSConfig: cfg}
	go func() { _ = srv.Serve(pc) }() // ends when the cleanup closes the server
	t.Cleanup(func() { _ = srv.Close() })
	_, port, _ := net.SplitHostPort(pc.LocalAddr().String())
	return "https://127.0.0.1:" + port, tlsSrv
}

func TestClient_Given_version_3_Then_uses_HTTP3(t *testing.T) {
	url, tlsSrv := startHTTP3(t, protoHandler())
	o := defaultOptions()
	o.Version, o.CAFiles = "3", []string{tlsCAFile(t, tlsSrv)}

	a := get(t, newClient(t, o), url)

	if a.Err != nil || string(a.Response.Body) != "HTTP/3.0" || a.Response.Proto != "HTTP/3.0" {
		t.Fatalf("got %+v", a)
	}
}

func TestNewClient_Given_version_3_and_a_proxy_Then_refused(t *testing.T) {
	o := defaultOptions()
	o.Version = "3"
	o.Proxy = &httpproxy.Config{HTTPSProxy: "http://proxy.test:3128"}

	_, err := NewClient(o)

	if !errors.Is(err, ErrHTTP3Proxy) {
		t.Fatalf("HTTP/3 would have bypassed the proxy; got %v", err)
	}
}

func TestClient_Close_should_release_the_HTTP3_sockets(t *testing.T) {
	url, tlsSrv := startHTTP3(t, protoHandler())
	o := defaultOptions()
	o.Version, o.CAFiles = "3", []string{tlsCAFile(t, tlsSrv)}
	before := runtime.NumGoroutine()

	for range 20 {
		c := newClient(t, o)
		if a := get(t, c, url); a.Err != nil {
			t.Fatal(a.Err)
		}
		c.Close()
	}

	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before+5 {
		if time.Now().After(deadline) {
			t.Fatalf("goroutines %d → %d after 20 closed clients: HTTP/3 transports leak", before, runtime.NumGoroutine())
		}
		runtime.Gosched()
	}
}
