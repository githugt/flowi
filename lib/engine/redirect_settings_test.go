// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"slices"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

func TestNewConfig_Given_user_and_request_redirect_settings_When_resolved_Then_request_overrides_user(t *testing.T) {
	user := settings.LayerValues{Layer: settings.LayerUser, Source: "user", Values: model.Settings{
		"http.redirect_credentials":  "keep",
		"http.redirect_keep_headers": []string{"X-Api-Key"},
	}}
	request := settings.LayerValues{Layer: settings.LayerRequest, Source: "request", Values: model.Settings{
		"http.redirect_credentials": "strip",
	}}

	forRequest := newConfig(settings.Resolve(settings.Schema(), user, request))
	elsewhere := newConfig(settings.Resolve(settings.Schema(), user))
	defaults := newConfig(settings.Resolve(settings.Schema()))

	if forRequest.redirectCreds != "strip" || !slices.Equal(forRequest.redirectKeep, []string{"X-Api-Key"}) {
		t.Errorf("request layer: %q %v", forRequest.redirectCreds, forRequest.redirectKeep)
	}
	if elsewhere.redirectCreds != "keep" {
		t.Errorf("user layer: %q", elsewhere.redirectCreds)
	}
	if defaults.redirectCreds != "strip" || len(defaults.redirectKeep) != 0 {
		t.Errorf("defaults: %q %v", defaults.redirectCreds, defaults.redirectKeep)
	}
}
