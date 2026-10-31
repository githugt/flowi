// SPDX-License-Identifier: Apache-2.0

// Package importerstest renders an importers.Result as text for golden
// files: the per-item report (with positions and excerpts) and every
// request, environment and flow in canonical YAML.
package importerstest

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi/lib/diag"
	"github.com/githugt/flowi/lib/importers"
	yamlcodec "github.com/githugt/flowi/lib/internal/codec/yaml"
	"github.com/githugt/flowi/lib/settings"
)

var update = flag.Bool("update", false, "rewrite golden files") //nolint:gochecknoglobals // test flag

// Render is r as golden text. Fatal problems (a document that does not
// parse) come first.
func Render(t *testing.T, r importers.Result, fatal diag.List) string {
	t.Helper()
	var b strings.Builder
	for _, d := range fatal {
		writeDiag(&b, "", d)
	}
	for _, it := range r.Items {
		status := "imported"
		if it.Skipped {
			status = "skipped"
		}
		fmt.Fprintf(&b, "item %q %s at %d:%d\n", it.Name, status, it.Source.Pos.Line, it.Source.Pos.Column)
		for _, d := range it.Diags {
			writeDiag(&b, "  ", d)
		}
	}
	codec := yamlcodec.New(settings.Schema())
	for _, req := range r.Requests {
		data, err := codec.EncodeRequest(req)
		check(t, err)
		fmt.Fprintf(&b, "--- request %s\n%s", req.Path, data)
	}
	for _, env := range r.Environments {
		data, err := codec.EncodeEnvironment(env)
		check(t, err)
		fmt.Fprintf(&b, "--- environment %s\n%s", env.Name, data)
	}
	for _, f := range r.Flows {
		data, err := codec.EncodeFlow(f)
		check(t, err)
		fmt.Fprintf(&b, "--- flow %s\n%s", f.Path, data)
	}
	for _, ex := range r.Examples {
		data, err := json.MarshalIndent(ex, "", "  ")
		check(t, err)
		fmt.Fprintf(&b, "--- example %s %s\n%s\n", ex.RequestPath, ex.Name, data)
	}
	return b.String()
}

func writeDiag(b *strings.Builder, indent string, d diag.Diagnostic) {
	msg, _ := diag.English().Message(d)
	fmt.Fprintf(b, "%s%s %s %d:%d %s\n", indent, d.Severity, d.Code, d.Pos.Line, d.Pos.Column, msg)
	for _, line := range d.Excerpt {
		fmt.Fprintf(b, "%s  | %s\n", indent, line)
	}
}

func check(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// Golden compares got with testdata/<name>, rewriting it under -update.
func Golden(t *testing.T, name, got string) {
	t.Helper()
	file := filepath.Join("testdata", name)
	if *update {
		check(t, os.MkdirAll(filepath.Dir(file), 0o755)) //nolint:gosec // testdata in the repository
		check(t, os.WriteFile(file, []byte(got), 0o644)) //nolint:gosec // testdata in the repository
	}
	want, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	if string(want) != got {
		t.Fatalf("%s differs; got:\n%s", name, got)
	}
}
