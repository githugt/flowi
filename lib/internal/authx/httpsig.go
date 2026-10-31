// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"hash"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/githugt/flowi-lib/internal/httpx"
)

// httpSig signs a request as RFC 9421 (HTTP Message Signatures) says: it builds the signature base from the covered
// components, signs it and sends Signature-Input and Signature. Content-Digest (RFC 9530) is added when it is covered and the
// request has none, and Date likewise.
type httpSig struct {
	alg        string
	keyID      string
	label      string
	components []component
	sign       func(base []byte) ([]byte, error)
	includeAlg bool
	expires    time.Duration
	useNonce   bool
	tag        string
	digest     string
	env        Env
}

// component is one covered part of the message: a derived component ("@method") or a header field, and for
// "@query-param" the parameter's name.
type component struct{ name, param string }

func (c component) id() string {
	if c.param != "" {
		return `"` + c.name + `";name="` + c.param + `"`
	}
	return `"` + c.name + `"`
}

// signAlgorithm is what a registered algorithm needs: how to sign a base and whether its key is a secret or a PEM key.
type signAlgorithm struct {
	symmetric bool
	build     func(key any) (func(base []byte) ([]byte, error), error)
}

func algorithmFor(alg string) (signAlgorithm, bool) {
	switch alg {
	case "hmac-sha256":
		return signAlgorithm{symmetric: true, build: func(key any) (func([]byte) ([]byte, error), error) {
			return hmacSigner(key.([]byte), sha256.New), nil
		}}, true
	case "ed25519":
		return signAlgorithm{build: func(key any) (func([]byte) ([]byte, error), error) {
			k, ok := key.(ed25519.PrivateKey)
			if !ok {
				return nil, errors.New("the key is not an Ed25519 key")
			}
			return func(base []byte) ([]byte, error) { return ed25519.Sign(k, base), nil }, nil
		}}, true
	case "ecdsa-p256-sha256":
		return signAlgorithm{build: ecdsaSigner(32, func(b []byte) []byte { h := sha256.Sum256(b); return h[:] })}, true
	case "ecdsa-p384-sha384":
		return signAlgorithm{build: ecdsaSigner(48, func(b []byte) []byte { h := sha512.Sum384(b); return h[:] })}, true
	case "rsa-pss-sha512":
		return signAlgorithm{build: rsaSigner(func(k *rsa.PrivateKey, base []byte) ([]byte, error) {
			h := sha512.Sum512(base)
			return rsa.SignPSS(rand.Reader, k, crypto.SHA512, h[:], &rsa.PSSOptions{SaltLength: 64})
		})}, true
	case "rsa-v1_5-sha256":
		return signAlgorithm{build: rsaSigner(func(k *rsa.PrivateKey, base []byte) ([]byte, error) {
			h := sha256.Sum256(base)
			return rsa.SignPKCS1v15(rand.Reader, k, crypto.SHA256, h[:])
		})}, true
	}
	return signAlgorithm{}, false
}

func hmacSigner(key []byte, h func() hash.Hash) func([]byte) ([]byte, error) {
	return func(base []byte) ([]byte, error) {
		mac := hmac.New(h, key)
		mac.Write(base)
		return mac.Sum(nil), nil
	}
}

// ecdsaSigner signs with ECDSA and writes the signature as the RFC wants it: r and s, each as a fixed-width integer, joined.
func ecdsaSigner(size int, digest func([]byte) []byte) func(key any) (func([]byte) ([]byte, error), error) {
	return func(key any) (func([]byte) ([]byte, error), error) {
		k, ok := key.(*ecdsa.PrivateKey)
		if !ok || (k.Curve.Params().BitSize+7)/8 != size {
			return nil, errors.New("the key is not an ECDSA key of the curve this algorithm needs")
		}
		return func(base []byte) ([]byte, error) {
			r, s, err := ecdsa.Sign(rand.Reader, k, digest(base))
			if err != nil {
				return nil, err
			}
			out := make([]byte, 2*size)
			r.FillBytes(out[:size])
			s.FillBytes(out[size:])
			return out, nil
		}, nil
	}
}

func rsaSigner(sign func(*rsa.PrivateKey, []byte) ([]byte, error)) func(key any) (func([]byte) ([]byte, error), error) {
	return func(key any) (func([]byte) ([]byte, error), error) {
		k, ok := key.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("the key is not an RSA key")
		}
		return func(base []byte) ([]byte, error) { return sign(k, base) }, nil
	}
}

func newHTTPSig(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("key_id"); err != nil {
		return nil, err
	}
	alg := strings.ToLower(p.get("algorithm", "hmac-sha256"))
	algorithm, ok := algorithmFor(alg)
	if !ok {
		return nil, fmt.Errorf("httpsig auth: algorithm %q is not supported (hmac-sha256, ed25519, ecdsa-p256-sha256, ecdsa-p384-sha384, rsa-pss-sha512, rsa-v1_5-sha256)", alg)
	}
	key, err := signingKey(algorithm, p)
	if err != nil {
		return nil, err
	}
	sign, err := algorithm.build(key)
	if err != nil {
		return nil, fmt.Errorf("httpsig auth: %w", err)
	}
	s := &httpSig{alg: alg, keyID: p.values["key_id"], label: p.get("label", "sig1"), sign: sign, env: env,
		includeAlg: strings.EqualFold(p.values["include_alg"], "true"), useNonce: strings.EqualFold(p.values["nonce"], "true"), tag: p.values["tag"]}
	if s.digest, err = digestAlgorithm(p.get("digest", "sha-256")); err != nil {
		return nil, err
	}
	if s.components, err = componentsOf(p.get("components", "@method @authority @path")); err != nil {
		return nil, err
	}
	if v := p.values["expires_in"]; v != "" {
		if s.expires, err = time.ParseDuration(v); err != nil || s.expires <= 0 {
			return nil, fmt.Errorf("httpsig auth: expires_in must be a duration like 5m, got %q", v)
		}
	}
	return s, nil
}

// signingKey is the HMAC secret or the parsed private key. A secret that starts with "base64:" is decoded.
func signingKey(a signAlgorithm, p params) (any, error) {
	if a.symmetric {
		if err := p.need("secret"); err != nil {
			return nil, fmt.Errorf("%w (the shared secret)", err)
		}
		secret := p.values["secret"]
		if encoded, ok := strings.CutPrefix(secret, "base64:"); ok {
			raw, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("httpsig auth: the secret after base64: is not base64: %w", err)
			}
			return raw, nil
		}
		return []byte(secret), nil
	}
	if err := p.need("private_key"); err != nil {
		return nil, fmt.Errorf("%w (a PEM private key)", err)
	}
	return parsePrivateKey(p.values["private_key"])
}

// parsePrivateKey reads a PEM private key in PKCS#8, SEC1 (EC) or PKCS#1 (RSA) form.
func parsePrivateKey(text string) (any, error) {
	block, _ := pem.Decode([]byte(text))
	if block == nil {
		return nil, errors.New("httpsig auth: the private key is not PEM")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	return nil, errors.New("httpsig auth: the private key is not a PKCS#8, SEC1 or PKCS#1 key")
}

func digestAlgorithm(name string) (string, error) {
	switch strings.ToLower(name) {
	case "sha-256", "sha-512":
		return strings.ToLower(name), nil
	}
	return "", fmt.Errorf("httpsig auth: digest %q is not sha-256 or sha-512", name)
}

// componentsOf reads the covered components: names separated by spaces or commas; "@query-param:name" covers one parameter.
func componentsOf(text string) ([]component, error) {
	var out []component
	for _, token := range strings.FieldsFunc(text, func(r rune) bool { return r == ' ' || r == ',' || r == '\n' || r == '\t' }) {
		name, param, _ := strings.Cut(token, ":")
		name = strings.ToLower(strings.Trim(name, `"`))
		if name == "@query-param" && param == "" {
			return nil, errors.New("httpsig auth: @query-param needs a name: write @query-param:name")
		}
		if name != "@query-param" {
			param = ""
		}
		out = append(out, component{name: name, param: param})
	}
	if len(out) == 0 {
		return nil, errors.New("httpsig auth: components lists nothing to sign")
	}
	return out, nil
}

func (s *httpSig) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("httpsig auth: the request URL cannot be read: %w", err)
	}
	body, err := bodyBytes(r)
	if err != nil {
		return nil, err
	}
	now := s.env.now()
	if err := s.addMissingHeaders(r, body, now); err != nil {
		return nil, err
	}
	lines, err := s.baseLines(r, u, body)
	if err != nil {
		return nil, err
	}
	params := s.signatureParams(now)
	base := strings.Join(append(lines, `"@signature-params": `+params), "\n")
	signature, err := s.sign([]byte(base))
	if err != nil {
		return nil, fmt.Errorf("httpsig auth: signing: %w", err)
	}
	input := s.label + "=" + params
	value := s.label + "=:" + base64.StdEncoding.EncodeToString(signature) + ":"
	r.Header.Set("Signature-Input", input)
	r.Header.Set("Signature", value)
	return []Applied{
		{Where: "header", Name: "Signature-Input", Value: input},
		{Where: "header", Name: "Signature", Value: value, Secret: true},
	}, nil
}

// addMissingHeaders sets the headers a covered component needs that the request does not carry yet: Content-Digest
// from the body, and Date from the clock.
func (s *httpSig) addMissingHeaders(r *httpx.Request, body []byte, now time.Time) error {
	for _, c := range s.components {
		switch {
		case c.name == "content-digest" && r.Header.Get("Content-Digest") == "":
			r.Header.Set("Content-Digest", contentDigest(s.digest, body))
		case c.name == "date" && r.Header.Get("Date") == "":
			r.Header.Set("Date", now.UTC().Format(http.TimeFormat))
		}
	}
	return nil
}

func contentDigest(alg string, body []byte) string {
	if alg == "sha-512" {
		sum := sha512.Sum512(body)
		return "sha-512=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
	}
	sum := sha256.Sum256(body)
	return "sha-256=:" + base64.StdEncoding.EncodeToString(sum[:]) + ":"
}

// baseLines are the lines of the signature base for the covered components, in order.
func (s *httpSig) baseLines(r *httpx.Request, u *url.URL, body []byte) ([]string, error) {
	lines := make([]string, 0, len(s.components)+1)
	for _, c := range s.components {
		value, err := componentValue(c, r, u, body)
		if err != nil {
			return nil, err
		}
		lines = append(lines, c.id()+": "+value)
	}
	return lines, nil
}

func componentValue(c component, r *httpx.Request, u *url.URL, body []byte) (string, error) {
	switch c.name {
	case "@method":
		return r.Method, nil
	case "@target-uri":
		return u.String(), nil
	case "@authority":
		return authorityOf(u), nil
	case "@scheme":
		return strings.ToLower(u.Scheme), nil
	case "@request-target":
		return requestTarget(u), nil
	case "@path":
		return pathOf(u), nil
	case "@query":
		return "?" + u.RawQuery, nil
	case "@query-param":
		values := u.Query()[c.param]
		if len(values) == 0 {
			return "", fmt.Errorf("httpsig auth: the query has no parameter %q to sign", c.param)
		}
		return url.QueryEscape(values[0]), nil
	case "content-length":
		if v := r.Header.Get("Content-Length"); v != "" {
			return strings.TrimSpace(v), nil
		}
		if r.Body != nil {
			return strconv.Itoa(len(body)), nil
		}
	}
	if strings.HasPrefix(c.name, "@") {
		return "", fmt.Errorf("httpsig auth: component %q is not one this can sign", c.name)
	}
	return headerValue(r, c.name)
}

func headerValue(r *httpx.Request, name string) (string, error) {
	values := r.Header.Values(name)
	if len(values) == 0 {
		return "", fmt.Errorf("httpsig auth: the request has no header %q to sign", name)
	}
	trimmed := make([]string, len(values))
	for i, v := range values {
		trimmed[i] = strings.TrimSpace(v)
	}
	return strings.Join(trimmed, ", "), nil
}

// authorityOf is the host with its port, lower case, without the port the scheme implies.
func authorityOf(u *url.URL) string {
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if port == "" || (port == "80" && u.Scheme == "http") || (port == "443" && u.Scheme == "https") {
		return host
	}
	return host + ":" + port
}

func pathOf(u *url.URL) string {
	if p := u.EscapedPath(); p != "" {
		return p
	}
	return "/"
}

func requestTarget(u *url.URL) string {
	if u.RawQuery != "" {
		return pathOf(u) + "?" + u.RawQuery
	}
	return pathOf(u)
}

// signatureParams is the inner list of covered components with the signature's parameters: the same text is the last line
// of the base and the value of Signature-Input.
func (s *httpSig) signatureParams(now time.Time) string {
	ids := make([]string, len(s.components))
	for i, c := range s.components {
		ids[i] = c.id()
	}
	var b strings.Builder
	b.WriteString("(" + strings.Join(ids, " ") + ")")
	b.WriteString(";created=" + strconv.FormatInt(now.Unix(), 10))
	if s.expires > 0 {
		b.WriteString(";expires=" + strconv.FormatInt(now.Add(s.expires).Unix(), 10))
	}
	b.WriteString(`;keyid="` + s.keyID + `"`)
	if s.includeAlg {
		b.WriteString(`;alg="` + s.alg + `"`)
	}
	if s.useNonce {
		b.WriteString(`;nonce="` + s.env.nonce(16) + `"`)
	}
	if s.tag != "" {
		b.WriteString(`;tag="` + s.tag + `"`)
	}
	return b.String()
}
