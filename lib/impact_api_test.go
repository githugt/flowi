// SPDX-License-Identifier: Apache-2.0

package flowi

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/analysis"
	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/model"
)

func loginTarget(t *testing.T, w *Workspace) RequestTarget {
	t.Helper()
	target, err := w.FindRequest("auth/login", "")
	if err != nil {
		t.Fatal(err)
	}
	return target
}

func flowNode(r analysis.Report, flow, node string) (analysis.Consumer, bool) {
	for _, v := range r.Versions {
		for _, c := range v.Consumers {
			if c.Kind == analysis.KindNode && c.Flow == flow && c.Node == node {
				return c, true
			}
		}
	}
	return analysis.Consumer{}, false
}

func TestImpact_should_report_example_collection_compatible(t *testing.T) {
	w := demoWorkspace(t)

	got, diags, err := w.Impact(loginTarget(t, w), "2")

	login, ok := flowNode(got, "user-creation", "login")
	if err != nil || len(diags) != 0 || !ok || login.Status != analysis.Compatible || got.Status != analysis.Compatible {
		t.Fatalf("got %+v %v %v", got, diags, err)
	}
	if got.Versions[0].Contract.Source != analysis.SourceExample {
		t.Fatalf("contract %+v, want the pinned example", got.Versions[0].Contract)
	}
}

func TestImpact_Given_unknown_version_Then_FLW_VER_004(t *testing.T) {
	w := demoWorkspace(t)

	_, _, err := w.Impact(loginTarget(t, w), "9")

	if codes := codesOfErr(err); !slices.Equal(codes, []diag.Code{diag.CodeVersionPinnedAbsent}) {
		t.Fatalf("got %v", err)
	}
}

// removeToken rewrites login's v2 example without "token".
func removeToken(t *testing.T, w *Workspace) string {
	t.Helper()
	file := filepath.Join(w.Collections()[0].Root, "requests", "auth", "login.examples", "v2-200.json")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), `"token"`, `"renamed"`, 1)
	if err := os.WriteFile(file, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	return file
}

func TestValidate_Given_example_missing_read_field_Then_contract_warning(t *testing.T) {
	w := demoWorkspace(t)
	removeToken(t, w)

	got := w.Validate()

	i := slices.IndexFunc(got, func(d diag.Diagnostic) bool {
		return d.Code == diag.CodeContractBroken && strings.HasSuffix(d.File, filepath.Join("flows", "user-creation.arazzo.yaml"))
	})
	if i < 0 || got.HasErrors() || got[i].Pos.Line == 0 {
		t.Fatalf("got %v, want a positioned FLW-CONTRACT-001 warning in user-creation.arazzo.yaml and no errors", got)
	}
}

func TestValidateDir_Given_example_missing_read_field_Then_contract_warning(t *testing.T) {
	w := demoWorkspace(t)
	removeToken(t, w)

	got, err := w.ValidateDir(w.Collections()[0].Root)

	if err != nil || !slices.ContainsFunc(got, func(d diag.Diagnostic) bool { return d.Code == diag.CodeContractBroken }) {
		t.Fatalf("got %v %v", got, err)
	}
}

func TestImpactOfDraft_Given_the_request_unchanged_Then_it_reports_what_Impact_reports(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]
	var target RequestTarget
	var req model.Request
	for _, r := range c.Requests {
		target, req = RequestTarget{Collection: c.Manifest.ID, Request: r.ID}, r
		if r.Path == "auth/login" {
			break
		}
	}

	saved, _, err1 := w.Impact(target, "")
	edited, _, err2 := w.ImpactOfDraft(target, req)

	if err1 != nil || err2 != nil || len(saved.Versions) != len(edited.Versions) || saved.Status != edited.Status {
		t.Fatalf("saved %+v (%v) edited %+v (%v)", saved, err1, edited, err2)
	}
}

func TestImpactOfDraft_Given_an_unknown_request_Then_an_error(t *testing.T) {
	w := demoWorkspace(t)
	c := w.Collections()[0]

	_, _, err := w.ImpactOfDraft(RequestTarget{Collection: c.Manifest.ID, Request: "nope"}, model.Request{})

	if err == nil {
		t.Fatal("want an error")
	}
}

// Before: impact was asked one request at a time, so a collection's page could not say per operation what reads it.
func TestCollectionImpact_Given_a_broken_example_When_asked_Then_the_report_of_each_used_request_comes_back(t *testing.T) {
	w := demoWorkspace(t)
	id := w.Collections()[0].Manifest.ID

	before, err := w.CollectionImpact(id)
	if err != nil {
		t.Fatal(err)
	}
	removeToken(t, w)
	after, err := w.CollectionImpact(id)

	if err != nil || len(before) == 0 {
		t.Fatalf("before %v err %v", before, err)
	}
	status := func(l []analysis.Report, request string) analysis.Status {
		for _, r := range l {
			if r.Request == request {
				return r.Status
			}
		}
		return ""
	}
	if status(before, "auth/login") != analysis.Compatible || status(after, "auth/login") != analysis.Broken {
		t.Fatalf("login was %q, is %q", status(before, "auth/login"), status(after, "auth/login"))
	}
	if _, err := w.CollectionImpact("nope"); err == nil {
		t.Fatal("an unknown collection gave no error")
	}
}
