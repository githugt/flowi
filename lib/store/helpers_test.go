// SPDX-License-Identifier: Apache-2.0

package store

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/githugt/flowi-lib/diag"
	yamlcodec "github.com/githugt/flowi-lib/internal/codec/yaml"
	"github.com/githugt/flowi-lib/settings"
)

func newStore() *Store { return New(yamlcodec.New(settings.Schema())) }

// copyTree copies a testdata tree into a temp dir so tests may modify it.
func copyTree(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func codesOf(l diag.List) []diag.Code {
	var out []diag.Code
	for _, d := range l {
		out = append(out, d.Code)
	}
	return out
}

// arazzoFlow wraps a flow's own YAML (id, name, nodes, …) in the minimal
// Arazzo document a flow file is: Flowi reads the flow from x-flowi-flow.
func arazzoFlow(body string) string {
	var sb strings.Builder
	sb.WriteString("arazzo: 1.1.0\nworkflows:\n  - workflowId: w\n    x-flowi-flow:\n")
	for _, line := range strings.Split(strings.TrimRight(body, "\n"), "\n") {
		sb.WriteString("      " + line + "\n")
	}
	return sb.String()
}
