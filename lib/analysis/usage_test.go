// SPDX-License-Identifier: Apache-2.0

package analysis

import (
	"slices"
	"strings"
	"testing"

	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/model"
	"github.com/githugt/flowi-lib/settings"
	"github.com/githugt/flowi-lib/store"
)

const (
	identityID model.ID = "01JB0000000000000000000001"
	loginID    model.ID = "01JB0000000000000000000010"
	profileID  model.ID = "01JB0000000000000000000011"
)

// loadUsage loads the identity collection and the onboarding collection
// that depends on it.
func loadUsage(t *testing.T) []model.Collection {
	t.Helper()
	s := store.New(yamlcodec.New(settings.Schema()))
	var out []model.Collection
	for _, dir := range []string{"testdata/usage/identity", "testdata/usage/onboarding"} {
		c, diags, err := s.Load(dir)
		if err != nil || diags.HasErrors() {
			t.Fatalf("loading %s: %v %v", dir, err, diags)
		}
		out = append(out, c)
	}
	return out
}

func consumersOf(t *testing.T, cs []model.Collection, request model.ID, label string) []Consumer {
	t.Helper()
	ix := newIndex(cs)
	c, r, ok := ix.request(Target{Collection: identityID, Request: request})
	if !ok {
		t.Fatalf("request %s not found", request)
	}
	for _, v := range r.Versions {
		if v.Label == label {
			return consumers(ix, c, r, v)
		}
	}
	t.Fatalf("version %s not found", label)
	return nil
}

func find(cs []Consumer, match func(Consumer) bool) (Consumer, bool) {
	i := slices.IndexFunc(cs, match)
	if i < 0 {
		return Consumer{}, false
	}
	return cs[i], true
}

func nodeIn(flow, node string) func(Consumer) bool {
	return func(c Consumer) bool { return c.Kind == KindNode && c.Flow == flow && c.Node == node }
}

func hookLabelled(substr string) func(Consumer) bool {
	return func(c Consumer) bool { return c.Kind == KindHook && strings.Contains(c.Hook, substr) }
}

func readOf(c Consumer, expr string) (Read, bool) {
	i := slices.IndexFunc(c.Reads, func(r Read) bool { return r.Expr == expr })
	if i < 0 {
		return Read{}, false
	}
	return c.Reads[i], true
}

func TestConsumers_should_find_cross_collection_node_by_dependency_name_and_id(t *testing.T) {
	cs := loadUsage(t)

	got := consumersOf(t, cs, loginID, "2")

	byName, okName := find(got, nodeIn("signup", "login"))
	_, okID := find(got, nodeIn("legacy", "login"))
	if !okName || !okID || byName.Collection != "onboarding" || byName.CollectionID != "01JC0000000000000000000001" {
		t.Fatalf("got %+v, want signup and legacy login nodes from onboarding", got)
	}
}

func TestConsumers_should_attribute_reads_from_nested_and_later_nodes(t *testing.T) {
	cs := loadUsage(t)

	c, _ := find(consumersOf(t, cs, loginID, "2"), nodeIn("signup", "login"))

	token, okToken := readOf(c, "nodes.login.token")
	header, okHeader := readOf(c, "nodes.login.headers.X-Request-Id")
	output, okOutput := readOf(c, "nodes.login.body.user.id")
	switch {
	case !okToken || token.Reader != "after" || token.Kind != ReadJSONPath || token.Target != "$.token" || token.Via != "token":
		t.Fatalf("token read %+v", token)
	case !okHeader || header.Reader != "hdr" || header.Kind != ReadHeader || header.Target != "X-Request-Id":
		t.Fatalf("header read %+v", header)
	case !okOutput || output.Reader != "" || output.Kind != ReadBody || output.Target != "$.user.id" || output.Path != "$.outputs.user":
		t.Fatalf("output read %+v", output)
	}
}

func TestConsumers_should_map_extract_names_to_their_paths(t *testing.T) {
	cs := loadUsage(t)

	c, ok := find(consumersOf(t, cs, loginID, "1"), nodeIn("local", "login"))

	token, _ := readOf(c, "nodes.login.token")
	sub, _ := readOf(c, "nodes.login.body.data.sub")
	_, status := readOf(c, "nodes.login.status")
	if !ok || token.Target != "$.data.token" || sub.Kind != ReadBody || sub.Target != "$.data.sub" || status {
		t.Fatalf("reads %+v, want token→$.data.token, sub path, no status read", c.Reads)
	}
}

func TestConsumers_should_map_cookie_extractions(t *testing.T) {
	cs := loadUsage(t)

	c, _ := find(consumersOf(t, cs, loginID, "2"), nodeIn("signup", "login"))

	session, ok := readOf(c, "nodes.login.session")
	if !ok || session.Kind != ReadCookie || session.Target != "sid" {
		t.Fatalf("session read %+v", session)
	}
}

func TestConsumers_should_include_collection_post_hooks_and_skip_pre_hooks(t *testing.T) {
	cs := loadUsage(t)

	got := consumersOf(t, cs, loginID, "1")

	audit, okAudit := find(got, hookLabelled("post · collection identity · inline #0"))
	_, okSign := find(got, hookLabelled("sign"))
	read, _ := readOf(audit, `response.headers.x-request-id`)
	if !okAudit || okSign || read.Kind != ReadHeader || read.Reader != "audit" {
		t.Fatalf("got %+v, want the collection post hook (header read) and no pre hook", got)
	}
}

func TestConsumers_should_include_hook_flow_reads_of_response(t *testing.T) {
	cs := loadUsage(t)

	c, ok := find(consumersOf(t, cs, loginID, "1"), hookLabelled("post · request auth/login · flow hooks/check-login"))

	read, _ := readOf(c, "response.body.token")
	if !ok || read.Target != "$.token" || read.Reader != "has-token" || !strings.HasSuffix(read.File, "check-login.arazzo.yaml") {
		t.Fatalf("got %+v", c)
	}
}

func TestConsumers_should_include_version_hooks_only_for_their_version(t *testing.T) {
	cs := loadUsage(t)

	_, inV1 := find(consumersOf(t, cs, loginID, "1"), hookLabelled("version 2"))
	_, inV2 := find(consumersOf(t, cs, loginID, "2"), hookLabelled("post · request auth/login version 2 · inline #0"))

	if inV1 || !inV2 {
		t.Fatalf("version hook in v1 %v, in v2 %v", inV1, inV2)
	}
}

func TestConsumers_should_include_the_versions_own_extract_and_assert(t *testing.T) {
	cs := loadUsage(t)

	c, ok := find(consumersOf(t, cs, loginID, "2"), func(c Consumer) bool { return c.Kind == KindRequest })

	extract, _ := readOf(c, "extract token")
	assert, _ := readOf(c, "response.body.user.id")
	if !ok || extract.Target != "$.token" || extract.Path != "$.versions.2.extract.token" || assert.Target != "$.user.id" {
		t.Fatalf("got %+v", c)
	}
}

func TestConsumers_should_list_unfilled_inputs(t *testing.T) {
	cs := loadUsage(t)
	got := consumersOf(t, cs, loginID, "2")

	legacy, _ := find(got, nodeIn("legacy", "login"))
	signup, _ := find(got, nodeIn("signup", "login"))

	missing, ok := readOf(legacy, "with.tenant")
	_, signupMissing := readOf(signup, "with.tenant")
	if !ok || missing.Kind != ReadInput || missing.Status != Broken || signupMissing || !slices.Equal(signup.Fills, []string{"email", "tenant"}) {
		t.Fatalf("legacy %+v signup %+v", legacy, signup)
	}
}

// A request execution starts with its own variables and no flow inputs
// (engine/execute.go): only with, pre-hook vars and non-secret env keys
// reach its templates. Counting flow vars and inputs gave false ✓.
func TestConsumers_Given_flow_var_but_no_with_Then_input_missing(t *testing.T) {
	cs := loadUsage(t)

	c, _ := find(consumersOf(t, cs, loginID, "1"), nodeIn("local", "login"))

	email, ok := readOf(c, "email")
	if !ok || email.Kind != ReadInput || email.Status != Broken {
		t.Fatalf("reads %+v, want email missing although the flow's generate writes vars.email", c.Reads)
	}
}

func TestConsumers_should_count_env_keys_and_pre_hook_vars_but_not_secrets(t *testing.T) {
	cs := loadUsage(t)

	c, _ := find(consumersOf(t, cs, loginID, "1"), nodeIn("local", "login"))

	_, region := readOf(c, "region")
	_, signature := readOf(c, "vars.signature")
	_, password := readOf(c, "password")
	if region || signature || !password {
		t.Fatalf("reads %+v, want region (env) and vars.signature (pre-hook) filled, password (secret) missing", c.Reads)
	}
}

func TestConsumers_Given_pinned_v1_Then_not_a_consumer_of_v2(t *testing.T) {
	cs := loadUsage(t)

	_, found := find(consumersOf(t, cs, loginID, "2"), nodeIn("local", "login"))

	if found {
		t.Fatal("local/login is pinned to version 1")
	}
}

// A guarded read (?? or ?.) is nil when missing, not an error, so neither
// a missing field nor a missing input breaks it (design §6.2).
func TestConsumers_should_skip_guarded_reads_and_inputs(t *testing.T) {
	cs := loadUsage(t)

	c, _ := find(consumersOf(t, cs, loginID, "2"), nodeIn("signup", "login"))

	_, nickname := readOf(c, "nodes.login.body.nickname")
	_, locale := readOf(c, "locale")
	if nickname || locale {
		t.Fatalf("reads %+v, want no guarded reads", c.Reads)
	}
}
