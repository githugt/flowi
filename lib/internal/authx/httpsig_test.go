// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/internal/httpx"
	"github.com/githugt/flowi/lib/model"
)

// The request of RFC 9421 appendix B.2.
func rfcRequest() *httpx.Request {
	r := request("POST", "https://example.com/foo?param=Value&Pet=dog")
	r.Header.Set("Host", "example.com")
	r.Header.Set("Date", "Tue, 20 Apr 2021 02:07:55 GMT")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Content-Digest", "sha-512=:WZDPaVn/7XgHaAy8pmojAkGWoRx2UFChF41A2svX+TaPm+AbwAgBWnrIiYllu7BNNyealdVLvRwEmTHWXvJwew==:")
	r.Header.Set("Content-Length", "18")
	r.Body = httpx.BytesBody([]byte(`{"hello": "world"}`), "application/json")
	return r
}

// rfcTime is the created time the RFC's examples use.
func rfcTime() time.Time { return time.Unix(1618884473, 0) }

func TestHTTPSig_Given_the_RFC_9421_hmac_example_Then_the_signature_is_the_one_in_the_RFC(t *testing.T) {
	// Arrange (appendix B.2.5)
	r := rfcRequest()
	a := auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "test-shared-secret", "label", "sig-b25",
		"secret", "base64:uzvJfB4u3N0Jy4T7NZ75MDVcr8zSTInedJtkgcu46YW4XByzNJjxBdtjUkdJPBtbmHhIDi6pcl8jsasjlTMtDQ==",
		"components", "date @authority content-type")

	// Act
	applied := mustApply(t, a, fixedEnv(rfcTime()), r)

	// Assert
	if got := r.Header.Get("Signature-Input"); got != `sig-b25=("date" "@authority" "content-type");created=1618884473;keyid="test-shared-secret"` {
		t.Errorf("Signature-Input %q", got)
	}
	if got := r.Header.Get("Signature"); got != "sig-b25=:pxcQw6G3AjtMBQjwo8XzkZf/bws5LelbaMk5rGIGtE8=:" {
		t.Errorf("Signature %q", got)
	}
	if len(applied) != 2 || !applied[1].Secret {
		t.Errorf("applied %+v", applied)
	}
}

func TestHTTPSig_Given_the_RFC_9421_ed25519_example_Then_the_signature_is_the_one_in_the_RFC(t *testing.T) {
	// Arrange (appendix B.2.6; ed25519 is deterministic)
	r := rfcRequest()
	a := auth(model.AuthHTTPSig, "algorithm", "ed25519", "key_id", "test-key-ed25519", "label", "sig-b26",
		"private_key", "-----BEGIN PRIVATE KEY-----\nMC4CAQAwBQYDK2VwBCIEIJ+DYvh6SEqVTm50DFtMDoQikTmiCqirVv9mWG9qfSnF\n-----END PRIVATE KEY-----\n",
		"components", "date @method @path @authority content-type content-length")

	// Act
	mustApply(t, a, fixedEnv(rfcTime()), r)

	// Assert
	if got := r.Header.Get("Signature-Input"); got != `sig-b26=("date" "@method" "@path" "@authority" "content-type" "content-length");created=1618884473;keyid="test-key-ed25519"` {
		t.Errorf("Signature-Input %q", got)
	}
	if got := r.Header.Get("Signature"); got != "sig-b26=:wqcAqbmYJ2ji2glfAMaRy4gruYYnx2nEFN2HN6jrnDnQCK1u02Gb04v9EDgwUPiu4A0w6vuQv5lIp5WPpBKRCw==:" {
		t.Errorf("Signature %q", got)
	}
}

func pemOf(t *testing.T, key any) string {
	t.Helper()
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

// signatureOf is the signature bytes of the header "label=:b64:".
func signatureOf(t *testing.T, header string) []byte {
	t.Helper()
	_, encoded, _ := strings.Cut(header, "=")
	raw, err := base64.StdEncoding.DecodeString(strings.Trim(encoded, ":"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

// signedBase is the base a verifier rebuilds for the request the signer saw: here the same three lines.
func signedBase(r *httpx.Request, params string) []byte {
	return []byte("\"@method\": " + r.Method + "\n\"@authority\": example.com\n\"@signature-params\": " + params)
}

func TestHTTPSig_Given_each_asymmetric_algorithm_Then_the_signature_verifies_with_the_public_key(t *testing.T) {
	components := "@method @authority"
	paramsFor := func(alg string) string {
		return `("@method" "@authority");created=1618884473;keyid="k";alg="` + alg + `"`
	}
	ecdsa256, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecdsa384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	tests := []struct {
		alg    string
		key    any
		verify func(base, sig []byte) bool
	}{
		{"ecdsa-p256-sha256", ecdsa256, func(base, sig []byte) bool {
			h := sha256.Sum256(base)
			return len(sig) == 64 && ecdsa.Verify(&ecdsa256.PublicKey, h[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:]))
		}},
		{"ecdsa-p384-sha384", ecdsa384, func(base, sig []byte) bool {
			h := sha512.Sum384(base)
			return len(sig) == 96 && ecdsa.Verify(&ecdsa384.PublicKey, h[:], new(big.Int).SetBytes(sig[:48]), new(big.Int).SetBytes(sig[48:]))
		}},
		{"rsa-pss-sha512", rsaKey, func(base, sig []byte) bool {
			h := sha512.Sum512(base)
			return rsa.VerifyPSS(&rsaKey.PublicKey, crypto.SHA512, h[:], sig, &rsa.PSSOptions{SaltLength: 64}) == nil
		}},
		{"rsa-v1_5-sha256", rsaKey, func(base, sig []byte) bool {
			h := sha256.Sum256(base)
			return rsa.VerifyPKCS1v15(&rsaKey.PublicKey, crypto.SHA256, h[:], sig) == nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.alg, func(t *testing.T) {
			r := request("GET", "https://example.com/x")
			a := auth(model.AuthHTTPSig, "algorithm", tt.alg, "key_id", "k", "private_key", pemOf(t, tt.key), "components", components, "include_alg", "true")

			mustApply(t, a, fixedEnv(rfcTime()), r)

			wantInput := "sig1=" + paramsFor(tt.alg)
			if got := r.Header.Get("Signature-Input"); got != wantInput {
				t.Fatalf("Signature-Input %q, want %q", got, wantInput)
			}
			if tt.alg == "rsa-v1_5-sha256" {
				h := sha256.Sum256(signedBase(r, paramsFor(tt.alg)))
				if err := rsa.VerifyPKCS1v15(&rsaKey.PublicKey, crypto.SHA256, h[:], signatureOf(t, r.Header.Get("Signature"))); err != nil {
					t.Fatalf("does not verify: %v", err)
				}
				return
			}
			if !tt.verify(signedBase(r, paramsFor(tt.alg)), signatureOf(t, r.Header.Get("Signature"))) {
				t.Fatal("the signature does not verify against the signature base")
			}
		})
	}
}

func TestHTTPSig_Given_content_digest_is_covered_and_missing_Then_it_is_added_from_the_body(t *testing.T) {
	r := request("POST", "https://example.com/x")
	r.Body = httpx.BytesBody([]byte(`{"a":1}`), "application/json")
	a := auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "s3cret", "components", "@method content-digest")

	mustApply(t, a, fixedEnv(rfcTime()), r)

	sum := sha256.Sum256([]byte(`{"a":1}`))
	if got, want := r.Header.Get("Content-Digest"), "sha-256=:"+base64.StdEncoding.EncodeToString(sum[:])+":"; got != want {
		t.Fatalf("Content-Digest %q, want %q", got, want)
	}
}

func TestHTTPSig_Given_options_Then_the_parameters_are_written_in_the_signature_input(t *testing.T) {
	r := request("GET", "https://example.com/x?a=b")
	a := auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "s3cret", "components", "@method @query @query-param:a",
		"expires_in", "5m", "tag", "app", "nonce", "true", "include_alg", "true")

	mustApply(t, a, fixedEnv(rfcTime()), r)

	want := `sig1=("@method" "@query" "@query-param";name="a");created=1618884473;expires=1618884773;keyid="k";alg="hmac-sha256";nonce="nnnnnnnnnnnnnnnn";tag="app"`
	if got := r.Header.Get("Signature-Input"); got != want {
		t.Fatalf("Signature-Input\n got %s\nwant %s", got, want)
	}
}

func TestHTTPSig_should_refuse_what_it_cannot_sign(t *testing.T) {
	tests := map[string]model.Auth{
		"no key id":          auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "secret", "s"),
		"hmac without key":   auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k"),
		"asymmetric no key":  auth(model.AuthHTTPSig, "algorithm", "ed25519", "key_id", "k"),
		"unknown algorithm":  auth(model.AuthHTTPSig, "algorithm", "md5", "key_id", "k", "secret", "s"),
		"wrong kind of key":  auth(model.AuthHTTPSig, "algorithm", "ed25519", "key_id", "k", "private_key", "-----BEGIN PRIVATE KEY-----\nbm90IGEga2V5\n-----END PRIVATE KEY-----"),
		"an unknown option":  auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "s", "colour", "red"),
		"no components":      auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "s", "components", " "),
		"a bad key encoding": auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "base64:!!!"),
	}
	for name, a := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := New(a, fixedEnv(rfcTime())); err == nil {
				t.Fatal("want an error")
			}
		})
	}
}

func TestHTTPSig_Given_a_covered_header_the_request_does_not_have_Then_it_says_which(t *testing.T) {
	a := auth(model.AuthHTTPSig, "algorithm", "hmac-sha256", "key_id", "k", "secret", "s", "components", "@method x-missing")
	s, err := New(a, fixedEnv(rfcTime()))
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Apply(t.Context(), request("GET", "https://example.com/"))

	if err == nil || !strings.Contains(err.Error(), "x-missing") {
		t.Fatalf("err %v", err)
	}
}
