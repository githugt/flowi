// SPDX-License-Identifier: Apache-2.0

package settings

import (
	"testing"
	"time"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func codes(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		out = append(out, d.Code)
	}
	return out
}

func TestFromBlock_Given_personal_key_at_collection_layer_Then_FLW_SET_003(t *testing.T) {
	_, diags := FromBlock(Schema(), LayerCollection, "flowi.yaml", map[string]any{"appearance.theme": "dark"})

	if len(diags) != 1 || diags[0].Code != diag.CodeSettingWrongLayer {
		t.Fatalf("got %v", codes(diags))
	}
}

func TestFromBlock_Given_unknown_key_Then_FLW_SET_001_warning_and_key_dropped(t *testing.T) {
	lv, diags := FromBlock(Schema(), LayerUser, "settings.yaml", map[string]any{"timout": "5s"})

	if len(diags) != 1 || diags[0].Code != diag.CodeSettingUnknown || diags[0].Severity != diag.SeverityWarning {
		t.Fatalf("got %v", codes(diags))
	}
	if len(lv.Values) != 0 {
		t.Fatalf("unknown key kept: %v", lv.Values)
	}
}

func TestFromBlock_Given_invalid_value_Then_FLW_SET_002_with_dotted_path(t *testing.T) {
	_, diags := FromBlock(Schema(), LayerCollection, "flowi.yaml", map[string]any{"http.max_redirects": int64(500)})

	if len(diags) != 1 || diags[0].Code != diag.CodeSettingInvalid {
		t.Fatalf("got %v", codes(diags))
	}
	if diags[0].Path != "$.settings.http.max_redirects" {
		t.Fatalf("path %q", diags[0].Path)
	}
}

func TestResolve_should_apply_precedence(t *testing.T) {
	user := LayerValues{Layer: LayerUser, Source: "settings.yaml", Values: model.Settings{"timeout": 20 * time.Second}}
	coll := LayerValues{Layer: LayerCollection, Source: "flowi.yaml", Values: model.Settings{"timeout": 5 * time.Second}}
	env := LayerValues{Layer: LayerEnv, Source: "FLOWI_TIMEOUT", Values: model.Settings{"timeout": 7 * time.Second}}
	tests := []struct {
		name      string
		layers    []LayerValues
		want      time.Duration
		wantLayer Layer
	}{
		{name: "env_wins", layers: []LayerValues{env, user, coll}, want: 7 * time.Second, wantLayer: LayerEnv},
		{name: "collection_beats_user", layers: []LayerValues{user, coll}, want: 5 * time.Second, wantLayer: LayerCollection},
		{name: "default_when_unset", layers: nil, want: 30 * time.Second, wantLayer: LayerDefault},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := Resolve(Schema(), tt.layers...)

			v, ok := r.Get("timeout")

			if !ok || v.Value != tt.want || v.Layer != tt.wantLayer {
				t.Fatalf("got %+v, %v", v, ok)
			}
		})
	}
}

func TestExplain_should_list_layers_highest_first_with_default_last(t *testing.T) {
	user := LayerValues{Layer: LayerUser, Source: "u", Values: model.Settings{"timeout": 20 * time.Second}}
	env := LayerValues{Layer: LayerEnv, Source: "e", Values: model.Settings{"timeout": 7 * time.Second}}

	got := Resolve(Schema(), user, env).Explain("timeout")

	want := []Layer{LayerEnv, LayerUser, LayerDefault}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for i, v := range got {
		if v.Layer != want[i] {
			t.Fatalf("position %d: got %v, want %v", i, v.Layer, want[i])
		}
	}
}

func TestLayer_String(t *testing.T) {
	if LayerFolder.String() != "folder" || LayerFlag.String() != "flag" {
		t.Fatalf("got %q, %q", LayerFolder, LayerFlag)
	}
}
