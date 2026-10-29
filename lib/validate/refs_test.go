// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/model"
)

func TestValidate_Given_valid_two_collection_workspace_Then_no_diagnostics(t *testing.T) {
	f := newFixture(t)
	identity := f.collection("identity")
	verify := f.request(&identity, "auth/verify", "1", "2")
	app := f.collection("app")
	app.Manifest.DependsOn = []model.Dependency{{ID: identity.Manifest.ID, Name: "identity"}}
	login := f.request(&app, "auth/login", "1")
	foreign := requestNode("verify", verify)
	foreign.Request.Request.Collection = "identity"
	foreign.Request.Version = "2"
	f.flow(&app, "journey", requestNode("login", login), foreign)

	got := Validate(Target{Collections: []model.Collection{app, identity}})

	if len(got) != 0 {
		t.Fatalf("unexpected %v %+v", codesOf(got), got)
	}
}

func TestDuplicateIDs_should_report_once_with_related_locations_across_collections(t *testing.T) {
	f := newFixture(t)
	a := f.collection("a")
	r := f.request(&a, "login", "1")
	b := f.collection("b")
	copied := r
	copied.File = b.Root + "/requests/login.req.yaml"
	b.Requests = append(b.Requests, copied)

	got := Validate(Target{Collections: []model.Collection{a, b}})

	d := only(t, got, diag.CodeIDDuplicate)
	if d.File != copied.File || d.Path != "$.id" || len(d.Related) != 1 || d.Related[0].File != r.File {
		t.Fatalf("got %+v", d)
	}
}

func TestRefs(t *testing.T) {
	type built struct {
		target   Target
		wantFile string
	}
	tests := []struct {
		name     string
		code     diag.Code
		wantPath string
		build    func(f *fixture) built
	}{
		{
			name: "unknown_request", code: diag.CodeRefUnknownRequest, wantPath: "$.nodes[0].request.id",
			build: func(f *fixture) built {
				c := f.collection("c")
				fl := f.flow(&c, "x", model.Node{ID: "a", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: f.ids.New(), Path: "nope"}}})
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
		{
			name: "unknown_flow", code: diag.CodeRefUnknownFlow, wantPath: "$.nodes[0].flow.id",
			build: func(f *fixture) built {
				c := f.collection("c")
				fl := f.flow(&c, "x", model.Node{ID: "a", Type: "subflow", Subflow: &model.SubflowCall{Flow: model.Ref{ID: f.ids.New()}}})
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
		{
			name: "undeclared_collection", code: diag.CodeRefUndeclared, wantPath: "$.nodes[0].request.collection",
			build: func(f *fixture) built {
				c := f.collection("c")
				fl := f.flow(&c, "x", model.Node{ID: "a", Type: "request", Request: &model.RequestCall{Request: model.Ref{Collection: "other", ID: f.ids.New()}}})
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
		{
			name: "dependency_not_mounted", code: diag.CodeRefNotMounted, wantPath: "$.nodes[0].request.collection",
			build: func(f *fixture) built {
				c := f.collection("c")
				c.Manifest.DependsOn = []model.Dependency{{ID: f.ids.New(), Name: "other", Repo: "https://example.com/other"}}
				fl := f.flow(&c, "x", model.Node{ID: "a", Type: "request", Request: &model.RequestCall{Request: model.Ref{Collection: "other", ID: f.ids.New()}}})
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
		{
			name: "stale_path_hint", code: diag.CodeRefStalePath, wantPath: "$.nodes[0].request.path",
			build: func(f *fixture) built {
				c := f.collection("c")
				r := f.request(&c, "auth/login", "1")
				n := requestNode("a", r)
				n.Request.Request.Path = "auth/old"
				fl := f.flow(&c, "x", n)
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
		{
			name: "default_version_absent", code: diag.CodeVersionDefaultAbsent, wantPath: "$.default",
			build: func(f *fixture) built {
				c := f.collection("c")
				f.request(&c, "r", "1")
				c.Requests[0].Default = "3"
				return built{Target{Collections: []model.Collection{c}}, c.Requests[0].File}
			},
		},
		{
			name: "pinned_version_absent", code: diag.CodeVersionPinnedAbsent, wantPath: "$.nodes[0].version",
			build: func(f *fixture) built {
				c := f.collection("c")
				r := f.request(&c, "r", "1", "1.1")
				n := requestNode("a", r)
				n.Request.Version = "2"
				fl := f.flow(&c, "x", n)
				return built{Target{Collections: []model.Collection{c}}, fl.File}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := tt.build(newFixture(t))

			got := Validate(b.target)

			d := only(t, got, tt.code)
			if d.File != b.wantFile || d.Path != tt.wantPath {
				t.Fatalf("got %s %s, want %s %s", d.File, d.Path, b.wantFile, tt.wantPath)
			}
		})
	}
}

func TestRefs_should_report_repo_hint_for_unmounted_dependency(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	c.Manifest.DependsOn = []model.Dependency{{ID: f.ids.New(), Name: "other", Repo: "https://example.com/other"}}
	f.flow(&c, "x", model.Node{ID: "a", Type: "request", Request: &model.RequestCall{Request: model.Ref{Collection: "other", ID: f.ids.New()}}})

	d := only(t, Validate(Target{Collections: []model.Collection{c}}), diag.CodeRefNotMounted)

	if d.Args["repo"] != "https://example.com/other" || d.Args["collection"] != "other" {
		t.Fatalf("args %v", d.Args)
	}
}

func TestRefs_should_check_hook_flow_steps_and_nested_container_nodes(t *testing.T) {
	f := newFixture(t)
	c := f.collection("c")
	missingFlow := model.Ref{ID: f.ids.New()}
	c.Manifest.Hooks.Pre = []model.HookStep{{Flow: &missingFlow}}
	r := f.request(&c, "r", "1")
	c.Requests[0].Versions[0].Hooks.Post = []model.HookStep{{Inline: []model.Node{{ID: "n", Type: "subflow", Subflow: &model.SubflowCall{Flow: missingFlow}}}}}
	nested := model.Node{ID: "p", Type: "parallel", Branches: []model.Branch{
		{ID: "a", Nodes: []model.Node{requestNode("ok", r)}},
		{ID: "b", Nodes: []model.Node{{ID: "bad", Type: "request", Request: &model.RequestCall{Request: model.Ref{ID: f.ids.New()}}}}},
	}}
	f.flow(&c, "x", nested)

	got := Validate(Target{Collections: []model.Collection{c}})

	paths := map[string]diag.Code{}
	for _, d := range got {
		paths[d.Path] = d.Code
	}
	want := map[string]diag.Code{
		"$.hooks.pre[0].flow.id":                       diag.CodeRefUnknownFlow,
		"$.versions.1.hooks.post[0].inline[0].flow.id": diag.CodeRefUnknownFlow,
		"$.nodes[0].branches[1].nodes[0].request.id":   diag.CodeRefUnknownRequest,
	}
	for p, code := range want {
		if paths[p] != code {
			t.Errorf("missing %s at %s; got %v", code, p, paths)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("got %d diagnostics, want %d: %v", len(got), len(want), paths)
	}
}

func TestRefs_should_report_a_step_that_names_an_unknown_flowi_overlay(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("c")
	r := f.request(&c, "r", "1")
	c.Overlays = []model.Overlay{{Name: "vip"}}
	known, unknown := requestNode("known", r), requestNode("unknown", r)
	known.Request.Overlay, unknown.Request.Overlay = "vip", "ghost"
	f.flow(&c, "x", known, unknown)

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	d := only(t, got, diag.CodeOverlayNotFound)
	if d.Path != "$.nodes[1].overlay" || d.Args["name"] != "ghost" {
		t.Errorf("got %+v", d)
	}
}

func TestValidate_should_report_auth_blocks_that_can_never_work(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("c")
	f.request(&c, "r", "1")
	c.Requests[0].Versions[0].Auth = &model.Auth{Type: "kerberos"}
	f.request(&c, "s", "1")
	c.Requests[1].Versions[0].Auth = &model.Auth{Type: "apikey", Params: []model.KeyValue{{Key: "name", Value: "k"}, {Key: "colour", Value: "red"}}}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	var details []string
	for _, d := range got {
		if d.Code == diag.CodeAuthInvalid {
			details = append(details, d.Args["detail"])
		}
	}
	if len(details) != 2 {
		t.Fatalf("details %v", details)
	}
}

func TestValidate_should_allow_only_mutual_TLS_as_the_auth_block_of_a_gRPC_call(t *testing.T) {
	// Arrange
	f := newFixture(t)
	c := f.collection("c")
	for _, name := range []string{"mtls", "digest"} {
		f.request(&c, name, "1")
	}
	call := model.GRPC{Server: "grpcs://h:1", Method: "a.B/C"}
	c.Requests[0].Versions[0].GRPC = &call
	c.Requests[0].Versions[0].Auth = &model.Auth{Type: model.AuthMutualTLS, Params: []model.KeyValue{{Key: "cert", Value: "c.pem"}, {Key: "key", Value: "c.key"}}}
	c.Requests[1].Versions[0].GRPC = &call
	c.Requests[1].Versions[0].Auth = &model.Auth{Type: model.AuthDigest}

	// Act
	got := Validate(Target{Collections: []model.Collection{c}})

	// Assert
	var files []string
	for _, d := range got {
		if d.Code == diag.CodeAuthInvalid {
			files = append(files, d.File)
		}
	}
	if len(files) != 1 || !strings.Contains(files[0], "digest") {
		t.Fatalf("only the digest request should be reported, got %v in %v", files, got)
	}
}
