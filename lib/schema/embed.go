// SPDX-License-Identifier: Apache-2.0

// Package schema carries the JSON Schemas of Flowi's files, so a tool can print them (flowi schema request)
// and an editor can be pointed at them with a YAML modeline.
package schema

import (
	"embed"
	"sort"
	"strings"
)

//go:embed *.schema.json
var files embed.FS

// Kinds are the file kinds that have a schema of their own ("request", "flow", ...), in name order.
// The shared definitions in common.schema.json are not a kind: no file is written against them.
func Kinds() []string {
	entries, _ := files.ReadDir(".")
	var kinds []string
	for _, e := range entries {
		if k := strings.TrimSuffix(e.Name(), ".schema.json"); k != "common" {
			kinds = append(kinds, k)
		}
	}
	sort.Strings(kinds)
	return kinds
}

// Get is the schema of one kind, and false when there is none.
func Get(kind string) ([]byte, bool) {
	if kind == "" || strings.ContainsAny(kind, "/\\.") {
		return nil, false
	}
	b, err := files.ReadFile(kind + ".schema.json")
	return b, err == nil
}

// Modeline is the comment that makes a YAML-aware editor validate and complete a file against the schema of
// its kind. It is empty for a kind with no schema.
func Modeline(kind string) string {
	if _, ok := Get(kind); !ok {
		return ""
	}
	return "# yaml-language-server: $schema=https://raw.githubusercontent.com/githugt/flowi/main/lib/schema/" + kind + ".schema.json\n"
}
