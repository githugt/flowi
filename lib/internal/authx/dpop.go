// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"strings"
	"sync"

	"github.com/golang-jwt/jwt/v5"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// dpop adds a DPoP proof (RFC 9449): a JWT signed with a key of the client's own, carrying the method and URL of this
// request (and a hash of the access token), so a stolen token is useless without the key. With an access_token it also
// sends "Authorization: DPoP <token>". A server that wants its own nonce answers 400 or 401 with DPoP-Nonce: Challenge
// keeps it and asks for one more send, which carries it.
type dpop struct {
	method  jwt.SigningMethod
	key     any
	jwk     map[string]any
	token   string
	env     Env
	mu      sync.Mutex
	nonce   string
	nonceOK bool // the nonce came from the server and goes into the next proof
}

func newDPoP(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("private_key"); err != nil {
		return nil, fmt.Errorf("%w (a PEM private key)", err)
	}
	alg := strings.ToUpper(p.get("algorithm", "ES256"))
	if !strings.HasPrefix(alg, "ES") && !strings.HasPrefix(alg, "RS") && !strings.HasPrefix(alg, "PS") && alg != "EDDSA" {
		return nil, fmt.Errorf("dpop auth: algorithm %q is not supported (ES256/384/512, RS256/384/512, PS256/384/512, EdDSA)", alg)
	}
	if alg == "EDDSA" {
		alg = "EdDSA"
	}
	method := jwt.GetSigningMethod(alg)
	if method == nil {
		return nil, fmt.Errorf("dpop auth: algorithm %q is not supported", alg)
	}
	key, err := parsePrivateKey(p.values["private_key"])
	if err != nil {
		return nil, errors.New(strings.Replace(err.Error(), "httpsig auth", "dpop auth", 1))
	}
	jwk, err := publicJWK(key)
	if err != nil {
		return nil, fmt.Errorf("dpop auth: %w", err)
	}
	token := p.values["access_token"]
	if strings.ContainsAny(token, " \t\r\n") {
		return nil, errors.New("dpop auth: the access token has a space in it")
	}
	return &dpop{method: method, key: key, jwk: jwk, token: token, env: env, nonce: p.values["nonce"], nonceOK: p.values["nonce"] != ""}, nil
}

// publicJWK is the public half of key as a JWK (RFC 7517): what a server needs to check the proof, and nothing private.
func publicJWK(key any) (map[string]any, error) {
	b64 := func(b []byte) string { return base64.RawURLEncoding.EncodeToString(b) }
	switch k := key.(type) {
	case *ecdsa.PrivateKey:
		point, err := k.PublicKey.Bytes() // uncompressed: 0x04, then x and y at the width of the curve
		if err != nil {
			return nil, fmt.Errorf("reading the public key: %w", err)
		}
		size := (len(point) - 1) / 2
		return map[string]any{"kty": "EC", "crv": k.Curve.Params().Name, "x": b64(point[1 : 1+size]), "y": b64(point[1+size:])}, nil
	case *rsa.PrivateKey:
		return map[string]any{"kty": "RSA", "n": b64(k.N.Bytes()), "e": b64(big.NewInt(int64(k.E)).Bytes())}, nil
	case ed25519.PrivateKey:
		return map[string]any{"kty": "OKP", "crv": "Ed25519", "x": b64(k.Public().(ed25519.PublicKey))}, nil
	}
	return nil, errors.New("the key is not an EC, RSA or Ed25519 private key")
}

func (d *dpop) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("dpop auth: the request URL cannot be read: %w", err)
	}
	// htu is the URL without its query and fragment.
	target := *u
	target.RawQuery, target.Fragment, target.RawFragment = "", "", ""
	claims := jwt.MapClaims{"jti": d.env.nonce(24), "htm": r.Method, "htu": target.String(), "iat": d.env.now().Unix()}
	if d.token != "" {
		sum := sha256.Sum256([]byte(d.token))
		claims["ath"] = base64.RawURLEncoding.EncodeToString(sum[:])
	}
	d.mu.Lock()
	if d.nonceOK {
		claims["nonce"] = d.nonce
	}
	d.mu.Unlock()
	tok := jwt.NewWithClaims(d.method, claims)
	tok.Header["typ"], tok.Header["jwk"] = "dpop+jwt", d.jwk
	proof, err := tok.SignedString(d.key)
	if err != nil {
		return nil, fmt.Errorf("dpop auth: signing the proof (does the key fit the algorithm?): %w", err)
	}
	r.Header.Set("DPoP", proof)
	applied := []Applied{{Where: "header", Name: "DPoP", Value: proof, Secret: true}}
	if d.token != "" {
		value := "DPoP " + d.token
		r.Header.Set("Authorization", value)
		applied = append(applied, Applied{Where: "header", Name: "Authorization", Value: value, Secret: true})
	}
	return applied, nil
}

// Challenge keeps the nonce a server asked for (RFC 9449 section 8) and says to send again. A response with no nonce, or
// one that is not a refusal, is not a challenge.
func (d *dpop) Challenge(resp *httpx.Response) bool {
	nonce := resp.Header.Get("DPoP-Nonce")
	if nonce == "" || (resp.Status != 400 && resp.Status != 401) {
		return false
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nonce, d.nonceOK = nonce, true
	return true
}
