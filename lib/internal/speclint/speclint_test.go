// SPDX-License-Identifier: Apache-2.0

package speclint

import (
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
)

const goodSpec = `
openapi: 3.0.3
info: {title: Pets, version: "1", description: The pet shop}
servers: [{url: "https://pets.example.test"}]
security: [{key: []}]
paths:
  /pets:
    get:
      summary: List pets
      operationId: listPets
      tags: [pets]
      responses: {"200": {description: ok, content: {application/json: {schema: {$ref: "#/components/schemas/Pet"}}}}}
components:
  schemas:
    Pet: {type: object}
  securitySchemes: {key: {type: apiKey, in: header, name: X-Key}}
`

func codesOf(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		out = append(out, d.Code)
	}
	return out
}

func lint(t *testing.T, spec string, cfg Config) diag.List {
	t.Helper()
	l, err := Lint([]byte(spec), "pets", "descriptions/pets.openapi.yaml", cfg)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestLint_Given_a_complete_description_When_linted_Then_nothing_is_found(t *testing.T) {
	got := lint(t, goodSpec, nil)

	if len(got) != 0 {
		t.Fatalf("findings = %v", got)
	}
}

func TestLint_Given_a_bare_operation_When_linted_Then_each_gap_is_found_where_it_is(t *testing.T) {
	spec := `
openapi: 3.0.3
info: {title: Pets, version: "1"}
paths:
  /Pets:
    post:
      responses: {"400": {description: bad}}
`

	got := lint(t, spec, nil)

	want := []diag.Code{diag.CodeSpecInfo, diag.CodeSpecSummary, diag.CodeSpecOperation, diag.CodeSpecTags, diag.CodeSpecSecurity, diag.CodeSpecSuccess}
	if !slices.Equal(codesOf(got), want) {
		t.Fatalf("codes = %v, want %v", codesOf(got), want)
	}
	if got[1].Path != "$.paths.'/Pets'.post" || got[1].Args["where"] != "POST /Pets" || got[1].File != "descriptions/pets.openapi.yaml" {
		t.Fatalf("operation finding = %+v", got[1])
	}
}

func TestLint_Given_an_unused_component_When_linted_Then_it_is_found_and_a_used_one_is_not(t *testing.T) {
	spec := strings.Replace(goodSpec, "Pet: {type: object}", "Pet: {type: object}\n    Old: {type: object}\n    'a/b': {type: object}", 1)

	got := lint(t, spec, nil)

	var unused []string
	for _, d := range got {
		if d.Code == diag.CodeSpecUnused {
			unused = append(unused, d.Args["component"])
		}
	}
	if !slices.Equal(unused, []string{"schemas/Old", "schemas/a/b"}) {
		t.Fatalf("unused = %v", unused)
	}
}

func TestLint_Given_a_component_named_with_a_slash_When_referred_to_escaped_Then_it_is_used(t *testing.T) {
	spec := strings.Replace(goodSpec, "Pet: {type: object}", "Pet: {type: object}\n    'a/b': {type: object}", 1)
	spec = strings.Replace(spec, "{schema: {$ref: \"#/components/schemas/Pet\"}}", "{schema: {allOf: [{$ref: \"#/components/schemas/Pet\"}, {$ref: \"#/components/schemas/a~1b\"}]}}", 1)

	got := lint(t, spec, nil)

	if len(got) != 0 {
		t.Fatalf("findings = %v", got)
	}
}

func TestLint_Given_security_decided_per_operation_When_linted_Then_an_empty_list_passes(t *testing.T) {
	spec := strings.Replace(goodSpec, "security: [{key: []}]\n", "", 1)
	if got := lint(t, spec, nil); len(got) != 1 || got[0].Code != diag.CodeSpecSecurity {
		t.Fatalf("no security anywhere: %v", got)
	}

	open := strings.Replace(spec, "      operationId: listPets", "      operationId: listPets\n      security: []", 1)
	if got := lint(t, open, nil); len(got) != 0 {
		t.Fatalf("a public operation on purpose: %v", got)
	}
}

func TestLint_Given_servers_When_linted_Then_plain_http_is_found_unless_it_stays_on_the_machine(t *testing.T) {
	for url, flagged := range map[string]bool{
		"http://pets.example.test": true, "https://pets.example.test": false, "http://localhost:8080": false,
		"http://127.0.0.1/v1": false, "http://[::1]:9": false, "http://dev.localhost": false,
	} {
		spec := strings.Replace(goodSpec, "https://pets.example.test", url, 1)

		got := lint(t, spec, nil)

		if (len(got) == 1 && got[0].Code == diag.CodeSpecPlaintext) != flagged || len(got) > 1 {
			t.Errorf("%s: findings = %v, want flagged %v", url, codesOf(got), flagged)
		}
	}
}

func TestLint_Given_a_config_When_linted_Then_rules_are_off_or_errors(t *testing.T) {
	spec := strings.Replace(goodSpec, "      summary: List pets\n", "", 1)
	spec = strings.Replace(spec, "/pets:", "/Pets:", 1)

	got := lint(t, spec, Config{"operation-description": LevelError, "path-case": LevelWarning, "operation-tags": LevelOff})

	if !slices.Equal(codesOf(got), []diag.Code{diag.CodeSpecSummary, diag.CodeSpecPathCase}) {
		t.Fatalf("codes = %v", codesOf(got))
	}
	if got[0].Severity != diag.SeverityError || got[1].Severity != diag.SeverityWarning {
		t.Fatalf("severities = %v, %v", got[0].Severity, got[1].Severity)
	}
}

func TestLint_Given_path_parameters_When_checking_case_Then_only_the_literal_segments_count(t *testing.T) {
	spec := strings.Replace(goodSpec, "/pets:", "/pets/{petId}:", 1)

	got := lint(t, spec, Config{"path-case": LevelWarning})

	if len(got) != 0 {
		t.Fatalf("findings = %v", got)
	}
}

// Before: only OpenAPI 3 was linted, so a Swagger 2 description kept at import was never checked.
func TestLint_Given_a_Swagger_2_description_When_linted_Then_its_own_fields_are_read(t *testing.T) {
	spec := `
swagger: "2.0"
info: {title: Pets, version: "1"}
host: pets.example.test
schemes: [http]
paths:
  /Pets:
    get:
      responses: {"404": {description: none}}
definitions:
  Pet: {type: object}
`

	got := lint(t, spec, Config{"path-case": LevelWarning})

	want := []diag.Code{diag.CodeSpecInfo, diag.CodeSpecSummary, diag.CodeSpecOperation, diag.CodeSpecTags, diag.CodeSpecSecurity, diag.CodeSpecSuccess, diag.CodeSpecUnused, diag.CodeSpecPlaintext, diag.CodeSpecPathCase}
	if !slices.Equal(codesOf(got), want) {
		t.Fatalf("codes = %v, want %v", codesOf(got), want)
	}
	if got[6].Path != "$.definitions.Pet" || got[7].Path != "$.schemes" {
		t.Fatalf("paths = %q, %q", got[6].Path, got[7].Path)
	}
}

func TestLint_Given_a_Swagger_2_description_on_this_machine_When_linted_Then_plain_http_passes(t *testing.T) {
	spec := "swagger: '2.0'\ninfo: {title: x, version: '1', description: d}\nhost: localhost:8080\nschemes: [http]\npaths: {}\n"

	if got := lint(t, spec, nil); len(got) != 0 {
		t.Fatalf("findings = %v", got)
	}
}

const asyncV2 = `
asyncapi: 2.6.0
info: {title: Orders, version: "1"}
servers:
  prod: {url: broker.example.test, protocol: mqtt}
  secure: {url: broker.example.test, protocol: mqtts}
  dev: {url: localhost:1883, protocol: mqtt}
channels:
  orders/created:
    publish:
      message: {$ref: "#/components/messages/Created"}
components:
  messages:
    Created: {payload: {type: object}}
    Spare: {payload: {type: object}}
`

// Before: AsyncAPI was kept at import and never linted.
func TestLint_Given_an_AsyncAPI_2_description_When_linted_Then_channels_servers_and_components_are_checked(t *testing.T) {
	got := lint(t, asyncV2, nil)

	want := []diag.Code{diag.CodeSpecInfo, diag.CodeSpecSummary, diag.CodeSpecOperation, diag.CodeSpecTags, diag.CodeSpecUnused, diag.CodeSpecPlaintext}
	if !slices.Equal(codesOf(got), want) {
		t.Fatalf("codes = %v, want %v", codesOf(got), want)
	}
	if got[1].Args["where"] != "PUBLISH orders/created" || got[1].Path != "$.channels.'orders/created'.publish" {
		t.Fatalf("operation finding = %v", got[1])
	}
	if got[4].Path != "$.components.messages.Spare" || got[5].Path != "$.servers.prod" {
		t.Fatalf("paths = %q, %q", got[4].Path, got[5].Path)
	}
}

func TestLint_Given_an_AsyncAPI_3_description_When_linted_Then_operations_need_no_operationId(t *testing.T) {
	spec := `
asyncapi: 3.0.0
info: {title: Orders, version: "1", description: d}
operations:
  sendCreated: {action: send, channel: {$ref: "#/channels/created"}, summary: Announce, tags: [{name: orders}]}
channels:
  created: {address: orders.created}
`

	if got := lint(t, spec, nil); len(got) != 0 {
		t.Fatalf("findings = %v", got)
	}
}

func TestLint_Given_an_OpenRPC_description_When_linted_Then_methods_are_the_operations(t *testing.T) {
	spec := `
openrpc: 1.2.6
info: {title: Calc, version: "1", description: d}
servers: [{name: live, url: "http://calc.example.test/rpc"}]
methods:
  - name: add
    summary: Adds
    tags: [{name: math}]
    result: {name: sum, schema: {type: number}}
  - name: sub
    result: {name: diff, schema: {type: number}}
components:
  schemas:
    Unused: {type: number}
`

	got := lint(t, spec, nil)

	want := []diag.Code{diag.CodeSpecSummary, diag.CodeSpecTags, diag.CodeSpecUnused, diag.CodeSpecPlaintext}
	if !slices.Equal(codesOf(got), want) {
		t.Fatalf("codes = %v, want %v", codesOf(got), want)
	}
	if got[0].Path != "$.methods[1]" || got[0].Args["where"] != "sub" {
		t.Fatalf("method finding = %v", got[0])
	}
}

func TestLint_Given_a_document_of_no_known_kind_When_linted_Then_it_is_left_alone(t *testing.T) {
	got, err := Lint([]byte("title: not a description\n"), "x", "x.yaml", nil)

	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v", got, err)
	}
}

const protoSource = `syntax = "proto3";
package shop;

// Orders takes and tracks orders.
service Orders {
  // Create takes an order.
  rpc Create(Order) returns (Order);
  rpc Cancel(Order) returns (Order);
}

service Bare {
  rpc Ping(Order) returns (Order);
}

message Order {}
`

// Before: a gRPC service was never linted; its findings have a line and no YAML path.
func TestLintProto_Given_services_and_rpcs_When_linted_Then_the_ones_without_a_comment_are_found_by_line(t *testing.T) {
	got := LintProto("shop.proto", []byte(protoSource), nil, "protos/shop.proto", nil)

	if len(got) != 3 {
		t.Fatalf("findings = %v", got)
	}
	wantLines := []int{8, 11, 12}
	wantCodes := []diag.Code{diag.CodeSpecSummary, diag.CodeSpecInfo, diag.CodeSpecSummary}
	for i, d := range got {
		if d.Pos.Line != wantLines[i] || d.Code != wantCodes[i] || d.Path != "" {
			t.Fatalf("finding %d = %+v, want line %d code %s", i, d, wantLines[i], wantCodes[i])
		}
	}
	if got[0].Args["where"] != "Orders.Cancel" || got[1].Args["spec"] != "shop.proto Bare" {
		t.Fatalf("args = %v, %v", got[0].Args, got[1].Args)
	}
}

func TestLintProto_Given_a_rule_off_or_a_file_that_does_not_compile_When_linted_Then_nothing_is_said(t *testing.T) {
	off := Config{"info-description": LevelOff, "operation-description": LevelOff}

	if got := LintProto("shop.proto", []byte(protoSource), nil, "f", off); len(got) != 0 {
		t.Fatalf("rules off: %v", got)
	}
	if got := LintProto("bad.proto", []byte("this is not proto"), nil, "f", nil); len(got) != 0 {
		t.Fatalf("broken file: %v", got)
	}
}

func TestParseConfig_Given_rules_When_parsed_Then_names_and_levels_are_checked_one_by_one(t *testing.T) {
	data := "rules:\n  path-case: error\n  operation-tags: warn\n  made-up: off\n  info-description: loud\n"

	cfg, diags := ParseConfig([]byte(data), "descriptions/lint.yaml")

	if cfg["path-case"] != LevelError || cfg["operation-tags"] != LevelWarning || len(cfg) != 2 {
		t.Fatalf("cfg = %v", cfg)
	}
	if len(diags) != 2 || diags[0].Code != diag.CodeSpecLintConfig || !strings.Contains(diags[0].Args["detail"], "loud") || !strings.Contains(diags[1].Args["detail"], "made-up") {
		t.Fatalf("diags = %v", diags)
	}
}

func TestParseConfig_Given_broken_yaml_When_parsed_Then_one_problem(t *testing.T) {
	_, diags := ParseConfig([]byte("rules: [unclosed"), "descriptions/lint.yaml")

	if len(diags) != 1 {
		t.Fatalf("diags = %v", diags)
	}
}

func TestDefaultLevel_should_have_naming_rules_off_and_the_rest_on(t *testing.T) {
	if DefaultLevel("path-case") != LevelOff || DefaultLevel("operation-id") != LevelWarning || DefaultLevel("nope") != "" {
		t.Fatal("defaults changed")
	}
}

func severityOf(l diag.List, code diag.Code) (diag.Severity, bool) {
	for _, d := range l {
		if d.Code == code {
			return d.Severity, true
		}
	}
	return 0, false
}

func TestLint_Given_a_missing_tag_When_the_level_is_chosen_or_not_Then_it_is_a_note_by_default_and_what_the_collection_says(t *testing.T) {
	// Before: a missing tag was a warning, though tags are a nicety and not a requirement.
	spec := `
openapi: 3.0.3
info: {title: Pets, version: "1", description: d}
paths:
  /pets:
    get:
      summary: List
      operationId: l
      security: []
      responses: {"200": {description: ok}}
`

	byDefault, _ := severityOf(lint(t, spec, nil), diag.CodeSpecTags)
	asNote, _ := severityOf(lint(t, spec, Config{"operation-tags": LevelNote}), diag.CodeSpecTags)
	asWarning, _ := severityOf(lint(t, spec, Config{"operation-tags": LevelWarning}), diag.CodeSpecTags)
	asError, _ := severityOf(lint(t, spec, Config{"operation-tags": LevelError}), diag.CodeSpecTags)
	_, whenOff := severityOf(lint(t, spec, Config{"operation-tags": LevelOff}), diag.CodeSpecTags)

	if byDefault != diag.SeverityNote || asNote != diag.SeverityNote {
		t.Errorf("default %v, note %v: both want a note", byDefault, asNote)
	}
	if asWarning != diag.SeverityWarning || asError != diag.SeverityError || whenOff {
		t.Errorf("warning %v, error %v, off found %v", asWarning, asError, whenOff)
	}
	if DefaultLevel("operation-tags") != LevelNote || DefaultLevel("operation-id") != LevelWarning {
		t.Errorf("only the tag rule is a note by default")
	}
}

func TestParseConfig_Given_note_When_read_Then_it_is_a_level(t *testing.T) {
	cfg, problems := ParseConfig([]byte("rules:\n  operation-id: note\n"), "lint.yaml")

	if len(problems) != 0 || cfg["operation-id"] != LevelNote {
		t.Fatalf("cfg %v problems %v", cfg, problems)
	}
}
