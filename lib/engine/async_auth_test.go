// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/transport"
)

// sendConnected sends a WebSocket or SSE version through a scripted transport and returns what the transport was
// given. tweak changes the collection before the send (named authentications, auth.default).
func sendConnected(t *testing.T, v model.RequestVersion, tweak func(*model.Collection)) (SendResult, *scripted) {
	t.Helper()
	tr := &scripted{}
	f := newFixture(t)
	f.transports = transport.Registry{"ws": tr, "sse": tr}
	if tweak != nil {
		f.update(tweak)
	}
	r := f.addRequest("feed/live", v)
	res, _ := f.send(f.input(r))
	return res, tr
}

func wsVersion(server, channel string) model.RequestVersion {
	return asyncVersion(model.Async{Protocol: "ws", Server: server, Channel: channel, Receive: &model.AsyncReceive{}})
}

func sseVersion(server string) model.RequestVersion {
	return asyncVersion(model.Async{Protocol: "sse", Server: server, Receive: &model.AsyncReceive{}})
}

func connectionHeader(op transport.Operation, name string) string {
	for _, h := range op.Headers {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

func recordedHeader(rec *Record, name string) string {
	for _, h := range rec.Request.Header {
		if strings.EqualFold(h.Name, name) {
			return h.Value
		}
	}
	return ""
}

// Before: a WebSocket or SSE request ignored its auth block (validation refused it), so the only way to log in was a
// header typed by hand.
func TestSend_Async_Given_a_ws_request_has_an_apikey_header_When_sent_Then_the_handshake_carries_it_masked_in_the_record(t *testing.T) {
	// Arrange
	v := withAuth(wsVersion("ws://localhost:9000", "feed"), "apikey", "name", "X-Key", "value", "{{secret.password}}")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusPassed || res.Record == nil {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if got := connectionHeader(tr.got, "X-Key"); got != secretV {
		t.Errorf("the transport got %q, want the real key", got)
	}
	if shown := recordedHeader(res.Record, "X-Key"); shown == "" || shown == secretV {
		t.Errorf("the record should list the header masked, got %q", shown)
	}
	data, _ := json.Marshal(res)
	if strings.Contains(string(data), secretV) {
		t.Errorf("the key is in the result: %s", data)
	}
}

func TestSend_Async_Given_an_sse_request_has_an_apikey_in_the_query_When_sent_Then_the_url_carries_it_and_the_record_hides_it(t *testing.T) {
	// Arrange
	v := withAuth(sseVersion("http://localhost:9000/events"), "apikey", "name", "token", "value", "{{secret.password}}", "in", "query")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusPassed || res.Record == nil {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if want := "http://localhost:9000/events?token=" + secretV; tr.got.Server != want {
		t.Errorf("server %q, want %q", tr.got.Server, want)
	}
	if strings.Contains(res.Record.Request.URL, secretV) || !strings.Contains(res.Record.Request.URL, "token=") {
		t.Errorf("the recorded URL %q should show the parameter without its value", res.Record.Request.URL)
	}
}

func TestSend_Async_Given_a_ws_request_with_a_channel_and_a_query_key_When_sent_Then_the_query_follows_the_channel_path(t *testing.T) {
	// Arrange
	v := withAuth(wsVersion("ws://localhost:9000", "feed"), "apikey", "name", "token", "value", "{{secret.password}}", "in", "query")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	// Before folding the channel into the address, the transport would have joined it after the query:
	// ws://localhost:9000?token=hunter2/feed.
	if got := transport.JoinURL(tr.got.Server, tr.got.Channel); got != "ws://localhost:9000/feed?token="+secretV {
		t.Errorf("the transport dials %q", got)
	}
}

func TestSend_Async_Given_a_ws_request_has_a_jwt_auth_When_sent_Then_a_bearer_token_is_signed_into_the_headers(t *testing.T) {
	// Arrange
	v := withAuth(wsVersion("ws://localhost:9000", ""), "jwt", "algorithm", "HS256", "secret", "{{secret.password}}", "prefix", "Bearer")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	token := strings.TrimPrefix(connectionHeader(tr.got, "Authorization"), "Bearer ")
	if strings.Count(token, ".") != 2 {
		t.Fatalf("Authorization %q is not a signed JWT", connectionHeader(tr.got, "Authorization"))
	}
	if shown := recordedHeader(res.Record, "Authorization"); strings.Contains(shown, token) {
		t.Errorf("the record shows the token: %q", shown)
	}
}

func TestSend_Async_Given_the_auth_block_and_a_typed_header_name_the_same_header_When_sent_Then_the_auth_block_wins(t *testing.T) {
	// Arrange
	v := withAuth(sseVersion("http://localhost:9000/events"), "apikey", "name", "X-Key", "value", "{{secret.password}}")
	v.Headers = []model.KeyValue{{Key: "x-key", Value: "stale"}, {Key: "Accept-Language", Value: "en"}}

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusPassed {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	count := 0
	for _, h := range tr.got.Headers {
		if strings.EqualFold(h.Name, "X-Key") {
			count++
		}
	}
	if count != 1 || connectionHeader(tr.got, "X-Key") != secretV || connectionHeader(tr.got, "Accept-Language") != "en" {
		t.Errorf("headers %+v", tr.got.Headers)
	}
}

func TestSend_Async_Given_an_sse_request_uses_a_named_authentication_When_sent_Then_the_named_scheme_is_applied(t *testing.T) {
	// Arrange
	v := sseVersion("http://localhost:9000/events")
	v.Auth = &model.Auth{Use: "partner-key"}
	named := func(c *model.Collection) {
		c.Auths = []model.NamedAuth{{Meta: model.Meta{Name: "partner-key"}, Auth: model.Auth{Type: model.AuthAPIKey,
			Params: []model.KeyValue{{Key: "name", Value: "X-Key"}, {Key: "value", Value: "{{secret.password}}"}}}}}
	}

	// Act
	res, tr := sendConnected(t, v, named)

	// Assert
	if res.Status != StatusPassed || connectionHeader(tr.got, "X-Key") != secretV {
		t.Fatalf("status %s diags %v headers %+v", res.Status, res.Diags, tr.got.Headers)
	}
}

func TestSend_Async_Given_a_request_names_an_authentication_the_collection_lacks_When_sent_Then_nothing_connects(t *testing.T) {
	// Arrange
	v := wsVersion("ws://localhost:9000", "")
	v.Auth = &model.Auth{Use: "gone"}

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAuthUnknown) || tr.got.Server != "" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

func TestSend_Async_Given_a_scheme_that_signs_the_request_When_sent_Then_it_fails_with_FLW_AUTH_001_and_nothing_connects(t *testing.T) {
	// Arrange
	v := withAuth(wsVersion("ws://localhost:9000", ""), "digest", "username", "u", "password", "{{secret.password}}")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAuthInvalid) || tr.got.Server != "" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

func TestSend_Async_Given_mtls_names_a_missing_certificate_When_sent_Then_it_fails_with_FLW_AUTH_001(t *testing.T) {
	// Arrange
	v := withAuth(sseVersion("https://localhost:9000/events"), "mtls", "cert", "certs/client.pem", "key", "certs/client.key")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeAuthInvalid) || tr.got.Server != "" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

func TestSend_Async_Given_the_auth_secret_would_go_to_an_untrusted_host_When_sent_Then_FLW_SEC_005_refuses_it(t *testing.T) {
	// Arrange
	// Before: the key reached the host as a header the host check never saw, because it was typed in no field.
	v := withAuth(wsVersion("wss://stream.evil.example", ""), "apikey", "name", "X-Key", "value", "{{secret.password}}")

	// Act
	res, tr := sendConnected(t, v, nil)

	// Assert
	if res.Status != StatusError || !hasCode(res.Diags, diag.CodeSecretHost) || tr.got.Server != "" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

func TestSend_Async_Given_auth_default_is_set_When_a_ws_request_has_none_Then_it_inherits_a_scheme_it_can_use(t *testing.T) {
	// Arrange
	byName := func(auth model.Auth) func(*model.Collection) {
		return func(c *model.Collection) {
			c.Auths = []model.NamedAuth{{Meta: model.Meta{Name: "house"}, Auth: auth}}
			c.Manifest.Settings = model.Settings{"auth.default": "house"}
		}
	}
	key := model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Key"}, {Key: "value", Value: "{{secret.password}}"}}}
	digest := model.Auth{Type: model.AuthDigest, Params: []model.KeyValue{{Key: "username", Value: "u"}, {Key: "password", Value: "{{secret.password}}"}}}

	// Act
	usable, usableTr := sendConnected(t, wsVersion("ws://localhost:9000", ""), byName(key))
	// Before: a default written for HTTP (digest) would now fail every WebSocket request of the collection.
	unusable, unusableTr := sendConnected(t, wsVersion("ws://localhost:9000", ""), byName(digest))

	// Assert
	if usable.Status != StatusPassed || connectionHeader(usableTr.got, "X-Key") != secretV {
		t.Errorf("usable default: status %s diags %v headers %+v", usable.Status, usable.Diags, usableTr.got.Headers)
	}
	if unusable.Status != StatusPassed || len(unusableTr.got.Headers) != 0 {
		t.Errorf("unusable default: status %s diags %v headers %+v", unusable.Status, unusable.Diags, unusableTr.got.Headers)
	}
}

func TestSend_Async_Given_a_protocol_that_takes_credentials_in_its_bindings_When_it_has_an_auth_block_Then_the_block_is_not_applied(t *testing.T) {
	// Arrange
	tr := &scripted{}
	v := withAuth(asyncVersion(model.Async{Server: "mqtt://localhost:1883", Receive: &model.AsyncReceive{}}), "apikey", "name", "X-Key", "value", "{{secret.password}}")

	// Act
	res, _ := sendAsync(t, tr, v, nil)

	// Assert
	if res.Status != StatusPassed || len(tr.got.Headers) != 0 || tr.got.Server != "mqtt://localhost:1883" {
		t.Fatalf("status %s diags %v got %+v", res.Status, res.Diags, tr.got)
	}
}

func TestSend_Async_Given_a_real_sse_server_When_the_request_has_an_apikey_Then_the_stream_opens_with_the_header(t *testing.T) {
	// Arrange
	var gotHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("X-Key")
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: hello\n\n")
	}))
	t.Cleanup(srv.Close)
	f := newFixture(t)
	v := asyncVersion(model.Async{Protocol: "sse", Server: srv.URL + "/events", Receive: &model.AsyncReceive{Count: 1}})
	v.Auth = &model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "X-Key"}, {Key: "value", Value: "{{secret.password}}"}}}
	r := f.addRequest("feed/live", v)

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed || res.Record == nil || len(res.Record.Messages) != 1 {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if gotHeader != secretV {
		t.Errorf("the server saw X-Key %q", gotHeader)
	}
}

func TestSend_Async_Given_a_real_websocket_server_When_the_request_has_an_apikey_in_the_query_Then_the_handshake_carries_it_on_the_channel_path(t *testing.T) {
	// Arrange
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotKey = r.URL.Path, r.URL.Query().Get("token")
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_ = c.Write(r.Context(), websocket.MessageText, []byte("hello"))
		_ = c.Close(websocket.StatusNormalClosure, "")
	}))
	t.Cleanup(srv.Close)
	f := newFixture(t)
	v := asyncVersion(model.Async{Protocol: "ws", Server: "ws" + strings.TrimPrefix(srv.URL, "http"), Channel: "feed", Receive: &model.AsyncReceive{Count: 1}})
	v.Auth = &model.Auth{Type: model.AuthAPIKey, Params: []model.KeyValue{{Key: "name", Value: "token"}, {Key: "value", Value: "{{secret.password}}"}, {Key: "in", Value: "query"}}}
	r := f.addRequest("feed/live", v)

	// Act
	res, _ := f.send(f.input(r))

	// Assert
	if res.Status != StatusPassed || res.Record == nil || len(res.Record.Messages) != 1 {
		t.Fatalf("status %s diags %v", res.Status, res.Diags)
	}
	if gotPath != "/feed" || gotKey != secretV {
		t.Errorf("the server saw path %q and token %q", gotPath, gotKey)
	}
}
