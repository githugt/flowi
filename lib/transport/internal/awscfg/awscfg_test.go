// SPDX-License-Identifier: Apache-2.0

package awscfg_test

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/transport"
	"github.com/githugt/flowi-lib/transport/internal/awscfg"
)

func TestLoad_should_refuse_an_unknown_auth_mode(t *testing.T) {
	op := transport.Operation{Binding: map[string]string{"auth": "maybe"}}

	_, err := awscfg.Load(t.Context(), op)

	if err == nil || !strings.Contains(err.Error(), "auth") {
		t.Fatalf("err %v, want one naming auth", err)
	}
}

func TestLoad_should_name_each_missing_explicit_credential(t *testing.T) {
	tests := []struct{ missing, want string }{
		{"access_key_id", "access_key_id"},
		{"secret_access_key", "secret_access_key"},
	}
	for _, tt := range tests {
		binding := map[string]string{"auth": "explicit", "access_key_id": "AKID", "secret_access_key": "secret"}
		binding[tt.missing] = ""

		_, err := awscfg.Load(t.Context(), transport.Operation{Binding: binding})

		if err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("without %s: err %v, want one naming it", tt.missing, err)
		}
	}
}

func TestLoad_should_default_the_region_when_a_custom_endpoint_is_used(t *testing.T) {
	t.Setenv("AWS_REGION", "")
	t.Setenv("AWS_DEFAULT_REGION", "")
	t.Setenv("AWS_CONFIG_FILE", t.TempDir()+"/none")
	op := transport.Operation{Server: "http://localhost:4566",
		Binding: map[string]string{"auth": "explicit", "access_key_id": "AKID", "secret_access_key": "secret"}}

	cfg, err := awscfg.Load(t.Context(), op)

	if err != nil || cfg.Region == "" {
		t.Fatalf("err %v, region %q, want a region for the emulator", err, cfg.Region)
	}
}
