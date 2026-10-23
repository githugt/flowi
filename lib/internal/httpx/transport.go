// SPDX-License-Identifier: Apache-2.0

package httpx

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

const (
	dialTimeout      = 30 * time.Second
	keepAlive        = 30 * time.Second
	idleConnTimeout  = 90 * time.Second
	maxIdleConns     = 100
	tlsHandshakeWait = 10 * time.Second
)

// ErrHTTP3Proxy is returned for HTTP/3 with a proxy configured: QUIC runs
// over UDP, which HTTP proxies do not carry, so the proxy would be skipped
// silently.
var ErrHTTP3Proxy = errors.New("HTTP/3 cannot go through a proxy; unset the proxy or choose another http.version")

// newRoundTripper returns the transport for o.Version: quic-go for "3", the
// standard library's otherwise.
func newRoundTripper(o Options) (http.RoundTripper, error) {
	if o.Version != "3" {
		return newTransport(o)
	}
	return newHTTP3Transport(o)
}

func newHTTP3Transport(o Options) (*http3.Transport, error) {
	if o.Proxy != nil {
		if c := o.Proxy; c.HTTPProxy != "" || c.HTTPSProxy != "" {
			return nil, ErrHTTP3Proxy
		}
	}
	roots, err := rootCAs(o.CAFiles)
	if err != nil {
		return nil, err
	}
	return &http3.Transport{
		TLSClientConfig:    &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS13}, // QUIC requires TLS 1.3
		QUICConfig:         &quic.Config{HandshakeIdleTimeout: dialTimeout},
		DisableCompression: true, // decoded by receive, like the other transports
	}, nil
}

func newTransport(o Options) (*http.Transport, error) {
	protocols, err := protocolsFor(o.Version)
	if err != nil {
		return nil, err
	}
	roots, err := rootCAs(o.CAFiles)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{Timeout: dialTimeout, KeepAlive: keepAlive}
	return &http.Transport{
		Proxy:               proxyFunc(o),
		DialContext:         dialer.DialContext,
		TLSClientConfig:     &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12},
		TLSHandshakeTimeout: tlsHandshakeWait,
		DisableCompression:  true, // decoded by receive, which also records raw sizes
		MaxIdleConns:        maxIdleConns,
		IdleConnTimeout:     idleConnTimeout,
		Protocols:           protocols,
	}, nil
}

// protocolsFor maps the http.version setting to the transport protocols.
func protocolsFor(version string) (*http.Protocols, error) {
	var p http.Protocols
	switch version {
	case "", "auto":
		p.SetHTTP1(true)
		p.SetHTTP2(true)
	case "1.1":
		p.SetHTTP1(true)
	case "2":
		p.SetHTTP2(true)
	case "h2c":
		p.SetUnencryptedHTTP2(true)
	default:
		return nil, fmt.Errorf("unknown HTTP version %q", version)
	}
	return &p, nil
}

func proxyFunc(o Options) func(*http.Request) (*url.URL, error) {
	if o.Proxy == nil {
		return nil
	}
	choose := o.Proxy.ProxyFunc()
	return func(r *http.Request) (*url.URL, error) { return choose(r.URL) }
}

// rootCAs returns the system pool plus the PEM certificates in files, or
// nil (the system pool) when there are none.
func rootCAs(files []string) (*x509.CertPool, error) {
	if len(files) == 0 {
		return nil, nil
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool() // no system pool (some minimal containers): trust only the given CAs
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("reading CA file %s: %w", f, err)
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("CA file %s contains no PEM certificates", f)
		}
	}
	return pool, nil
}

// TLSConfig is the client TLS configuration for the given extra CA files
// (the system pool alone when there are none).
func TLSConfig(caFiles []string) (*tls.Config, error) {
	roots, err := rootCAs(caFiles)
	if err != nil {
		return nil, err
	}
	return &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, nil
}
