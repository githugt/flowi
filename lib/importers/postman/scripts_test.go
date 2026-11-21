// SPDX-License-Identifier: Apache-2.0

package postman

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/githugt/flowi/lib/model"
	"github.com/githugt/flowi/lib/template"
)

// importWithScripts imports one request whose events are the given scripts, listen name to lines.
func importWithScripts(t *testing.T, scripts map[string][]string) (model.RequestVersion, []string) {
	t.Helper()
	var events []map[string]any
	for _, listen := range []string{"prerequest", "test"} {
		if lines, ok := scripts[listen]; ok {
			events = append(events, map[string]any{"listen": listen, "script": map[string]any{"exec": lines}})
		}
	}
	coll := map[string]any{
		"info": map[string]any{"name": "c", "schema": "https://schema.getpostman.com/json/collection/v2.1.0/collection.json"},
		"item": []any{map[string]any{"name": "r", "event": events, "request": map[string]any{"method": "GET", "url": map[string]any{"raw": "https://api.example.com/x"}}}},
	}
	src, _ := json.Marshal(coll)

	r, fatal := Import(src, "c.postman_collection.json")

	if fatal != nil || len(r.Requests) != 1 {
		t.Fatalf("import: %v %+v", fatal, r.Requests)
	}
	var texts []string
	for _, it := range r.Items {
		for _, d := range it.Diags {
			texts = append(texts, string(d.Code)+" "+d.Args["what"]+" "+d.Args["how"])
		}
	}
	return r.Requests[0].Versions[0], texts
}

// Before: every script was dropped, including "save the token" and "status is 200", the two most common.
func TestImport_should_turn_saving_a_response_value_into_an_extract(t *testing.T) {
	v, diags := importWithScripts(t, map[string][]string{"test": {
		`const jsonData = pm.response.json();`,
		`pm.environment.set("token", jsonData.data.access_token);`,
		`pm.collectionVariables.set('first', pm.response.json().items[0]["id-x"]);`,
		`pm.globals.set("etag", pm.response.headers.get("ETag"));`,
		`pm.variables.set("code", pm.response.code);`,
		`pm.environment.set("whole", jsonData);`,
	}})

	want := []model.Extraction{
		{Name: "token", From: "body", Path: "$.data.access_token"},
		{Name: "first", From: "body", Path: "$.items[0]['id-x']"},
		{Name: "etag", From: "header", Header: "ETag"},
		{Name: "code", From: "status"},
		{Name: "whole", From: "body", Path: "$"},
	}
	if !slices.Equal(v.Extract, want) || v.Docs != "" || len(diags) != 0 {
		t.Fatalf("extract %+v\ndocs %q\ndiags %v", v.Extract, v.Docs, diags)
	}
}

func TestImport_should_turn_pm_test_checks_into_asserts_in_any_wrapper(t *testing.T) {
	v, diags := importWithScripts(t, map[string][]string{"test": {
		`pm.test("Status is 200", function () {`,
		`    pm.response.to.have.status(200);`,
		`});`,
		`pm.test("fast", () => { pm.expect(pm.response.responseTime).to.be.below(800); });`,
		`const body = pm.response.json();`,
		`pm.test("body", () => {`,
		`  pm.expect(body.ok).to.eql(true);`,
		`  pm.expect(body.name).to.equal('Ann "A"');`,
		`  pm.expect(body.items[0].qty).to.be.above(2);`,
		`  pm.expect(body.msg).to.include("done");`,
		`  pm.expect(pm.response.json().id).to.exist;`,
		`  pm.expect(pm.response.headers.get("Content-Type")).to.include("json");`,
		`  pm.expect(pm.response.code).to.not.eql(500);`,
		`  pm.response.to.have.header("X-Request-Id");`,
		`  pm.response.to.be.ok;`,
		`});`,
	}})

	want := []string{
		`status == 200`,
		`response.time < 800ms`,
		`response.body.ok == true`,
		`response.body.name == "Ann \"A\""`,
		`response.body.items[0].qty > 2`,
		`response.body.msg contains "done"`,
		`response.body.id != empty`,
		`response.headers["content-type"] contains "json"`,
		`status != 500`,
		`response.headers["x-request-id"] != empty`,
		`status >= 200`,
		`status < 300`,
	}
	if !slices.Equal(v.Assert, want) || v.Docs != "" || len(diags) != 0 {
		t.Fatalf("assert %q\ndocs %q\ndiags %v", v.Assert, v.Docs, diags)
	}
	for _, check := range v.Assert {
		if err := template.CheckSyntax(check); err != nil {
			t.Errorf("%q does not compile: %v", check, err)
		}
	}
	// Before: the checks read `body.…` and `headers[…]`, names a request's assert does not have (its response is
	// `response.body`), so every translated check failed with "undefined" on the first run.
	scope := template.Scope{Values: map[string]any{
		"status": 200,
		"response": map[string]any{
			"time":    100 * time.Millisecond,
			"headers": map[string]any{"content-type": "application/json", "x-request-id": "r1"},
			"body": map[string]any{"ok": true, "name": `Ann "A"`, "msg": "done", "id": 7,
				"items": []any{map[string]any{"qty": 3}}},
		},
	}}
	for _, check := range v.Assert {
		ok, _, err := template.Check(check, scope)
		if err != nil || !ok {
			t.Errorf("%q on a matching response: %v %v", check, ok, err)
		}
	}
}

func TestImport_should_keep_what_it_cannot_translate_in_the_docs_and_say_so(t *testing.T) {
	v, diags := importWithScripts(t, map[string][]string{
		"prerequest": {`pm.variables.set("ts", Date.now());`},
		"test": {
			`pm.response.to.have.status(201);`,
			`for (const x of pm.response.json().items) { console.log(x); }`,
			`pm.expect(pm.response.json().items).to.have.lengthOf(3);`,
		},
	})

	if !slices.Equal(v.Assert, []string{"status == 201"}) {
		t.Fatalf("assert %q", v.Assert)
	}
	for _, kept := range []string{"```js", "// pre-request script", `pm.variables.set("ts", Date.now());`, "for (const x of pm.response.json().items)", "lengthOf(3)"} {
		if !strings.Contains(v.Docs, kept) {
			t.Errorf("docs lack %q:\n%s", kept, v.Docs)
		}
	}
	if strings.Contains(v.Docs, "status(201)") {
		t.Errorf("a translated statement was kept too:\n%s", v.Docs)
	}
	if len(diags) != 1 || !strings.Contains(diags[0], "FLW-IMPORT-001") || !strings.Contains(diags[0], "partly translated: 1 statement became") {
		t.Fatalf("diags %v", diags)
	}
}

func TestImport_should_not_carry_a_credential_written_in_a_kept_script_line(t *testing.T) {
	v, diags := importWithScripts(t, map[string][]string{"prerequest": {
		`pm.environment.set("password", "hunter2-real-value");`,
		`const token = "eyJ.live.token";`,
	}})

	if strings.Contains(v.Docs, "hunter2") || strings.Contains(v.Docs, "eyJ.live") || !strings.Contains(v.Docs, `pm.environment.set("password", "…")`) {
		t.Fatalf("docs %q", v.Docs)
	}
	if !slices.ContainsFunc(diags, func(d string) bool {
		return strings.HasPrefix(d, "FLW-IMPORT-006")
	}) {
		t.Errorf("no report of the redaction: %v", diags)
	}
}

func TestImport_should_refuse_to_translate_what_could_change_meaning(t *testing.T) {
	v, _ := importWithScripts(t, map[string][]string{"test": {
		`pm.environment.set("a", jsonData.x);`,                  // jsonData was never assigned
		`pm.environment.set("bad name", pm.response.json().x);`, // not a variable name
		`pm.environment.set("p", pm.response.json().a.map(f));`, // a call in the path
		`pm.expect(pm.response.json().n).to.eql(null);`,         // null is not a Flowi literal
		`pm.expect(pm.response.json().n).to.include(5);`,        // contains looks for text
		`pm.expect(pm.response.responseTime).to.eql(5);`,        // time only compares
		`pm.expect(pm.response.json().a).to.be.above("x");`,     // a number is expected
		`pm.environment.set("t", pm.response.json().x);`,        // fine ...
		`pm.environment.set("t", pm.response.json().y);`,        // ... but not twice
	}})

	if len(v.Assert) != 0 || len(v.Extract) != 1 || v.Extract[0].Path != "$.x" || strings.Count(v.Docs, "\n") < 8 {
		t.Fatalf("extract %+v assert %q docs %q", v.Extract, v.Assert, v.Docs)
	}
}
