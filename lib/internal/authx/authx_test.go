// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/githugt/flowi-lib/internal/httpx"
	"github.com/githugt/flowi-lib/model"
)

func auth(typ string, kv ...string) model.Auth {
	a := model.Auth{Type: typ}
	for i := 0; i+1 < len(kv); i += 2 {
		a.Params = append(a.Params, model.KeyValue{Key: kv[i], Value: kv[i+1]})
	}
	return a
}

func request(method, rawURL string) *httpx.Request {
	return &httpx.Request{Method: method, URL: rawURL, Header: http.Header{}}
}

func fixedEnv(at time.Time) Env {
	return Env{Now: func() time.Time { return at }, Nonce: func(n int) string { return strings.Repeat("n", n) }}
}

func mustApply(t *testing.T, a model.Auth, env Env, r *httpx.Request) []Applied {
	t.Helper()
	s, err := New(a, env)
	if err != nil {
		t.Fatal(err)
	}
	applied, err := s.Apply(context.Background(), r)
	if err != nil {
		t.Fatal(err)
	}
	return applied
}

func TestNew_should_refuse_unknown_types_unknown_params_and_missing_ones(t *testing.T) {
	cases := map[string]model.Auth{
		"no type":       auth(""),
		"unknown type":  auth("kerberos"),
		"unknown param": auth("apikey", "name", "k", "value", "v", "colour", "red"),
		"missing":       auth("apikey", "name", "k"),
		"bad in":        auth("apikey", "name", "k", "value", "v", "in", "body"),
		"bad alg":       auth("jwt", "algorithm", "none", "secret", "s"),
		"code grant":    auth("oauth2", "grant", "authorization_code", "token_url", "http://t", "client_id", "c"),
	}
	for name, a := range cases {
		t.Run(name, func(t *testing.T) {
			// Act
			_, err := New(a, Env{})

			// Assert
			if err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAPIKey_should_go_in_the_header_the_query_or_a_cookie(t *testing.T) {
	// Arrange
	header, query, cookie := request("GET", "http://h/p?a=1"), request("GET", "http://h/p?a=1"), request("GET", "http://h/p")

	// Act
	mustApply(t, auth("apikey", "name", "X-Key", "value", "s3"), Env{}, header)
	applied := mustApply(t, auth("apikey", "name", "key", "value", "s 3", "in", "query"), Env{}, query)
	mustApply(t, auth("apikey", "name", "sid", "value", "abc", "in", "cookie"), Env{}, cookie)

	// Assert
	if header.Header.Get("X-Key") != "s3" || cookie.Header.Get("Cookie") != "sid=abc" {
		t.Errorf("header %v cookie %v", header.Header, cookie.Header)
	}
	if query.URL != "http://h/p?a=1&key=s+3" {
		t.Errorf("url %s", query.URL)
	}
	if len(applied) != 1 || !applied[0].Secret || applied[0].Where != "query" {
		t.Errorf("applied %+v", applied)
	}
}

// RFC 2617 section 3.5's example.
func TestDigest_should_answer_the_challenge_as_the_RFC_does(t *testing.T) {
	// Arrange
	env := fixedEnv(time.Unix(0, 0))
	env.Nonce = func(int) string { return "0a4f113b" }
	s, err := New(auth("digest", "username", "Mufasa", "password", "Circle Of Life"), env)
	if err != nil {
		t.Fatal(err)
	}
	r := request("GET", "http://www.nowhere.org/dir/index.html")
	resp := &httpx.Response{Status: 401, Header: http.Header{"Www-Authenticate": {`Digest realm="testrealm@host.com", qop="auth,auth-int", nonce="dcd98b7102dd2f0e8b11d0f600bfb0c093", opaque="5ccc069c403ebaf9f0171e9517f40e41"`}}}

	// Act
	first, _ := s.Apply(context.Background(), r)
	challenged := s.(Challenger).Challenge(resp)
	applied, err := s.Apply(context.Background(), r)

	// Assert
	if len(first) != 0 || !challenged || err != nil {
		t.Fatalf("first %v challenged %v err %v", first, challenged, err)
	}
	h := r.Header.Get("Authorization")
	if !strings.Contains(h, `response="6629fae49393a05397450978507c4ef1"`) || !strings.Contains(h, "qop=auth") || !strings.Contains(h, `opaque="5ccc069c403ebaf9f0171e9517f40e41"`) {
		t.Errorf("authorization %s", h)
	}
	if len(applied) != 1 || !applied[0].Secret {
		t.Errorf("applied %+v", applied)
	}
}

func TestDigest_should_not_answer_a_second_401_or_a_different_scheme(t *testing.T) {
	// Arrange
	s, _ := New(auth("digest", "username", "u", "password", "p"), Env{})
	basic := &httpx.Response{Status: 401, Header: http.Header{"Www-Authenticate": {`Basic realm="x"`}}}
	digestResp := &httpx.Response{Status: 401, Header: http.Header{"Www-Authenticate": {`Digest realm="x", nonce="n"`}}}

	// Act and Assert
	c := s.(Challenger)
	if c.Challenge(basic) {
		t.Error("a Basic challenge is not a Digest one")
	}
	if !c.Challenge(digestResp) || c.Challenge(digestResp) {
		t.Error("the first Digest challenge is answered, a second one means the credentials were wrong")
	}
}

func TestOAuth2_should_get_a_token_once_cache_it_and_send_it_as_a_bearer(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		user, pass, _ := r.BasicAuth()
		if err := r.ParseForm(); err != nil || r.Form.Get("grant_type") != "client_credentials" || user != "id" || pass != "sec" || r.Form.Get("scope") != "read" {
			http.Error(w, `{"error":"invalid_client"}`, http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "tok-" + string(rune('0'+calls.Load())), "expires_in": 3600})
	}))
	defer srv.Close()
	now := time.Unix(1000, 0)
	env := Env{Now: func() time.Time { return now }, Tokens: NewTokenCache()}
	a := auth("oauth2", "token_url", srv.URL, "client_id", "id", "client_secret", "sec", "scope", "read")

	// Act
	r1, r2 := request("GET", "http://api/x"), request("GET", "http://api/y")
	mustApply(t, a, env, r1)
	mustApply(t, a, env, r2)
	now = now.Add(2 * time.Hour)
	r3 := request("GET", "http://api/z")
	mustApply(t, a, env, r3)

	// Assert
	if r1.Header.Get("Authorization") != "Bearer tok-1" || r2.Header.Get("Authorization") != "Bearer tok-1" {
		t.Errorf("first two: %q %q", r1.Header.Get("Authorization"), r2.Header.Get("Authorization"))
	}
	if r3.Header.Get("Authorization") != "Bearer tok-2" || calls.Load() != 2 {
		t.Errorf("after expiry: %q after %d calls (one to get a token, one more when it ran out)", r3.Header.Get("Authorization"), calls.Load())
	}
}

func TestOAuth2_should_renew_with_the_refresh_token_when_the_endpoint_gave_one(t *testing.T) {
	// Arrange
	var grants []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		grants = append(grants, r.Form.Get("grant_type"))
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "a" + string(rune('0'+len(grants))), "refresh_token": "r", "expires_in": 60})
	}))
	defer srv.Close()
	now := time.Unix(0, 0)
	env := Env{Now: func() time.Time { return now }, Tokens: NewTokenCache()}
	a := auth("oauth2", "grant", "password", "token_url", srv.URL, "client_id", "c", "client_secret", "s", "username", "u", "password", "p", "client_auth", "body")

	// Act
	mustApply(t, a, env, request("GET", "http://api/"))
	now = now.Add(2 * time.Minute)
	r := request("GET", "http://api/")
	mustApply(t, a, env, r)

	// Assert
	if len(grants) != 2 || grants[0] != "password" || grants[1] != "refresh_token" || r.Header.Get("Authorization") != "Bearer a2" {
		t.Errorf("grants %v authorization %q", grants, r.Header.Get("Authorization"))
	}
}

func TestOAuth2_should_report_the_endpoint_error_without_the_secret(t *testing.T) {
	// Arrange
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"invalid_client","error_description":"bad secret hunter2"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()
	s, _ := New(auth("oauth2", "token_url", srv.URL, "client_id", "id", "client_secret", "hunter2"), Env{})

	// Act
	_, err := s.Apply(context.Background(), request("GET", "http://api/"))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "401") || !strings.Contains(err.Error(), "invalid_client") || strings.Contains(err.Error(), "hunter2") {
		t.Errorf("err %v", err)
	}
}

func TestJWT_should_sign_with_hmac_and_add_issued_and_expiry_times(t *testing.T) {
	// Arrange
	now := time.Unix(1_700_000_000, 0)
	r := request("GET", "http://api/")

	// Act
	mustApply(t, auth("jwt", "secret", "k", "claims", `{"sub":"ada"}`, "expires_in", "10m"), fixedEnv(now), r)

	// Assert
	raw := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	claims := jwt.MapClaims{}
	if _, err := jwt.ParseWithClaims(raw, claims, func(*jwt.Token) (any, error) { return []byte("k"), nil }, jwt.WithTimeFunc(func() time.Time { return now }), jwt.WithValidMethods([]string{"HS256"})); err != nil {
		t.Fatalf("the token does not verify: %v", err)
	}
	if claims["sub"] != "ada" || claims["iat"] != float64(now.Unix()) || claims["exp"] != float64(now.Add(10*time.Minute).Unix()) {
		t.Errorf("claims %v", claims)
	}
}

func TestJWT_should_sign_with_an_rsa_private_key(t *testing.T) {
	// Arrange
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemKey := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}))
	r := request("GET", "http://api/")

	// Act
	mustApply(t, auth("jwt", "algorithm", "RS256", "private_key", pemKey, "claims", `{"iss":"me"}`, "header", "X-Token", "prefix", ""), Env{}, r)

	// Assert
	raw := r.Header.Get("X-Token")
	if _, err := jwt.Parse(raw, func(*jwt.Token) (any, error) { return &key.PublicKey, nil }, jwt.WithValidMethods([]string{"RS256"})); err != nil {
		t.Errorf("the token does not verify: %v (%s)", err, raw)
	}
}

// The example of the Hawk documentation (sha256, no payload).
func TestHawk_should_compute_the_documented_mac(t *testing.T) {
	// Arrange
	h := &hawk{id: "dh37fgj492je", key: "werxhqb98rpaxn39848xrunpaw3489ruxnpa98w4rxn", algorithm: "sha256", ext: "some-app-ext-data", newHash: sha256.New}
	u, _ := url.Parse("http://example.com:8000/resource/1?b=1&a=2")

	// Act
	mac := h.mac("GET", u, "1353832234", "j4h3g2", "")

	// Assert
	if mac != "6R4rV5iE+NPoym+WwjeHzjAGXUtLNIxmo1vpMofpLAE=" {
		t.Errorf("mac %s", mac)
	}
}

func TestHawk_should_send_a_header_with_the_id_time_and_mac(t *testing.T) {
	// Arrange
	r := request("POST", "http://example.com/x")
	r.Header.Set("Content-Type", "text/plain; charset=utf-8")
	r.Body = httpx.BytesBody([]byte("Thank you for flying Hawk"), "text/plain")

	// Act
	applied := mustApply(t, auth("hawk", "id", "abc", "key", "k"), fixedEnv(time.Unix(1353832234, 0)), r)

	// Assert
	h := r.Header.Get("Authorization")
	for _, want := range []string{`Hawk id="abc"`, `ts="1353832234"`, `nonce="nnnnnn"`, `hash="`, `mac="`} {
		if !strings.Contains(h, want) {
			t.Errorf("authorization lacks %s: %s", want, h)
		}
	}
	if len(applied) != 1 || !applied[0].Secret {
		t.Errorf("applied %+v", applied)
	}
}

// Twitter's documented signing example for OAuth 1.0a.
func TestOAuth1_should_compute_the_documented_signature(t *testing.T) {
	// Arrange
	o := &oauth1{consumerSecret: "kAcSOqF21Fu85e7zjz7ZN2U4ZRhfV3WpwPAoE3Z7kBw", tokenSecret: "LswwdoUaIvS8ltyTt5jkRh4J50vUPVVHtR2YPi5kE", method: "HMAC-SHA1"}
	u, _ := url.Parse("https://api.twitter.com/1.1/statuses/update.json?include_entities=true")
	params := append(queryParams(u),
		pair{"status", "Hello Ladies + Gentlemen, a signed OAuth request!"},
		pair{"oauth_consumer_key", "xvz1evFS4wEEPTGEFPHBog"},
		pair{"oauth_nonce", "kYjzVBB8Y0ZFabxSWbWovY3uYSQ2pTgmZeNu2VS4cg"},
		pair{"oauth_signature_method", "HMAC-SHA1"},
		pair{"oauth_timestamp", "1318622958"},
		pair{"oauth_token", "370773112-GmHxMAgYyLbNEtIKZeRNFsMKPR9EyMZeS9weJAEb"},
		pair{"oauth_version", "1.0"},
	)

	// Act
	sig := o.signature("POST", u, params)

	// Assert
	if sig != "hCtSmYh+iHYCEqBWrE7C7hYmtUk=" {
		t.Errorf("signature %s", sig)
	}
}

func TestOAuth1_should_send_the_oauth_parameters_in_the_header(t *testing.T) {
	// Arrange
	r := request("GET", "http://api/x?a=1")

	// Act
	mustApply(t, auth("oauth1", "consumer_key", "ck", "consumer_secret", "cs", "token", "t", "token_secret", "ts", "realm", "r"), fixedEnv(time.Unix(5, 0)), r)

	// Assert
	h := r.Header.Get("Authorization")
	for _, want := range []string{`OAuth realm="r"`, `oauth_consumer_key="ck"`, `oauth_token="t"`, `oauth_timestamp="5"`, `oauth_version="1.0"`, `oauth_signature="`} {
		if !strings.Contains(h, want) {
			t.Errorf("authorization lacks %s: %s", want, h)
		}
	}
}

func TestSigV4_should_add_an_authorization_with_the_credential_scope(t *testing.T) {
	// Arrange
	r := request("GET", "https://example.execute-api.eu-west-1.amazonaws.com/prod/items?x=1")
	r.Header.Set("Host", "example.execute-api.eu-west-1.amazonaws.com")

	// Act
	applied := mustApply(t, auth("aws_sigv4", "access_key_id", "AKIDEXAMPLE", "secret_access_key", "wJalrXUtnFEMI", "region", "eu-west-1", "service", "execute-api"), fixedEnv(time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC)), r)

	// Assert
	h := r.Header.Get("Authorization")
	if !strings.HasPrefix(h, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20261025/eu-west-1/execute-api/aws4_request") || r.Header.Get("X-Amz-Date") != "20261025T120000Z" {
		t.Errorf("authorization %s, date %s", h, r.Header.Get("X-Amz-Date"))
	}
	secret := 0
	for _, a := range applied {
		if a.Secret {
			secret++
		}
	}
	if len(applied) < 2 || secret != 1 {
		t.Errorf("applied %+v: the Authorization is a secret, the date is not", applied)
	}
}
