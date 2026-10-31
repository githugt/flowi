// SPDX-License-Identifier: Apache-2.0

package validate

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/internal/yamlpath"
	"github.com/githugt/flowi-lib/model"
)

// LintFinding is one suspicious property of a value. Char is set for
// invisible characters, as "U+XXXX".
type LintFinding struct {
	Code diag.Code
	Char string
}

// invisibleRunes are characters that look like nothing (or like a space)
// but change a value: they usually arrive by copy-paste from documents.
func invisibleRunes() []rune {
	return []rune{
		0x200B, // zero-width space
		0x200C, // zero-width non-joiner
		0x200D, // zero-width joiner
		0x2060, // word joiner
		0xFEFF, // byte order mark / zero-width no-break space
		0x00A0, // no-break space
	}
}

// LintValue reports leading/trailing whitespace, line breaks or tabs, and
// invisible characters in a single-line value.
func LintValue(value string) []LintFinding {
	var out []LintFinding
	if strings.TrimFunc(value, unicode.IsSpace) != value {
		out = append(out, LintFinding{Code: diag.CodeLintWhitespace})
	}
	if strings.ContainsAny(strings.TrimFunc(value, unicode.IsSpace), "\n\r\t") {
		out = append(out, LintFinding{Code: diag.CodeLintControl})
	}
	for _, r := range invisibleRunes() {
		if strings.ContainsRune(value, r) {
			out = append(out, LintFinding{Code: diag.CodeLintInvisible, Char: fmt.Sprintf("U+%04X", r)})
		}
	}
	return out
}

// lintSite is one value to lint, the field name shown to the user, and
// where the value lives.
type lintSite struct {
	file, path, field, value string
}

func lintSites(c model.Collection) []lintSite {
	sites := []lintSite{{c.Manifest.File, "$.name", "name", c.Manifest.Name}}
	for _, r := range c.Requests {
		sites = append(sites, lintSite{r.File, "$.name", "name", r.Name})
		for _, v := range r.Versions {
			sites = append(sites, versionSites(r.File, v)...)
		}
	}
	for _, f := range c.Flows {
		sites = append(sites, lintSite{f.File, "$.name", "name", f.Name})
	}
	for _, e := range c.Environments {
		for _, v := range e.Vars {
			sites = append(sites, lintSite{e.File, yamlpath.Key("$.vars", v.Name), "variable " + v.Name, v.Value})
		}
	}
	return sites
}

func versionSites(file string, v model.RequestVersion) []lintSite {
	base := yamlpath.Key(yamlpath.Key("$", "versions"), v.Label)
	sites := []lintSite{{file, yamlpath.Key(base, "url"), "url", v.URL}}
	headers := yamlpath.Key(base, "headers")
	for _, h := range v.Headers {
		sites = append(sites,
			lintSite{file, headers, "header name " + h.Key, h.Key},
			lintSite{file, yamlpath.Key(headers, h.Key), "header " + h.Key, h.Value})
	}
	for _, q := range v.Query {
		sites = append(sites, lintSite{file, yamlpath.Key(yamlpath.Key(base, "query"), q.Key), "query " + q.Key, q.Value})
	}
	return sites
}

func checkLint(ix *index) diag.List {
	var out diag.List
	reported := map[lintSite]bool{} // a global environment is in every collection: report it once
	for _, c := range ix.collections {
		for _, s := range lintSites(c) {
			if reported[s] {
				continue
			}
			reported[s] = true
			for _, f := range LintValue(s.value) {
				args := map[string]string{"field": s.field}
				if f.Char != "" {
					args["char"] = f.Char
				}
				out = append(out, diag.New(f.Code, diag.Location{File: s.file, Path: s.path}, args))
			}
		}
	}
	return out
}
