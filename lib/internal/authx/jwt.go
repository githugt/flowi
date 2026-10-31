// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/githugt/flowi/lib/internal/httpx"
)

// signedJWT builds a JWT from claims, signs it, and sends it as
// "Authorization: Bearer …" (or in the header named). HMAC algorithms take
// the secret; RSA, RSA-PSS and ECDSA take a PEM private key. With expires_in
// the claims get iat and exp; claims is a JSON object (templates are rendered
// before it gets here).
type signedJWT struct {
	method  jwt.SigningMethod
	key     any
	claims  jwt.MapClaims
	expires time.Duration
	header  string
	prefix  string
	env     Env
}

func newJWT(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	alg := strings.ToUpper(p.get("algorithm", "HS256"))
	method := jwt.GetSigningMethod(alg)
	if method == nil || alg == "NONE" {
		return nil, fmt.Errorf("jwt auth: algorithm %q is not supported (HS256/384/512, RS256/384/512, PS256/384/512, ES256/384/512, EdDSA)", alg)
	}
	key, err := jwtKey(alg, p)
	if err != nil {
		return nil, err
	}
	claims := jwt.MapClaims{}
	if text := strings.TrimSpace(p.values["claims"]); text != "" {
		if err := json.Unmarshal([]byte(text), &claims); err != nil {
			return nil, fmt.Errorf("jwt auth: claims must be a JSON object: %w", err)
		}
	}
	var expires time.Duration
	if v := p.values["expires_in"]; v != "" {
		if expires, err = time.ParseDuration(v); err != nil || expires <= 0 {
			return nil, fmt.Errorf("jwt auth: expires_in must be a duration like 5m, got %q", v)
		}
	}
	return &signedJWT{method: method, key: key, claims: claims, expires: expires, header: p.get("header", "Authorization"),
		prefix: p.get("prefix", "Bearer"), env: env}, nil
}

func jwtKey(alg string, p params) (any, error) {
	if strings.HasPrefix(alg, "HS") {
		if err := p.need("secret"); err != nil {
			return nil, fmt.Errorf("%w (the HMAC key)", err)
		}
		return []byte(p.values["secret"]), nil
	}
	if err := p.need("private_key"); err != nil {
		return nil, fmt.Errorf("%w (a PEM private key)", err)
	}
	pem := []byte(p.values["private_key"])
	var key any
	var err error
	switch {
	case strings.HasPrefix(alg, "RS"), strings.HasPrefix(alg, "PS"):
		key, err = jwt.ParseRSAPrivateKeyFromPEM(pem)
	case strings.HasPrefix(alg, "ES"):
		key, err = jwt.ParseECPrivateKeyFromPEM(pem)
	default:
		key, err = jwt.ParseEdPrivateKeyFromPEM(pem)
	}
	if err != nil {
		return nil, fmt.Errorf("jwt auth: the private key is not a valid %s key: %w", alg, err)
	}
	return key, nil
}

func (j *signedJWT) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	claims := make(jwt.MapClaims, len(j.claims)+2)
	for k, v := range j.claims {
		claims[k] = v
	}
	if j.expires > 0 {
		now := j.env.now()
		claims["iat"], claims["exp"] = now.Unix(), now.Add(j.expires).Unix()
	}
	signed, err := jwt.NewWithClaims(j.method, claims).SignedString(j.key)
	if err != nil {
		return nil, fmt.Errorf("signing the JWT: %w", err)
	}
	value := signed
	if strings.EqualFold(j.header, "Authorization") && j.prefix != "" {
		value = j.prefix + " " + signed
	}
	r.Header.Set(j.header, value)
	return []Applied{{Where: "header", Name: j.header, Value: value, Secret: true}}, nil
}
