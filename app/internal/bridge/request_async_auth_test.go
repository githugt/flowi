// SPDX-License-Identifier: AGPL-3.0-or-later

package bridge_test

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"

	"github.com/githugt/flowi/app/internal/bridge"
	"github.com/githugt/flowi/app/internal/vm/vmtest"
)

// openStream opens a message request of the protocol in a collection that has two named authentications: an API key
// (usable on a WebSocket) and a Digest login (not).
func openStream(t *testing.T, protocol string) (*bridge.RequestService, string, *published) {
	t.Helper()
	c := vmtest.SampleCollection()
	c.Auths = []model.NamedAuth{
		{Meta: model.Meta{ID: "A1", Name: "stream-key"}, Auth: model.Auth{Type: model.AuthAPIKey}},
		{Meta: model.Meta{ID: "A2", Name: "office"}, Auth: model.Auth{Type: model.AuthDigest}},
	}
	c.Requests = append(c.Requests, model.Request{Meta: model.Meta{ID: "R9", Name: "Stream"}, Path: "stream", File: "/c/requests/stream.req.yaml", Default: "1",
		Versions: []model.RequestVersion{{Label: "1", Async: &model.Async{Protocol: protocol, Server: "wss://h/x", Receive: &model.AsyncReceive{}}}}})
	b := &vmtest.Backend{Cols: []model.Collection{c}}
	pub := &published{}
	app := bridge.NewApp(b, bridge.NewInlineActor(), pub.emit)
	app.Start(t.Context())
	for _, n := range app.Workspace().Tree().Roots[0].Children[0].Children {
		if n.Label == "Stream" {
			if _, err := app.Requests().Open(n.ID); err != nil {
				t.Fatal(err)
			}
			return app.Requests(), n.ID, pub
		}
	}
	t.Fatal("no Stream request")
	return nil, "", nil
}

func ids(cs []bridge.Choice) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.ID)
	}
	return out
}

// Before: a WebSocket or SSE request had no Auth section: the editor showed Credentials only for protocols that define
// credential bindings, and these define none.
func TestRequestService_Given_a_ws_or_sse_request_When_opened_Then_it_takes_an_auth_block_with_the_schemes_a_handshake_can_use(t *testing.T) {
	for _, protocol := range []string{"ws", "sse"} {
		// Arrange
		reqs, node, pub := openStream(t, protocol)

		// Act
		reqs.SetAuthScheme(node, model.AuthAPIKey)
		got := pub.last["request"].(bridge.RequestView)

		// Assert
		if !got.Async.Auth || len(got.Async.Credentials) != 0 {
			t.Errorf("%s: auth %v credentials %v", protocol, got.Async.Auth, got.Async.Credentials)
		}
		want := []string{"scheme:apikey", "scheme:oauth2", "scheme:jwt", "scheme:mtls"}
		if !slices.Equal(ids(got.Auth.Schemes), want) {
			t.Errorf("%s: schemes %v, want %v", protocol, ids(got.Auth.Schemes), want)
		}
		if got.Auth.Kind != "scheme:apikey" {
			t.Errorf("%s: kind %q", protocol, got.Auth.Kind)
		}
	}
}

func TestRequestService_Given_a_ws_request_When_the_named_authentications_are_listed_Then_only_the_ones_it_can_use_are_offered(t *testing.T) {
	// Arrange
	reqs, node, pub := openStream(t, "ws")

	// Act
	reqs.SetAuthUse(node, "stream-key")
	got := pub.last["request"].(bridge.RequestView).Auth

	// Assert
	if got.Kind != "use" || got.UseMissing || !slices.Equal(ids(got.Named), []string{"stream-key"}) {
		t.Fatalf("auth %+v", got)
	}
}

func TestRequestService_Given_a_ws_request_uses_an_authentication_it_cannot_use_When_listed_Then_it_is_still_shown_and_not_called_missing(t *testing.T) {
	// Arrange
	reqs, node, pub := openStream(t, "ws")

	// Act
	reqs.SetAuthUse(node, "office")
	got := pub.last["request"].(bridge.RequestView).Auth

	// Assert
	if got.Use != "office" || got.UseMissing || !slices.Contains(ids(got.Named), "office") {
		t.Fatalf("auth %+v", got)
	}
}

func TestRequestService_Given_an_mqtt_request_When_opened_Then_it_keeps_its_credentials_section_and_every_scheme_is_listed(t *testing.T) {
	// Arrange / Act
	reqs, node, pub := openStream(t, "mqtt")
	reqs.SetAuthScheme(node, "")
	got := pub.last["request"].(bridge.RequestView)

	// Assert
	if got.Async.Auth || len(got.Async.Credentials) == 0 {
		t.Errorf("auth %v credentials %v", got.Async.Auth, got.Async.Credentials)
	}
	if len(got.Auth.Schemes) != len(model.AuthTypes()) {
		t.Errorf("schemes %v", ids(got.Auth.Schemes))
	}
}

// Before: moving a WebSocket request with an API key to MQTT left an auth block validation refuses.
func TestRequestService_Given_a_ws_request_with_auth_When_moved_to_mqtt_Then_the_auth_block_is_dropped(t *testing.T) {
	// Arrange
	reqs, node, pub := openStream(t, "ws")
	reqs.SetAuthScheme(node, model.AuthAPIKey)

	// Act
	reqs.SetProtocol(node, "mqtt")
	got := pub.last["request"].(bridge.RequestView)

	// Assert
	if got.Auth.Kind != "none" {
		t.Fatalf("auth %+v", got.Auth)
	}
}
