// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// selfSigned writes a certificate for 127.0.0.1 and its key into a temp folder and returns both paths and a client that trusts it.
func selfSigned(t *testing.T) (certFile, keyFile string, client *http.Client) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "flowi test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	certFile, keyFile = filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	return certFile, keyFile, &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}
}

func serveTriggers(t *testing.T, w *Workspace, mutate func(*ServeOptions)) (*Server, error) {
	t.Helper()
	o := ServeOptions{NoReplay: true, TriggerAddr: "127.0.0.1:0", ShutdownTimeout: 5 * time.Second}
	mutate(&o)
	ctx, cancel := context.WithCancel(context.Background())
	s, err := w.Serve(ctx, o)
	if err != nil {
		cancel()
		return nil, err
	}
	t.Cleanup(func() { cancel(); _ = s.Wait() })
	return s, nil
}

func TestServe_Given_a_trigger_address_open_to_the_network_without_TLS_Then_it_is_refused(t *testing.T) {
	// Before: the trigger server only warned, and the bearer token crossed the network in clear text.
	w := demoWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}

	_, err := serveTriggers(t, w, func(o *ServeOptions) { o.TriggerAddr = "0.0.0.0:0" })

	if !errors.Is(err, ErrInvalidServeOptions) || !strings.Contains(err.Error(), "--insecure-listen") {
		t.Fatalf("err %v", err)
	}
}

func TestServe_Given_insecure_listen_Then_the_exposed_address_is_allowed_and_a_note_says_it_is_plain_HTTP(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}

	s, err := serveTriggers(t, w, func(o *ServeOptions) { o.TriggerAddr = "0.0.0.0:0"; o.InsecureListen = true })

	if err != nil {
		t.Fatal(err)
	}
	notes := strings.Join(s.Notes(), "\n")
	if s.TriggerTLS() || !strings.Contains(notes, "plain HTTP") {
		t.Fatalf("tls %v, notes %q", s.TriggerTLS(), notes)
	}
}

func TestServe_Given_a_certificate_Then_the_trigger_endpoint_speaks_HTTPS_and_not_plain_HTTP(t *testing.T) {
	w := demoWorkspace(t)
	token, err := w.NewTriggerToken()
	if err != nil {
		t.Fatal(err)
	}
	cert, key, client := selfSigned(t)

	s, err := serveTriggers(t, w, func(o *ServeOptions) { o.TLSCert, o.TLSKey = cert, key })
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "https://"+s.TriggerAddr()+"/flows/no-such-flow/run", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	secure, secureErr := client.Do(req)
	plainReq, _ := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://"+s.TriggerAddr()+"/flows/no-such-flow/run", nil)
	plainReq.Header.Set("Content-Type", "text/plain")
	plain, plainErr := http.DefaultClient.Do(plainReq)

	if secureErr != nil || !s.TriggerTLS() || secure.StatusCode != http.StatusNotFound {
		t.Fatalf("secure %v %v tls %v", secure, secureErr, s.TriggerTLS())
	}
	secure.Body.Close()
	if plainErr == nil && plain.StatusCode < 400 {
		t.Errorf("plain HTTP was answered %d", plain.StatusCode)
	}
	if plain != nil {
		plain.Body.Close()
	}
}

func TestServe_Given_a_certificate_Then_an_exposed_address_needs_no_insecure_flag(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}
	cert, key, _ := selfSigned(t)

	_, err := serveTriggers(t, w, func(o *ServeOptions) { o.TriggerAddr = "0.0.0.0:0"; o.TLSCert, o.TLSKey = cert, key })

	if err != nil {
		t.Fatal(err)
	}
}

func TestServe_Given_a_certificate_without_a_key_or_unreadable_files_Then_it_is_invalid_options(t *testing.T) {
	w := demoWorkspace(t)
	if _, err := w.NewTriggerToken(); err != nil {
		t.Fatal(err)
	}
	cert, _, _ := selfSigned(t)
	tests := map[string]func(*ServeOptions){
		"certificate only": func(o *ServeOptions) { o.TLSCert = cert },
		"key only":         func(o *ServeOptions) { o.TLSKey = cert },
		"missing files":    func(o *ServeOptions) { o.TLSCert, o.TLSKey = "/no/cert.pem", "/no/key.pem" },
		"mismatched files": func(o *ServeOptions) { o.TLSCert, o.TLSKey = cert, cert },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := serveTriggers(t, w, mutate)

			if !errors.Is(err, ErrInvalidServeOptions) {
				t.Fatalf("err %v", err)
			}
		})
	}
}
