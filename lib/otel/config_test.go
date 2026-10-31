// SPDX-License-Identifier: Apache-2.0

package otel

import (
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/settings"
)

func resolved(values model.Settings) settings.Resolved {
	return settings.Resolve(settings.Schema(), settings.LayerValues{Layer: settings.LayerUser, Source: "settings.yaml", Values: values})
}

func TestConfigFrom_Given_no_endpoint_Then_off(t *testing.T) {
	_, on, err := ConfigFrom(resolved(model.Settings{}), "1.0")

	if on || err != nil {
		t.Fatalf("on=%v err=%v", on, err)
	}
}

func TestConfigFrom_should_read_endpoint_headers_and_propagation(t *testing.T) {
	r := resolved(model.Settings{
		"otel.endpoint": "http://collector:4318", "otel.propagate": true,
		"otel.headers": []string{"Authorization=Bearer abc=", "X-Team = ops"},
	})

	cfg, on, err := ConfigFrom(r, "1.2.3")

	if !on || err != nil {
		t.Fatalf("on=%v err=%v", on, err)
	}
	if cfg.Endpoint != "http://collector:4318" || !cfg.Propagate || cfg.Version != "1.2.3" ||
		cfg.Headers["Authorization"] != "Bearer abc=" || cfg.Headers["X-Team"] != "ops" {
		t.Fatalf("cfg %+v", cfg)
	}
}

func TestConfigFrom_Given_a_bad_header_or_endpoint_Then_error(t *testing.T) {
	tests := map[string]model.Settings{
		"header without =":  {"otel.endpoint": "http://c:4318", "otel.headers": []string{"oops"}},
		"header with CRLF":  {"otel.endpoint": "http://c:4318", "otel.headers": []string{"X=a\r\nInjected: 1"}},
		"endpoint not http": {"otel.endpoint": "ftp://c"},
	}
	for name, values := range tests {
		t.Run(name, func(t *testing.T) {
			_, on, err := ConfigFrom(resolved(values), "1.0")

			if err == nil || on {
				t.Fatalf("accepted: on=%v err=%v", on, err)
			}
		})
	}
}

func TestConfigFrom_should_never_repeat_a_header_entry_in_its_error(t *testing.T) {
	for _, entry := range []string{"Authorization: Bearer SECRETTOKEN123", "X=a\r\nInjected: SECRETTOKEN123", "SECRETTOKEN123"} {
		_, _, err := ConfigFrom(resolved(model.Settings{"otel.endpoint": "http://c:4318", "otel.headers": []string{entry}}), "1.0")

		if err == nil || strings.Contains(err.Error(), "SECRETTOKEN123") {
			t.Errorf("entry %q: error %v", entry, err)
		}
	}
}
