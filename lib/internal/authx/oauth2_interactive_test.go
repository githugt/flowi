// SPDX-License-Identifier: Apache-2.0

package authx

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
)

// idp is a fake identity provider for the grants that need a person.
type idp struct {
	srv *httptest.Server

	mu                sync.Mutex
	tokenReq          []url.Values
	pending           int // device polls answered "authorization_pending" before approving
	deny              bool
	challenge, method string
}

func newIDP(t *testing.T) *idp {
	t.Helper()
	p := &idp{}
	mux := http.NewServeMux()
	mux.HandleFunc("/authorize", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		p.mu.Lock()
		p.challenge, p.method = q.Get("code_challenge"), q.Get("code_challenge_method")
		p.mu.Unlock()
		target := q.Get("redirect_uri") + "?code=the-code&state=" + url.QueryEscape(q.Get("state"))
		if q.Get("deny") != "" {
			target = q.Get("redirect_uri") + "?error=access_denied&state=" + url.QueryEscape(q.Get("state"))
		}
		http.Redirect(w, r, target, http.StatusFound)
	})
	mux.HandleFunc("/device", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		_ = json.NewEncoder(w).Encode(map[string]any{"device_code": "dev-123", "user_code": "ABCD-EFGH", "verification_uri": p.srv.URL + "/verify",
			"expires_in": 600, "interval": 1})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		p.mu.Lock()
		p.tokenReq = append(p.tokenReq, r.PostForm)
		pending, deny := p.pending, p.deny
		if r.PostForm.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" && p.pending > 0 {
			p.pending--
		}
		p.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.PostForm.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" && deny:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"access_denied"}`))
		case r.PostForm.Get("grant_type") == "urn:ietf:params:oauth:grant-type:device_code" && pending > 0:
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
		default:
			_, _ = w.Write([]byte(`{"access_token":"tok-1","refresh_token":"ref-1","expires_in":3600}`))
		}
	})
	p.srv = httptest.NewServer(mux)
	t.Cleanup(p.srv.Close)
	return p
}

func (p *idp) tokenRequests() []url.Values {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]url.Values(nil), p.tokenReq...)
}

// browse plays the person: it opens the authorization URL the signer asks for.
func browse(t *testing.T) func(Prompt) {
	return func(pr Prompt) {
		if pr.Kind != PromptOpen {
			return
		}
		go func() {
			resp, err := http.Get(pr.URL) //nolint:noctx // a test stand-in for the user's browser
			if err != nil {
				t.Errorf("the browser could not reach %s: %v", pr.URL, err)
				return
			}
			_ = resp.Body.Close()
		}()
	}
}

func immediately(time.Duration) <-chan time.Time {
	c := make(chan time.Time, 1)
	c <- time.Time{}
	return c
}

func TestOAuth2_AuthorizationCode_should_send_the_person_to_the_provider_and_exchange_the_code_with_PKCE(t *testing.T) {
	// Arrange
	p := newIDP(t)
	a := auth("oauth2", "grant", "authorization_code", "authorization_url", p.srv.URL+"/authorize", "token_url", p.srv.URL+"/token",
		"client_id", "app", "scope", "read write")
	s, err := New(a, Env{Prompt: browse(t)})
	if err != nil {
		t.Fatal(err)
	}
	r := request("GET", "http://api/x")

	// Act
	applied, err := s.Apply(context.Background(), r)

	// Assert
	if err != nil || r.Header.Get("Authorization") != "Bearer tok-1" || len(applied) != 1 || !applied[0].Secret {
		t.Fatalf("applied %+v, header %q, err %v", applied, r.Header.Get("Authorization"), err)
	}
	req := p.tokenRequests()
	if len(req) != 1 || req[0].Get("grant_type") != "authorization_code" || req[0].Get("code") != "the-code" {
		t.Fatalf("token requests %v", req)
	}
	sum := sha256.Sum256([]byte(req[0].Get("code_verifier")))
	if want := base64.RawURLEncoding.EncodeToString(sum[:]); p.challenge != want || p.method != "S256" {
		t.Errorf("the challenge %q (%s) is not the S256 of the verifier sent to the token endpoint, want %q", p.challenge, p.method, want)
	}
	if !strings.HasPrefix(req[0].Get("redirect_uri"), "http://127.0.0.1:") {
		t.Errorf("the redirect must be on the loopback address: %q", req[0].Get("redirect_uri"))
	}
}

func TestOAuth2_AuthorizationCode_should_not_ask_the_person_again_while_the_token_lasts(t *testing.T) {
	// Arrange
	p := newIDP(t)
	var prompts int
	var mu sync.Mutex
	open := browse(t)
	env := Env{Tokens: NewTokenCache(), Prompt: func(pr Prompt) { mu.Lock(); prompts++; mu.Unlock(); open(pr) }}
	a := auth("oauth2", "grant", "authorization_code", "authorization_url", p.srv.URL+"/authorize", "token_url", p.srv.URL+"/token", "client_id", "app")
	s, _ := New(a, env)

	// Act
	_, first := s.Apply(context.Background(), request("GET", "http://api/x"))
	_, second := s.Apply(context.Background(), request("GET", "http://api/y"))

	// Assert
	mu.Lock()
	defer mu.Unlock()
	if first != nil || second != nil || prompts != 1 {
		t.Fatalf("errors %v %v, prompted %d times", first, second, prompts)
	}
}

func TestOAuth2_AuthorizationCode_should_report_a_person_who_says_no(t *testing.T) {
	// Arrange
	p := newIDP(t)
	a := auth("oauth2", "grant", "authorization_code", "authorization_url", p.srv.URL+"/authorize?deny=1", "token_url", p.srv.URL+"/token", "client_id", "app")
	s, _ := New(a, Env{Prompt: browse(t)})

	// Act
	_, err := s.Apply(context.Background(), request("GET", "http://api/x"))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("err %v", err)
	}
	if n := len(p.tokenRequests()); n != 0 {
		t.Errorf("no code was given, yet %d token requests were made", n)
	}
}

func TestOAuth2_AuthorizationCode_Given_nobody_comes_Then_gives_up_when_the_context_ends(t *testing.T) {
	// Arrange
	p := newIDP(t)
	a := auth("oauth2", "grant", "authorization_code", "authorization_url", p.srv.URL+"/authorize", "token_url", p.srv.URL+"/token", "client_id", "app")
	s, _ := New(a, Env{Prompt: func(Prompt) {}})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Act
	_, err := s.Apply(ctx, request("GET", "http://api/x"))

	// Assert
	if err == nil {
		t.Fatal("waiting for a person must end with the context")
	}
}

func TestOAuth2_AuthorizationCode_should_ignore_a_redirect_with_the_wrong_state(t *testing.T) {
	// Arrange
	p := newIDP(t)
	a := auth("oauth2", "grant", "authorization_code", "authorization_url", p.srv.URL+"/authorize", "token_url", p.srv.URL+"/token", "client_id", "app")
	env := Env{Prompt: func(pr Prompt) {
		u, _ := url.Parse(pr.URL)
		redirect := u.Query().Get("redirect_uri")
		go func() {
			resp, err := http.Get(redirect + "?code=evil&state=guessed") //nolint:noctx // a test stand-in for another program on this machine
			if err == nil {
				_ = resp.Body.Close()
			}
		}()
	}}
	s, _ := New(a, env)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Act
	_, err := s.Apply(ctx, request("GET", "http://api/x"))

	// Assert
	if err == nil || len(p.tokenRequests()) != 0 {
		t.Fatalf("a redirect that does not carry our state must not be exchanged (err %v, token requests %v)", err, p.tokenRequests())
	}
}

func TestOAuth2_DeviceCode_should_show_the_code_poll_until_approved_and_send_the_token(t *testing.T) {
	// Arrange
	p := newIDP(t)
	p.pending = 2
	var shown []Prompt
	env := Env{After: immediately, Prompt: func(pr Prompt) { shown = append(shown, pr) }}
	a := auth("oauth2", "grant", "device_code", "device_url", p.srv.URL+"/device", "token_url", p.srv.URL+"/token", "client_id", "tv", "scope", "read")
	s, err := New(a, env)
	if err != nil {
		t.Fatal(err)
	}
	r := request("GET", "http://api/x")

	// Act
	_, err = s.Apply(context.Background(), r)

	// Assert
	if err != nil || r.Header.Get("Authorization") != "Bearer tok-1" {
		t.Fatalf("header %q err %v", r.Header.Get("Authorization"), err)
	}
	if len(shown) != 1 || shown[0].Kind != PromptDevice || shown[0].Code != "ABCD-EFGH" || !strings.HasSuffix(shown[0].URL, "/verify") {
		t.Fatalf("the person was shown %+v", shown)
	}
	if n := len(p.tokenRequests()); n != 3 {
		t.Errorf("polled %d times, want 3 (two pending, then the token)", n)
	}
}

func TestOAuth2_DeviceCode_should_report_a_person_who_refuses(t *testing.T) {
	// Arrange
	p := newIDP(t)
	p.deny = true
	a := auth("oauth2", "grant", "device_code", "device_url", p.srv.URL+"/device", "token_url", p.srv.URL+"/token", "client_id", "tv")
	s, _ := New(a, Env{After: immediately, Prompt: func(Prompt) {}})

	// Act
	_, err := s.Apply(context.Background(), request("GET", "http://api/x"))

	// Assert
	if err == nil || !strings.Contains(err.Error(), "denied") {
		t.Fatalf("err %v", err)
	}
}

func TestOAuth2_should_say_what_an_interactive_grant_needs(t *testing.T) {
	cases := map[string]struct {
		a    model.Auth
		want string
	}{
		"authorization_url": {auth("oauth2", "grant", "authorization_code", "token_url", "http://t", "client_id", "c"), "authorization_url"},
		"device_url":        {auth("oauth2", "grant", "device_code", "token_url", "http://t", "client_id", "c"), "device_url"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := New(c.a, Env{})

			if err == nil || !strings.Contains(err.Error(), c.want) {
				t.Fatalf("err %v, want it to name %s", err, c.want)
			}
		})
	}
}
