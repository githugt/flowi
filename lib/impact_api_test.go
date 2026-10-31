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
