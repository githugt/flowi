// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	// approvalWait is how long a person has to approve before the grant gives up.
	approvalWait = 5 * time.Minute
	// verifierLength is the PKCE code verifier's length (RFC 7636 allows 43 to 128).
	verifierLength = 64
	// defaultPollInterval is the device grant's polling interval when the provider names none.
	defaultPollInterval = 5 * time.Second
	deviceGrantType     = "urn:ietf:params:oauth:grant-type:device_code"
	callbackPath        = "/callback"
)

// callbackResult is what the provider sent back to the loopback address.
type callbackResult struct {
	code string
	err  error
}

// authorizationCode runs the authorization code grant (RFC 6749 §4.1) for a
// native client (RFC 8252): it listens on the loopback address, asks the
// person to open the provider's page, takes the code from the redirect and
// exchanges it, with PKCE (RFC 7636) unless the block turns it off.
//
// Goroutine ownership: the callback server runs in one goroutine started and
// stopped here; it is closed before this returns.
func (o *oauth2) authorizationCode(ctx context.Context) (token, error) {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:"+o.redirectPort)
	if err != nil {
		return token{}, fmt.Errorf("oauth2 auth: listening for the redirect: %w", err)
	}
	redirect := "http://" + listener.Addr().String() + callbackPath
	state, verifier := o.env.nonce(32), o.env.nonce(verifierLength)
	results := make(chan callbackResult, 1)
	srv := &http.Server{Handler: callbackHandler(state, results), ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(listener) }() // ends with Shutdown below; its error is the shutdown itself
	defer func() { _ = srv.Shutdown(context.WithoutCancel(ctx)) }()

	o.env.prompt(Prompt{Kind: PromptOpen, URL: o.authorizeURLFor(redirect, state, verifier), Expires: o.env.now().Add(approvalWait)})
	select {
	case res := <-results:
		if res.err != nil {
			return token{}, res.err
		}
		return o.fetch(ctx, o.codeForm(res.code, redirect, verifier))
	case <-ctx.Done():
		return token{}, fmt.Errorf("oauth2 auth: waiting for the person to approve: %w", context.Cause(ctx))
	case <-o.env.after(approvalWait):
		return token{}, errors.New("oauth2 auth: nobody approved within five minutes")
	}
}

// callbackHandler accepts the one redirect that carries our state. Anything
// else (another program probing the port, a forged redirect) is answered and
// ignored, so only the person's own approval ends the wait.
func callbackHandler(state string, results chan<- callbackResult) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc(callbackPath, func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "this is not the answer to the request Flowi made", http.StatusBadRequest)
			return
		}
		res := callbackResult{code: q.Get("code")}
		switch {
		case q.Get("error") != "":
			res.err = fmt.Errorf("oauth2 auth: the provider answered %s", strings.TrimSpace(q.Get("error")+" "+q.Get("error_description")))
		case res.code == "":
			res.err = errors.New("oauth2 auth: the provider's redirect had no code")
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if res.err != nil {
			fmt.Fprintf(w, "<p>Flowi could not sign in: %s</p>", html.EscapeString(res.err.Error()))
		} else {
			_, _ = io.WriteString(w, "<p>Signed in. You can close this window and go back to Flowi.</p>")
		}
		select {
		case results <- res:
		default: // a second answer to the same request: the first one stands
		}
	})
	return mux
}

func (o *oauth2) authorizeURLFor(redirect, state, verifier string) string {
	q := url.Values{"response_type": {"code"}, "client_id": {o.clientID}, "redirect_uri": {redirect}, "state": {state}}
	if o.scope != "" {
		q.Set("scope", o.scope)
	}
	if o.audience != "" {
		q.Set("audience", o.audience)
	}
	if o.pkce == "on" {
		sum := sha256.Sum256([]byte(verifier))
		q.Set("code_challenge", base64.RawURLEncoding.EncodeToString(sum[:]))
		q.Set("code_challenge_method", "S256")
	}
	sep := "?"
	if strings.Contains(o.authorizeURL, "?") {
		sep = "&"
	}
	return o.authorizeURL + sep + q.Encode()
}

func (o *oauth2) codeForm(code, redirect, verifier string) url.Values {
	f := url.Values{"grant_type": {"authorization_code"}, "code": {code}, "redirect_uri": {redirect}}
	if o.pkce == "on" {
		f.Set("code_verifier", verifier)
	}
	return f
}

// deviceAuthorization is the provider's answer to a device authorization request (RFC 8628 §3.2).
type deviceAuthorization struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURL         string `json:"verification_url"` // Google's spelling
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               any    `json:"expires_in"`
	Interval                any    `json:"interval"`
}

// deviceCode runs the device authorization grant (RFC 8628): it asks for a
// code, shows it to the person, and polls the token endpoint until they have
// approved on another device, refused, or the code has expired.
func (o *oauth2) deviceCode(ctx context.Context) (token, error) {
	da, err := o.requestDeviceCode(ctx)
	if err != nil {
		return token{}, err
	}
	link := cmpOrString(da.VerificationURIComplete, cmpOrString(da.VerificationURI, da.VerificationURL))
	validFor := lifetimeOr(da.ExpiresIn, approvalWait)
	o.env.prompt(Prompt{Kind: PromptDevice, URL: link, Code: da.UserCode, Expires: o.env.now().Add(validFor)})
	interval := lifetimeOr(da.Interval, defaultPollInterval)
	deadline := o.env.now().Add(validFor)
	for o.env.now().Before(deadline) {
		select {
		case <-ctx.Done():
			return token{}, fmt.Errorf("oauth2 auth: waiting for the person to approve: %w", context.Cause(ctx))
		case <-o.env.after(interval):
		}
		t, retry, err := o.pollDevice(ctx, da.DeviceCode)
		switch {
		case err != nil:
			return token{}, err
		case retry == pollSlowDown:
			interval += 5 * time.Second
		case retry == pollDone:
			return t, nil
		}
	}
	return token{}, errors.New("oauth2 auth: the code expired before anyone approved it")
}

func (o *oauth2) requestDeviceCode(ctx context.Context) (deviceAuthorization, error) {
	form := url.Values{"client_id": {o.clientID}}
	if o.scope != "" {
		form.Set("scope", o.scope)
	}
	if o.audience != "" {
		form.Set("audience", o.audience)
	}
	data, status, err := o.post(ctx, o.deviceURL, form)
	if err != nil {
		return deviceAuthorization{}, err
	}
	var da deviceAuthorization
	_ = json.Unmarshal(data, &da) // a body that is not JSON is reported below by its status or its missing code
	if status/100 != 2 {
		var tr tokenResponse
		_ = json.Unmarshal(data, &tr)
		return deviceAuthorization{}, o.tokenError(status, tr)
	}
	if da.DeviceCode == "" || da.UserCode == "" {
		return deviceAuthorization{}, errors.New("the device authorization endpoint answered without a device_code and a user_code")
	}
	return da, nil
}

// poll outcomes
const (
	pollDone     = iota // the person approved: the token is in hand
	pollWait            // not yet
	pollSlowDown        // not yet, and ask less often
)

func (o *oauth2) pollDevice(ctx context.Context, deviceCode string) (token, int, error) {
	form := url.Values{"grant_type": {deviceGrantType}, "device_code": {deviceCode}, "client_id": {o.clientID}}
	if o.clientSecret != "" {
		form.Set("client_secret", o.clientSecret)
	}
	data, status, err := o.post(ctx, o.tokenURL, form)
	if err != nil {
		return token{}, pollWait, err
	}
	var tr tokenResponse
	_ = json.Unmarshal(data, &tr)
	if status/100 == 2 && tr.AccessToken != "" {
		return token{access: tr.AccessToken, refresh: tr.RefreshToken, expires: o.env.now().Add(lifetime(tr.ExpiresIn))}, pollDone, nil
	}
	switch tr.Error {
	case "authorization_pending":
		return token{}, pollWait, nil
	case "slow_down":
		return token{}, pollSlowDown, nil
	case "access_denied":
		return token{}, pollWait, errors.New("oauth2 auth: the person denied the request")
	case "expired_token":
		return token{}, pollWait, errors.New("oauth2 auth: the code expired before anyone approved it")
	}
	return token{}, pollWait, o.tokenError(status, tr)
}

// post sends a form and returns the body and status, with the credentials
// kept out of any error.
func (o *oauth2) post(ctx context.Context, target string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, fmt.Errorf("oauth2 request: %w", o.redact(err))
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	resp, err := o.client().Do(req)
	if err != nil {
		var inner *url.Error
		if errors.As(err, &inner) {
			err = inner.Err
		}
		return nil, 0, fmt.Errorf("oauth2 request to %s: %w", target, o.redact(err))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponse))
	if err != nil {
		return nil, 0, fmt.Errorf("reading the response: %w", err)
	}
	return data, resp.StatusCode, nil
}

func cmpOrString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// lifetimeOr reads a number of seconds, or def when it is absent or not positive.
func lifetimeOr(v any, def time.Duration) time.Duration {
	if d, ok := seconds(v); ok {
		return d
	}
	return def
}
