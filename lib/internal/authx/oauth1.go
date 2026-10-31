// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // OAuth 1.0a's HMAC-SHA1 signature method
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/githugt/flowi/lib/internal/httpx"
)

// oauth1 signs a request with OAuth 1.0a (RFC 5849) and sends the result in
// the Authorization header: HMAC-SHA1, HMAC-SHA256 or PLAINTEXT over the
// method, URL, query and form-body parameters and the oauth_* parameters.
type oauth1 struct {
	consumerKey, consumerSecret, token, tokenSecret, method, realm, verifier, callback string
	env                                                                                Env
}

func newOAuth1(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	if err := p.need("consumer_key", "consumer_secret"); err != nil {
		return nil, err
	}
	o := &oauth1{consumerKey: p.values["consumer_key"], consumerSecret: p.values["consumer_secret"], token: p.values["token"],
		tokenSecret: p.values["token_secret"], method: strings.ToUpper(p.get("signature_method", "HMAC-SHA1")),
		realm: p.values["realm"], verifier: p.values["verifier"], callback: p.values["callback"], env: env}
	switch o.method {
	case "HMAC-SHA1", "HMAC-SHA256", "PLAINTEXT":
	default:
		return nil, fmt.Errorf("oauth1 auth: signature_method must be HMAC-SHA1, HMAC-SHA256 or PLAINTEXT, got %q", o.method)
	}
	return o, nil
}

func (o *oauth1) Apply(_ context.Context, r *httpx.Request) ([]Applied, error) {
	u, err := url.Parse(r.URL)
	if err != nil {
		return nil, fmt.Errorf("oauth1 auth: %w", err)
	}
	form, err := formParams(r)
	if err != nil {
		return nil, err
	}
	oauth := o.oauthParams()
	sig := o.signature(r.Method, u, append(append(queryParams(u), form...), oauth...))
	oauth = append(oauth, pair{"oauth_signature", sig})
	var parts []string
	if o.realm != "" {
		parts = append(parts, `realm="`+o.realm+`"`)
	}
	for _, p := range oauth {
		parts = append(parts, percent(p.name)+`="`+percent(p.value)+`"`)
	}
	value := "OAuth " + strings.Join(parts, ", ")
	r.Header.Set("Authorization", value)
	return []Applied{{Where: "header", Name: "Authorization", Value: value, Secret: true}}, nil
}

type pair struct{ name, value string }

func (o *oauth1) oauthParams() []pair {
	ps := []pair{
		{"oauth_consumer_key", o.consumerKey},
		{"oauth_nonce", o.env.nonce(32)},
		{"oauth_signature_method", o.method},
		{"oauth_timestamp", strconv.FormatInt(o.env.now().Unix(), 10)},
		{"oauth_version", "1.0"},
	}
	for _, opt := range []pair{{"oauth_token", o.token}, {"oauth_verifier", o.verifier}, {"oauth_callback", o.callback}} {
		if opt.value != "" {
			ps = append(ps, opt)
		}
	}
	return ps
}

// signature is the oauth_signature of a request with the given parameters
// (every oauth_* one except the signature itself).
func (o *oauth1) signature(method string, u *url.URL, params []pair) string {
	key := percent(o.consumerSecret) + "&" + percent(o.tokenSecret)
	if o.method == "PLAINTEXT" {
		return key
	}
	newHash := sha1.New
	if o.method == "HMAC-SHA256" {
		newHash = sha256.New
	}
	m := hmac.New(newHash, []byte(key))
	m.Write([]byte(baseString(method, u, params)))
	return base64.StdEncoding.EncodeToString(m.Sum(nil))
}

// baseString is the signature base string of RFC 5849 section 3.4.1.
func baseString(method string, u *url.URL, params []pair) string {
	enc := make([]pair, len(params))
	for i, p := range params {
		enc[i] = pair{percent(p.name), percent(p.value)}
	}
	sort.Slice(enc, func(i, j int) bool {
		if enc[i].name != enc[j].name {
			return enc[i].name < enc[j].name
		}
		return enc[i].value < enc[j].value
	})
	parts := make([]string, len(enc))
	for i, p := range enc {
		parts[i] = p.name + "=" + p.value
	}
	return strings.ToUpper(method) + "&" + percent(baseURL(u)) + "&" + percent(strings.Join(parts, "&"))
}

// baseURL is the scheme, host and path in lower case, without a default port
// or the query.
func baseURL(u *url.URL) string {
	scheme, host := strings.ToLower(u.Scheme), strings.ToLower(u.Hostname())
	port := u.Port()
	if port != "" && (scheme != "http" || port != "80") && (scheme != "https" || port != "443") {
		host += ":" + port
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	return scheme + "://" + host + path
}

func queryParams(u *url.URL) []pair {
	var out []pair
	for name, values := range u.Query() {
		for _, v := range values {
			out = append(out, pair{name, v})
		}
	}
	return out
}

// formParams are the parameters of an x-www-form-urlencoded body, which the
// signature covers; any other body is not part of it.
func formParams(r *httpx.Request) ([]pair, error) {
	media, _, _ := strings.Cut(r.Header.Get("Content-Type"), ";")
	if !strings.EqualFold(strings.TrimSpace(media), "application/x-www-form-urlencoded") {
		return nil, nil
	}
	body, err := bodyBytes(r)
	if err != nil {
		return nil, err
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, fmt.Errorf("oauth1 auth: the form body does not parse: %w", err)
	}
	var out []pair
	for name, vs := range values {
		for _, v := range vs {
			out = append(out, pair{name, v})
		}
	}
	return out, nil
}

// percent encodes as RFC 3986 asks of OAuth: everything but letters, digits,
// "-", ".", "_" and "~".
func percent(s string) string {
	const hexDigits = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9', c == '-', c == '.', c == '_', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexDigits[c>>4])
			b.WriteByte(hexDigits[c&15])
		}
	}
	return b.String()
}
