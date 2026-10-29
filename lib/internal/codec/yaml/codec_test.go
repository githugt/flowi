// SPDX-License-Identifier: Apache-2.0

package yamlcodec

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
)

func newCodec() *Codec { return New(settings.Schema()) }

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func requireNoDiagnostics(t *testing.T, l diag.List) {
	t.Helper()
	if len(l) > 0 {
		for _, d := range l {
			msg, _ := diag.English().Message(d)
			t.Errorf("%s %d:%d %s %s", d.Code, d.Pos.Line, d.Pos.Column, d.Path, msg)
		}
		t.FailNow()
	}
}

func TestDecodeManifest_should_round_trip_canonical_fixture(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/flowi.yaml")

	m, diags := c.DecodeManifest("flowi.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeManifest(m)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n--- got\n%s\n--- want\n%s", out, data)
	}
	if len(m.Variables) != 2 || m.Variables[0].Key != "region" || m.Variables[1].Value != "https://api.example.com" {
		t.Fatalf("variables %+v", m.Variables)
	}
	if m.Format != 1 || len(m.DependsOn) != 2 || m.Settings["http.max_redirects"] != int64(5) {
		t.Fatalf("unexpected manifest %+v", m)
	}
	if len(m.Hooks.Pre) != 2 || m.Hooks.Pre[0].Flow == nil || len(m.Hooks.Pre[1].Inline) != 1 {
		t.Fatalf("unexpected hooks %+v", m.Hooks)
	}
}

func TestDecodeEnvironment_should_round_trip_and_mark_secret(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/staging.yaml")

	env, diags := c.DecodeEnvironment("staging.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeEnvironment(env)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n%s", out)
	}
	if len(env.Vars) != 3 || !env.Vars[2].Secret || env.Vars[2].Value != "" {
		t.Fatalf("unexpected vars %+v", env.Vars)
	}
}

func TestDecodeEnvironment_should_reject_inline_secret_value(t *testing.T) {
	data := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nvars:\n  password:\n    secret: true\n    value: hunter2\n")

	env, diags := newCodec().DecodeEnvironment("e.yaml", data)

	if len(diags) != 1 || diags[0].Code != diag.CodeYAMLType || diags[0].Pos.Line != 5 {
		t.Fatalf("got %+v", diags)
	}
	if env.Vars[0].Value != "" || !env.Vars[0].Secret {
		t.Fatalf("secret value kept: %+v", env.Vars[0])
	}
}

func TestDecodeEnvironment_should_default_name_to_nothing_when_absent(t *testing.T) {
	data := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nvars:\n  a: \"1\"\n")

	env, diags := newCodec().DecodeEnvironment("local.yaml", data)

	requireNoDiagnostics(t, diags)
	if env.Name != "" {
		t.Fatalf("codec must not invent names; got %q", env.Name)
	}
}

func TestDecodeWorkspace_should_round_trip(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/workspace.yaml")

	ws, diags := c.DecodeWorkspace("workspace.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeWorkspace(ws)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n%s", out)
	}
	if len(ws.Collections) != 2 || ws.Settings["defaults.environment"] != "staging" {
		t.Fatalf("unexpected workspace %+v", ws)
	}
}

func TestDecodeFolder_should_round_trip(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/_folder.yaml")

	f, diags := c.DecodeFolder("_folder.yaml", "requests/auth", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFolder(f)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) || f.Dir != "requests/auth" || f.Settings["timeout"] != 10*time.Second {
		t.Fatalf("got %+v\n%s", f, out)
	}
}

func TestDecodeSettingsFile_should_flatten_nested_keys(t *testing.T) {
	values, diags := newCodec().DecodeSettingsFile("settings.yaml", readFixture(t, "valid/settings.yaml"))

	requireNoDiagnostics(t, diags)
	if values["http.max_redirects"] != int64(3) || values["appearance.theme"] != "dark" || values["timeout"] != "20s" {
		t.Fatalf("got %v", values)
	}
}

func TestEncodeSettingsFile_should_write_schema_order(t *testing.T) {
	c := newCodec()
	values, _ := c.DecodeSettingsFile("settings.yaml", readFixture(t, "valid/settings.yaml"))
	lv, diags := settings.FromBlock(settings.Schema(), settings.LayerUser, "settings.yaml", values)
	requireNoDiagnostics(t, diags)

	out, err := c.EncodeSettingsFile(lv.Values)

	if err != nil {
		t.Fatal(err)
	}
	want := "timeout: 20s\nhttp:\n  max_redirects: 3\nappearance:\n  theme: dark\nlogging:\n  level: debug\n"
	if string(out) != want {
		t.Fatalf("got:\n%s", out)
	}
}

func TestDecode_invalid(t *testing.T) {
	tests := []struct {
		file       string
		wantCode   diag.Code
		wantLine   int
		wantColumn int
	}{
		{"unknown-key.yaml", diag.CodeYAMLUnknownKey, 3, 1},
		{"duplicate-key.yaml", diag.CodeYAMLDuplicateKey, 4, 1},
		{"bad-type.yaml", diag.CodeYAMLType, 1, 9},
		{"syntax.yaml", diag.CodeYAMLSyntax, 2, 5},
		{"empty.yaml", diag.CodeYAMLEmpty, 0, 0},
		{"comments-only.yaml", diag.CodeYAMLEmpty, 0, 0},
		{"missing-id.yaml", diag.CodeIDMissing, 1, 1},
		{"bad-id.yaml", diag.CodeIDInvalid, 2, 5},
		{"empty-id.yaml", diag.CodeIDMissing, 2, 5},
		{"personal-setting.yaml", diag.CodeSettingWrongLayer, 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			_, diags := newCodec().DecodeManifest(tt.file, readFixture(t, "invalid/"+tt.file))

			if len(diags) != 1 {
				t.Fatalf("got %d diagnostics: %+v", len(diags), diags)
			}
			d := diags[0]
			if d.Code != tt.wantCode || d.Pos.Line != tt.wantLine || d.Pos.Column != tt.wantColumn || d.File != tt.file {
				t.Fatalf("got %s at %d:%d in %s", d.Code, d.Pos.Line, d.Pos.Column, d.File)
			}
		})
	}
}

func TestDecode_should_accept_CRLF_and_warn_on_BOM(t *testing.T) {
	lf := readFixture(t, "valid/flowi.yaml")
	crlf := append([]byte{0xEF, 0xBB, 0xBF}, bytes.ReplaceAll(lf, []byte("\n"), []byte("\r\n"))...)
	c := newCodec()

	want, _ := c.DecodeManifest("flowi.yaml", lf)
	got, diags := c.DecodeManifest("flowi.yaml", crlf)

	if len(diags) != 1 || diags[0].Code != diag.CodeLintInvisible || diags[0].Pos.Line != 1 || diags[0].Args["char"] != "U+FEFF" {
		t.Fatalf("got %+v", diags)
	}
	wantOut, _ := c.EncodeManifest(want)
	gotOut, _ := c.EncodeManifest(got)
	if !bytes.Equal(wantOut, gotOut) {
		t.Fatalf("CRLF decoded differently:\n%s", gotOut)
	}
}

func TestLocate_should_return_position_of_path(t *testing.T) {
	data := readFixture(t, "valid/flowi.yaml")

	pos, ok := newCodec().Locate(data, "$.depends_on[1].id")

	if !ok || pos.Line != 9 || pos.Column != 9 {
		t.Fatalf("got %+v %v", pos, ok)
	}
}

func TestLocate_should_return_false_for_missing_path(t *testing.T) {
	_, ok := newCodec().Locate(readFixture(t, "valid/flowi.yaml"), "$.nodes[3]")

	if ok {
		t.Fatal("ok = true for a path that does not exist")
	}
}

func TestDecode_should_reject_multiple_documents(t *testing.T) {
	data := []byte("format: 1\nid: 01J9Z8K3M4N5P6Q7R8S9T0VWXY\n---\nname: second\n")

	_, diags := newCodec().DecodeManifest("flowi.yaml", data)

	if len(diags) != 1 || diags[0].Code != diag.CodeYAMLSyntax || diags[0].Pos.Line != 3 {
		t.Fatalf("got %+v", diags)
	}
}

func TestHasComments(t *testing.T) {
	tests := []struct {
		src  string
		want bool
	}{
		{"a: 1 # trailing\n", true},
		{"# head\na: 1\n", true},
		{"a: \"# not a comment\"\nb: x#y\n", false},
		{"a: |\n  # inside a literal block\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			if got := HasComments([]byte(tt.src)); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDecodeWorkspace_should_read_and_round_trip_schedules(t *testing.T) {
	c := newCodec()
	data := readFixture(t, "valid/workspace-schedules.yaml")

	ws, diags := c.DecodeWorkspace("workspace.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeWorkspace(ws)

	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n%s", out)
	}
	if len(ws.Schedules) != 2 {
		t.Fatalf("schedules %+v", ws.Schedules)
	}
	nightly := ws.Schedules[0]
	if nightly.Name != "nightly-smoke" || nightly.Flow != "user-creation" || nightly.Collection != "onboarding-api" ||
		nightly.Cron != "0 3 * * *" || nightly.Env != "staging" || nightly.TZ != "Europe/Berlin" || nightly.Disabled ||
		len(nightly.Inputs) != 1 || nightly.Inputs[0].Key != "email" {
		t.Fatalf("nightly %+v", nightly)
	}
	if hourly := ws.Schedules[1]; hourly.Cron != "@hourly" || !hourly.Disabled {
		t.Fatalf("hourly %+v", hourly)
	}
}

func TestDecodeWorkspace_Given_a_schedule_with_an_unknown_key_Then_reports_it(t *testing.T) {
	data := []byte("id: 01JA003M4N5P6Q7R8S9T0VWXYZ\nschedules:\n  - name: a\n    flow: f\n    cron: '@daily'\n    whne: now\n")

	_, diags := newCodec().DecodeWorkspace("workspace.yaml", data)

	if len(diags) != 1 || diags[0].Code != diag.CodeYAMLUnknownKey {
		t.Fatalf("diags %+v", diags)
	}
}

func TestDecodeFolder_should_read_and_write_the_order_of_its_entries(t *testing.T) {
	c := newCodec()
	data := []byte("order:\n  - verify\n  - login\n")

	f, diags := c.DecodeFolder("_folder.yaml", "requests/auth", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeFolder(f)

	if err != nil || !bytes.Equal(out, data) || len(f.Order) != 2 || f.Order[0] != "verify" {
		t.Fatalf("got %+v\n%s (%v)", f.Order, out, err)
	}
}

func TestEncodeEnvironment_should_round_trip_its_colour(t *testing.T) {
	c := newCodec()
	data := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nname: prod\ncolour: red\nvars:\n  host: example.com\n")

	env, diags := c.DecodeEnvironment("prod.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeEnvironment(env)

	if err != nil {
		t.Fatal(err)
	}
	if env.Colour != "red" {
		t.Fatalf("colour = %q, want red", env.Colour)
	}
	if !bytes.Equal(out, data) {
		t.Fatalf("round trip differs:\n%s", out)
	}
}

func TestEncodeEnvironment_should_round_trip_extends(t *testing.T) {
	c := newCodec()
	data := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nname: staging\nextends: base\nvars:\n  host: example.com\n")

	env, diags := c.DecodeEnvironment("staging.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeEnvironment(env)

	if err != nil || env.Extends != "base" || !bytes.Equal(out, data) {
		t.Fatalf("extends %q err %v\n%s", env.Extends, err, out)
	}
}

func TestTags_should_round_trip_on_a_flow_a_request_and_a_folder(t *testing.T) {
	c := newCodec()
	flow := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nname: login\ntags:\n  - smoke\n  - auth\nnodes: []\n")
	req := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXY0\nname: login\ntags:\n  - smoke\nversions:\n  \"1\":\n    method: GET\n    url: https://example.test\n")
	folder := []byte("tags:\n  - slow\n")

	f, fd := c.DecodeFlow("login.flow.yaml", "login", flow)
	r, rd := c.DecodeRequest("login.req.yaml", "login", req)
	fo, od := c.DecodeFolder("_folder.yaml", "flows", folder)

	requireNoDiagnostics(t, fd)
	requireNoDiagnostics(t, rd)
	requireNoDiagnostics(t, od)
	if len(f.Tags) != 2 || f.Tags[1] != "auth" || len(r.Tags) != 1 || len(fo.Tags) != 1 {
		t.Fatalf("flow %v request %v folder %v", f.Tags, r.Tags, fo.Tags)
	}
	out, err := c.EncodeFolder(fo)
	if err != nil || !bytes.Equal(out, folder) {
		t.Fatalf("folder round trip: %q (%v)", out, err)
	}
}

func TestEnvironment_Given_typed_values_Then_they_keep_their_type_through_a_round_trip(t *testing.T) {
	c := newCodec()
	data := []byte("id: 01J9ZE3M4N5P6Q7R8S9T0VWXYZ\nname: typed\nvars:\n  count: 5\n  ratio: 1.5\n  enabled: true\n  text: \"5\"\n  limits:\n    max: 3\n    tags:\n      - a\n  password:\n    secret: true\n")

	env, diags := c.DecodeEnvironment("typed.yaml", data)
	requireNoDiagnostics(t, diags)
	out, err := c.EncodeEnvironment(env)

	kinds := map[string]model.VarKind{}
	for _, v := range env.Vars {
		kinds[v.Name] = v.Kind
	}
	want := map[string]model.VarKind{"count": model.VarNumber, "ratio": model.VarNumber, "enabled": model.VarBoolean, "text": model.VarString, "limits": model.VarJSON, "password": model.VarString}
	for name, kind := range want {
		if kinds[name] != kind {
			t.Errorf("%s: kind %q, want %q", name, kinds[name], kind)
		}
	}
	if err != nil || !bytes.Equal(out, data) {
		t.Fatalf("round trip differs (%v):\n%s", err, out)
	}
}

func TestEnvironment_Given_a_typed_value_that_does_not_fit_Then_it_is_written_as_text_not_lost(t *testing.T) {
	c := newCodec()
	env := model.Environment{Meta: model.Meta{ID: "01J9ZE3M4N5P6Q7R8S9T0VWXYZ", Name: "x"}, Vars: []model.EnvVar{{Name: "count", Value: "five", Kind: model.VarNumber}}}

	out, err := c.EncodeEnvironment(env)

	if err != nil || !strings.Contains(string(out), "count: five") {
		t.Fatalf("%v\n%s", err, out)
	}
}
