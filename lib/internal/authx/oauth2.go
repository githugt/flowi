// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/githugt/flowi-lib/internal/httpx"
)

const (
	tokenTimeout = 30 * time.Second
	// tokenSkew is how long before its expiry a token stops being used, so it
	// does not run out on the way to the server.
	tokenSkew = 30 * time.Second
	// maxTokenResponse bounds what a token endpoint may send.
	maxTokenResponse = 1 << 20
	defaultLifetime  = time.Hour
)

// TokenCache keeps OAuth 2.0 tokens between sends, so a collection run asks
// the token endpoint once, not for every request. It is safe for concurrent use.
type TokenCache struct {
	mu     sync.Mutex
	tokens map[string]token
}

// NewTokenCache returns an empty cache.
func NewTokenCache() *TokenCache { return &TokenCache{tokens: map[string]token{}} }

type token struct {
	access, refresh string
	expires         time.Time
}

func (c *TokenCache) get(key string) (token, bool) {
	if c == nil {
		return token{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t, ok := c.tokens[key]
	return t, ok
}

func (c *TokenCache) put(key string, t token) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tokens[key] = t
}

// oauth2 gets an access token from the token endpoint (client credentials,
// resource owner password, refresh token, or, with a person to approve it,
// authorization code with PKCE and device code) and sends it as
// "Authorization: Bearer …". The token is cached until it expires, and renewed
// with its refresh token when the endpoint gave one, so a person is asked
// once, not for every send.
type oauth2 struct {
	grant, tokenURL, clientID, clientSecret, scope, audience string
	user, password, refresh, clientAuth, prefix              string
	authorizeURL, deviceURL, redirectPort, pkce              string
	details                                                  string // authorization_details (RFC 9396), a JSON array
	env                                                      Env
}

func newOAuth2(p params, env Env) (Signer, error) {
	if err := p.only(); err != nil {
		return nil, err
	}
	o := &oauth2{
		grant: p.get("grant", "client_credentials"), tokenURL: p.values["token_url"], clientID: p.values["client_id"],
		clientSecret: p.values["client_secret"], scope: p.values["scope"], audience: p.values["audience"],
		user: p.values["username"], password: p.values["password"], refresh: p.values["refresh_token"],
		clientAuth: strings.ToLower(p.get("client_auth", "basic")), prefix: p.get("prefix", "Bearer"), env: env,
		authorizeURL: p.values["authorization_url"], deviceURL: p.values["device_url"], redirectPort: p.values["redirect_port"],
		pkce: strings.ToLower(p.get("pkce", "on")), details: strings.TrimSpace(p.values["authorization_details"]),
	}
	if err := o.check(p); err != nil {
		return nil, err
	}
	return o, nil
}

func (o *oauth2) check(p params) error {
	if err := p.need("token_url", "client_id"); err != nil {
		return err
	}
	if err := checkAuthorizationDetails(o.details); err != nil {
		return err
	}
	if o.clientAuth != "basic" && o.clientAuth != "body" {
		return fmt.Errorf("oauth2 auth: client_auth must be basic or body, got %q", o.clientAuth)
	}
	switch o.grant {
	case "client_credentials":
		return p.need("client_secret")
	case "password":
		return p.need("username", "password")
	case "refresh_token":
		return p.need("refresh_token")
	case "authorization_code":
		return o.checkAuthorizationCode(p)
	case "device_code":
		return p.need("device_url")
	case "implicit":
		return errors.New("oauth2 auth: the implicit grant is not supported (it is deprecated); use authorization_code, which has PKCE")
	}
	return fmt.Errorf("oauth2 auth: grant must be client_credentials, password, refresh_token, authorization_code or device_code, got %q", o.grant)
}

// checkAuthorizationDetails refuses authorization_details (RFC 9396) that is not a JSON array of objects that each have a
// "type": a server would answer with an error that does not say what was wrong with the value.
func checkAuthorizationDetails(details string) error {
	if details == "" {
		return nil
	}
	var list []map[string]any
	if err := json.Unmarshal([]byte(details), &list); err != nil {
		return fmt.Errorf("oauth2 auth: authorization_details must be a JSON array of objects: %w", err)
	}
	for i, item := range list {
		if t, _ := item["type"].(string); t == "" {
			return fmt.Errorf("oauth2 auth: authorization_details item %d has no \"type\"", i+1)
		}
	}
	return nil
}

// key identifies the token this block asks for: everything that changes
// what the endpoint answers, but not the secrets themselves.
func (o *oauth2) key() string {
	sum := sha256.Sum256([]byte(strings.Join([]string{o.grant, o.tokenURL, o.clientID, o.clientSecret, o.scope, o.audience, o.user, o.password, o.details}, "\x00")))
	return hex.EncodeToString(sum[:])
}

func (o *oauth2) Apply(ctx context.Context, r *httpx.Request) ([]Applied, error) {
	t, err := o.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	value := strings.TrimSpace(o.prefix + " " + t)
	r.Header.Set("Authorization", value)
	return []Applied{{Where: "header", Name: "Authorization", Value: value, Secret: true}}, nil
}

func (o *oauth2) checkAuthorizationCode(p params) error {
	if err := p.need("authorization_url"); err != nil {
		return err
	}
	if o.pkce != "on" && o.pkce != "off" {
		return fmt.Errorf("oauth2 auth: pkce must be on or off, got %q", o.pkce)
	}
	if port := o.redirectPort; port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return fmt.Errorf("oauth2 auth: redirect_port must be a port number, got %q", port)
		}
	}
	return nil
}

func (o *oauth2) accessToken(ctx context.Context) (string, error) {
	key := o.key()
	cached, ok := o.env.Tokens.get(key)
	if ok && o.env.now().Before(cached.expires.Add(-tokenSkew)) {
		return cached.access, nil
	}
	if ok && cached.refresh != "" {
		if t, err := o.fetch(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {cached.refresh}}); err == nil {
			o.env.Tokens.put(key, t)
			return t.access, nil
		}
		// a refresh token that no longer works falls back to asking afresh
	}
	t, err := o.newToken(ctx)
	if err != nil {
		return "", err
	}
	o.env.Tokens.put(key, t)
	return t.access, nil
}

// newToken asks for a token afresh: the grants that need a person go through
// them, the others post their form.
func (o *oauth2) newToken(ctx context.Context) (token, error) {
	switch o.grant {
	case "authorization_code":
		return o.authorizationCode(ctx)
	case "device_code":
		return o.deviceCode(ctx)
	}
	return o.fetch(ctx, o.form())
}

func (o *oauth2) form() url.Values {
	f := url.Values{"grant_type": {o.grant}}
	switch o.grant {
	case "password":
		f.Set("username", o.user)
		f.Set("password", o.password)
	case "refresh_token":
		f.Set("refresh_token", o.refresh)
	}
	if o.scope != "" {
		f.Set("scope", o.scope)
	}
	if o.audience != "" {
		f.Set("audience", o.audience)
	}
	if o.details != "" {
		f.Set("authorization_details", o.details)
	}
	return f
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    any    `json:"expires_in"`
	Error        string `json:"error"`
	Description  string `json:"error_description"`
}

func (o *oauth2) fetch(ctx context.Context, form url.Values) (token, error) {
	if o.clientAuth == "body" || o.clientSecret == "" { // a public client has no secret to send as Basic: it names itself in the form
		form.Set("client_id", o.clientID)
		if o.clientSecret != "" {
			form.Set("client_secret", o.clientSecret)
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return token{}, fmt.Errorf("oauth2 token request: %w", o.redact(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	if o.clientAuth == "basic" && o.clientSecret != "" {
		req.SetBasicAuth(url.QueryEscape(o.clientID), url.QueryEscape(o.clientSecret))
	}
	resp, err := o.client().Do(req)
	if err != nil {
		var inner *url.Error
		if errors.As(err, &inner) {
			err = inner.Err // the URL error would quote the endpoint's URL again
		}
		return token{}, fmt.Errorf("oauth2 token request to %s: %w", o.tokenURL, o.redact(err))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponse))
	if err != nil {
		return token{}, fmt.Errorf("reading the token response: %w", err)
	}
	var tr tokenResponse
	_ = json.Unmarshal(data, &tr) // a response that is not JSON is reported below by its status or its missing token
	if resp.StatusCode/100 != 2 {
		return token{}, o.tokenError(resp.StatusCode, tr)
	}
	if tr.AccessToken == "" {
		return token{}, errors.New("the token endpoint answered without an access_token")
	}
	return token{access: tr.AccessToken, refresh: tr.RefreshToken, expires: o.env.now().Add(lifetime(tr.ExpiresIn))}, nil
}

func (o *oauth2) tokenError(status int, tr tokenResponse) error {
	detail := strings.TrimSpace(tr.Error + " " + tr.Description)
	if detail == "" {
		detail = http.StatusText(status)
	}
	return fmt.Errorf("the token endpoint answered %d: %s", status, o.redactText(detail))
}

func (o *oauth2) client() *http.Client {
	if o.env.HTTP != nil {
		return o.env.HTTP
	}
	return &http.Client{Timeout: tokenTimeout}
}

// redact hides the credentials in an error that may quote them.
func (o *oauth2) redact(err error) error {
	if err == nil {
		return errors.New("failed")
	}
	return errors.New(o.redactText(err.Error()))
}

func (o *oauth2) redactText(s string) string {
	for _, secret := range []string{o.clientSecret, o.password, o.refresh} {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	return s
}

// lifetime reads expires_in, a number of seconds that some servers send as a
// string.
func lifetime(v any) time.Duration {
	if d, ok := seconds(v); ok {
		return d
	}
	return defaultLifetime
}

// seconds reads a positive number of seconds that a server sent as a number or a string.
func seconds(v any) (time.Duration, bool) {
	switch n := v.(type) {
	case float64:
		if n > 0 {
			return time.Duration(n * float64(time.Second)), true
		}
	case string:
		var secs float64
		if _, err := fmt.Sscanf(n, "%f", &secs); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second)), true
		}
	}
	return 0, false
}
