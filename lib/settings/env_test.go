// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
)

func TestEnvVarName(t *testing.T) {
	if got := EnvVarName("http.max_redirects"); got != "FLOWI_HTTP_MAX_REDIRECTS" {
		t.Fatalf("got %q", got)
	}
}

func TestFromEnviron(t *testing.T) {
	tests := []struct {
		name     string
		environ  []string
		wantKey  string
		wantVal  any
		wantCode diag.Code
	}{
		{name: "known_key", environ: []string{"FLOWI_TIMEOUT=5s"}, wantKey: "timeout", wantVal: 5 * time.Second},
		{name: "secret_ignored", environ: []string{"FLOWI_SECRET_STAGING_PASSWORD=x"}},
		{name: "reserved_ignored", environ: []string{"FLOWI_CONFIG_DIR=/x", "FLOWI_VAULT_KEY=k"}},
		{name: "non_flowi_ignored", environ: []string{"HOME=/h", "TIMEOUT=1"}},
		{name: "empty_value_is_unset", environ: []string{"FLOWI_TIMEOUT="}},
		{name: "typo_warns", environ: []string{"FLOWI_TIMOUT=5s"}, wantCode: diag.CodeSettingUnknown},
		{name: "bad_value", environ: []string{"FLOWI_HTTP_MAX_REDIRECTS=abc"}, wantCode: diag.CodeSettingInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lv, diags := FromEnviron(Schema(), tt.environ)

			if tt.wantCode != "" {
				if len(diags) != 1 || diags[0].Code != tt.wantCode {
					t.Fatalf("got %v", codes(diags))
				}
				return
			}
			if len(diags) != 0 {
				t.Fatalf("unexpected diagnostics %v", codes(diags))
			}
			if tt.wantKey == "" {
				if len(lv.Values) != 0 {
					t.Fatalf("unexpected values %v", lv.Values)
				}
				return
			}
			if lv.Values[tt.wantKey] != tt.wantVal || lv.Layer != LayerEnv {
				t.Fatalf("got %+v", lv)
			}
		})
	}
}
