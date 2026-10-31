// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func withAuth(v model.RequestVersion, typ string, kv ...string) model.RequestVersion {
	a := &model.Auth{Type: typ}
	for i := 0; i+1 < len(kv); i += 2 {
		a.Params = append(a.Params, model.KeyValue{Key: kv[i], Value: kv[i+1]})
	}
	v.Auth = a
	return v
}

func TestSend_Auth_apikey_should_send_the_key_and_keep_the_secret_out_of_the_result(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", withAuth(get("/echo/auth"), "apikey", "name", "X-Key", "value", "{{secret.password}}"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed || res.Record == nil {
		t.Fatalf("result %+v diags %v", res.Status, res.Diags)
	}
	if f.lastEcho()["X-Key"] != secretV {
		t.Errorf("the server saw %q", f.lastEcho()["X-Key"])
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Errorf("the key is in the result: %s", data)
	}
	shown := ""
	for _, h := range res.Record.Request.Header {
		if h.Name == "X-Key" {
			shown = h.Value
		}
	}
	if shown == "" || shown == secretV {
		t.Errorf("the recorded request should list the header, masked: %q", shown)
	}
}

func TestSend_Auth_apikey_in_the_query_should_mask_the_recorded_url(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", withAuth(get("/echo/auth"), "apikey", "name", "key", "value", "{{secret.password}}", "in", "query"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
	if !strings.Contains(echoed(t, res.Record)["query"].(string), "key=") {
		t.Errorf("the server saw no key: %v", echoed(t, res.Record))
	}
	if strings.Contains(res.Record.Request.URL, secretV) {
		t.Errorf("the recorded URL shows the key: %s", res.Record.Request.URL)
	}
}

func TestSend_Auth_should_fail_the_send_for_an_auth_block_that_cannot_work(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", withAuth(get("/echo/auth"), "oauth2", "token_url", "http://x"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAuthInvalid) || f.hits.Load() != 0 {
		t.Fatalf("status %v diags %v hits %d (nothing may be sent)", res.Status, res.Diags, f.hits.Load())
	}
}

func TestSend_Auth_digest_should_answer_the_401_and_record_both_attempts(t *testing.T) {
	// Arrange
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Digest ") {
			w.Header().Set("WWW-Authenticate", `Digest realm="r", nonce="abc", qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("hello"))
	}))
	defer srv.Close()
	f := newFixture(t)
	v := model.RequestVersion{Method: "GET", URL: srv.URL + "/secret"}
	r := f.addRequest("digest", withAuth(v, "digest", "username", "u", "password", "{{secret.password}}"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed || res.Record == nil || res.Record.Status != 200 {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
	if calls.Load() != 2 || len(res.Record.Attempts) != 2 {
		t.Errorf("calls %d attempts %d: one without credentials, one with", calls.Load(), len(res.Record.Attempts))
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Errorf("the password is in the result: %s", data)
	}
}

func TestSend_Auth_mtls_should_fail_clearly_when_the_certificate_is_missing(t *testing.T) {
	// Arrange
	f := newFixture(t)
	r := f.addRequest("echo", withAuth(get("/echo/auth"), "mtls", "cert", "certs/client.pem", "key", "certs/client.key"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAuthInvalid) {
		t.Fatalf("status %v diags %v", res.Status, res.Diags)
	}
}

func TestSend_Auth_device_code_should_announce_the_code_and_send_the_token_once_approved(t *testing.T) {
	// Arrange
	var polls atomic.Int32
	idp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/device":
			_, _ = w.Write([]byte(`{"device_code":"d","user_code":"WXYZ-1234","verification_uri":"https://idp.example/verify","expires_in":600,"interval":1}`))
		default:
			if polls.Add(1) < 2 {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
				return
			}
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
		}
	}))
	defer idp.Close()
	f := newFixture(t)
	r := f.addRequest("echo", withAuth(get("/echo/auth"), "oauth2", "grant", "device_code", "device_url", idp.URL+"/device", "token_url", idp.URL+"/token", "client_id", "tv"))

	// Act
	res, events := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if got := f.lastEcho()["Authorization"]; got != "Bearer tok" {
		t.Errorf("the server saw %q", got)
	}
	var prompts []AuthPrompted
	for _, ev := range events {
		if p, ok := ev.(AuthPrompted); ok {
			prompts = append(prompts, p)
		}
	}
	if len(prompts) != 1 || prompts[0].Kind != "device" || prompts[0].Code != "WXYZ-1234" || prompts[0].URL != "https://idp.example/verify" {
		t.Fatalf("prompts %+v", prompts)
	}
}

// ntlmServer answers an NTLM handshake: it challenges the negotiate message and
// accepts an authenticate message that names the user and arrives on the same
// connection as the negotiate message.
func ntlmServer(t *testing.T, sameConn *atomic.Bool) *httptest.Server {
	t.Helper()
	var negotiatedOn atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		raw, _ := base64.StdEncoding.DecodeString(strings.TrimPrefix(h, "NTLM "))
		switch {
		case len(raw) > 12 && raw[8] == 1:
			negotiatedOn.Store(r.RemoteAddr)
			challenge := make([]byte, 48)
			copy(challenge, "NTLMSSP\x00")
			challenge[8] = 2
			challenge[20] = 0x01 // unicode
			copy(challenge[24:], "01234567")
			w.Header().Set("WWW-Authenticate", "NTLM "+base64.StdEncoding.EncodeToString(challenge))
			w.WriteHeader(http.StatusUnauthorized)
		case len(raw) > 12 && raw[8] == 3:
			sameConn.Store(negotiatedOn.Load() == r.RemoteAddr)
			_, _ = w.Write([]byte("welcome"))
		default:
			w.Header().Set("WWW-Authenticate", "NTLM")
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestSend_Auth_ntlm_should_negotiate_then_answer_on_the_same_connection(t *testing.T) {
	// Arrange
	var sameConn atomic.Bool
	srv := ntlmServer(t, &sameConn)
	f := newFixture(t)
	v := model.RequestVersion{Method: "GET", URL: srv.URL + "/secure"}
	r := f.addRequest("intranet", withAuth(v, "ntlm", "username", `CORP\ada`, "password", "{{secret.password}}"))

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed || res.Record == nil || res.Record.Status != 200 {
		t.Fatalf("status %s diags %v record %+v", res.Status, res.Diags, res.Record)
	}
	if !sameConn.Load() {
		t.Error("the answer must arrive on the connection the negotiate message used")
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Errorf("the password is in the result: %s", data)
	}
}
