// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"

	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
)

// verifiedProof parses a DPoP proof and checks its signature with the public key it carries in its own header, as a
// server does; it returns the header and claims.
func verifiedProof(t *testing.T, proof string) (map[string]any, jwt.MapClaims) {
	t.Helper()
	var header map[string]any
	claims := jwt.MapClaims{}
	_, err := jwt.ParseWithClaims(proof, claims, func(tok *jwt.Token) (any, error) {
		header = tok.Header
		jwk, _ := tok.Header["jwk"].(map[string]any)
		return publicKeyOf(t, jwk), nil
	}, jwt.WithoutClaimsValidation())
	if err != nil {
		t.Fatalf("the proof does not verify: %v", err)
	}
	return header, claims
}

func publicKeyOf(t *testing.T, jwk map[string]any) any {
	t.Helper()
	b64 := func(name string) []byte {
		raw, err := base64.RawURLEncoding.DecodeString(jwk[name].(string))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	switch jwk["kty"] {
	case "EC":
		curve := map[string]elliptic.Curve{"P-256": elliptic.P256(), "P-384": elliptic.P384()}[jwk["crv"].(string)]
		point := append(append([]byte{4}, b64("x")...), b64("y")...) // the uncompressed point, as the JWK's x and y make it
		key, err := ecdsa.ParseUncompressedPublicKey(curve, point)
		if err != nil {
			t.Fatal(err)
		}
		return key
	case "RSA":
		return &rsa.PublicKey{N: new(big.Int).SetBytes(b64("n")), E: int(new(big.Int).SetBytes(b64("e")).Int64())}
	case "OKP":
		return ed25519.PublicKey(b64("x"))
	}
	t.Fatalf("jwk %v", jwk)
	return nil
}

func TestDPoP_Given_an_access_token_Then_the_proof_binds_method_url_and_token_and_verifies(t *testing.T) {
	// Arrange
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	r := request("POST", "https://api.example.com/resource?a=b#frag")
	a := auth(model.AuthDPoP, "private_key", pemOf(t, key), "access_token", "Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU")

	// Act
	mustApply(t, a, fixedEnv(rfcTime()), r)

	// Assert
	header, claims := verifiedProof(t, r.Header.Get("DPoP"))
	sum := sha256.Sum256([]byte("Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU"))
	if header["typ"] != "dpop+jwt" || header["alg"] != "ES256" {
		t.Errorf("header %v", header)
	}
	if claims["htm"] != "POST" || claims["htu"] != "https://api.example.com/resource" {
		t.Errorf("htm/htu %v %v", claims["htm"], claims["htu"])
	}
	if claims["iat"] != float64(1618884473) || claims["jti"] == "" || claims["jti"] == nil {
		t.Errorf("iat/jti %v %v", claims["iat"], claims["jti"])
	}
	if claims["ath"] != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Errorf("ath %v", claims["ath"])
	}
	if got := r.Header.Get("Authorization"); got != "DPoP Kz~8mXK1EalYznwH-LC-1fBAo.4Ljp~zsPE_NeO.gxU" {
		t.Errorf("Authorization %q", got)
	}
	if _, hasPrivate := header["jwk"].(map[string]any)["d"]; hasPrivate {
		t.Error("the private part of the key must never be in the proof")
	}
}

func TestDPoP_Given_each_key_type_Then_the_proof_verifies(t *testing.T) {
	ec384, _ := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	rsaKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	_, edKey, _ := ed25519.GenerateKey(rand.Reader)
	tests := []struct {
		alg string
		key any
	}{{"ES384", ec384}, {"RS256", rsaKey}, {"PS256", rsaKey}, {"EdDSA", edKey}}
	for _, tt := range tests {
		t.Run(tt.alg, func(t *testing.T) {
			r := request("GET", "https://api.example.com/x")
			a := auth(model.AuthDPoP, "private_key", pemOf(t, tt.key), "algorithm", tt.alg)

			mustApply(t, a, fixedEnv(rfcTime()), r)

			header, claims := verifiedProof(t, r.Header.Get("DPoP"))
			if header["alg"] != tt.alg || claims["htm"] != "GET" {
				t.Fatalf("header %v claims %v", header, claims)
			}
			if _, has := claims["ath"]; has || r.Header.Get("Authorization") != "" {
				t.Fatal("without an access token there is no ath and no Authorization")
			}
		})
	}
}

func TestDPoP_Given_a_server_nonce_challenge_Then_the_retry_carries_the_nonce(t *testing.T) {
	// Arrange
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	counter := 0
	env := fixedEnv(rfcTime())
	env.Nonce = func(n int) string { counter++; return strings.Repeat(string(rune('a'+counter)), n) } // a new value each time, as a random one is
	s, err := New(auth(model.AuthDPoP, "private_key", pemOf(t, key)), env)
	if err != nil {
		t.Fatal(err)
	}
	first := request("POST", "https://as.example.com/token")
	if _, err := s.Apply(t.Context(), first); err != nil {
		t.Fatal(err)
	}
	_, firstClaims := verifiedProof(t, first.Header.Get("DPoP"))

	// Act
	retry := s.(Challenger).Challenge(&httpx.Response{Status: 400, Header: http.Header{"Dpop-Nonce": {"eyJ7S_zG.eyJH0-Z.HX4w-7v"}}})
	second := request("POST", "https://as.example.com/token")
	_, err = s.Apply(t.Context(), second)

	// Assert
	_, secondClaims := verifiedProof(t, second.Header.Get("DPoP"))
	if err != nil || !retry || firstClaims["nonce"] != nil || secondClaims["nonce"] != "eyJ7S_zG.eyJH0-Z.HX4w-7v" {
		t.Fatalf("err %v retry %v first %v second %v", err, retry, firstClaims["nonce"], secondClaims["nonce"])
	}
	if firstClaims["jti"] == secondClaims["jti"] {
		t.Fatal("each proof needs its own jti")
	}
}

func TestDPoP_Given_a_response_without_a_nonce_Then_no_retry(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s, _ := New(auth(model.AuthDPoP, "private_key", pemOf(t, key)), fixedEnv(rfcTime()))

	for _, resp := range []*httpx.Response{
		{Status: 401, Header: http.Header{}},
		{Status: 200, Header: http.Header{"Dpop-Nonce": {"n"}}},
	} {
		if s.(Challenger).Challenge(resp) {
			t.Errorf("status %d asked for a retry", resp.Status)
		}
	}
}

func TestDPoP_should_refuse_what_it_cannot_sign_with(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tests := map[string]model.Auth{
		"no key":              auth(model.AuthDPoP),
		"unknown algorithm":   auth(model.AuthDPoP, "private_key", pemOf(t, key), "algorithm", "HS256"),
		"an unknown option":   auth(model.AuthDPoP, "private_key", pemOf(t, key), "colour", "red"),
		"not PEM":             auth(model.AuthDPoP, "private_key", "nope"),
		"key and alg differ":  auth(model.AuthDPoP, "private_key", pemOf(t, key), "algorithm", "RS256"),
		"a token with spaces": auth(model.AuthDPoP, "private_key", pemOf(t, key), "access_token", "a b"),
	}
	for name, a := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := New(a, fixedEnv(rfcTime()))
			if err == nil && !strings.Contains(name, "differ") {
				t.Fatal("want an error")
			}
			if err == nil {
				// A key that does not fit the algorithm shows when signing.
				s, _ := New(a, fixedEnv(rfcTime()))
				if _, err = s.Apply(t.Context(), request("GET", "https://x/")); err == nil {
					t.Fatal("want an error")
				}
			}
		})
	}
}

func TestDPoP_should_encode_json_claims_the_way_the_spec_names_them(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	r := request("GET", "https://api.example.com/x")
	mustApply(t, auth(model.AuthDPoP, "private_key", pemOf(t, key), "nonce", "abc"), fixedEnv(rfcTime()), r)

	_, claims := verifiedProof(t, r.Header.Get("DPoP"))

	raw, _ := json.Marshal(claims)
	for _, name := range []string{`"htm"`, `"htu"`, `"iat"`, `"jti"`, `"nonce"`} {
		if !strings.Contains(string(raw), name) {
			t.Errorf("claims %s lack %s", raw, name)
		}
	}
}
