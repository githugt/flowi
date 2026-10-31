// SPDX-License-Identifier: Apache-2.0

// Package diag defines diagnostics: user-facing problems found in files,
// settings or the workspace. Diagnostics carry a stable code and arguments
// rather than prose, so each application can render them through its own
// translation catalogue. English() is the default catalogue.
package diag

import (
	"cmp"
	"fmt"
	"slices"
)

// Code identifies a kind of diagnostic, e.g. "FLW-REF-001".
type Code string

// Severity says whether a diagnostic blocks the operation.
type Severity int

// Severities.
const (
	SeverityError   Severity = 1
	SeverityWarning Severity = 2
)

func (s Severity) String() string {
	if s == SeverityWarning {
		return "warning"
	}
	return "error"
}

// Position is a 1-based line and column; zero means unknown.
type Position struct{ Line, Column int }

// Location points at a place in a file. Path is a YAML path such as
// "$.nodes[2].needs[0]"; empty means the whole file.
type Location struct {
	File string
	Path string
	Pos  Position
}

// Diagnostic is one user-facing problem.
type Diagnostic struct {
	Code     Code
	Severity Severity
	Location
	Args    map[string]string
	Related []Location
	// Excerpt is the source around Pos (see ExcerptAt), for inputs that
	// are not collection files: imported documents and pasted text.
	Excerpt []string
}

// New builds a diagnostic with the code's default severity.
func New(c Code, loc Location, args map[string]string) Diagnostic {
	return Diagnostic{Code: c, Severity: DefaultSeverity(c), Location: loc, Args: args}
}

// List is a set of diagnostics.
type List []Diagnostic

// HasErrors reports whether any diagnostic is an error.
func (l List) HasErrors() bool {
	return slices.ContainsFunc(l, func(d Diagnostic) bool { return d.Severity == SeverityError })
}

// Sorted returns a copy ordered by file, line, column and code.
func (l List) Sorted() List {
	out := slices.Clone(l)
	slices.SortStableFunc(out, func(a, b Diagnostic) int {
		return cmp.Or(
			cmp.Compare(a.File, b.File),
			cmp.Compare(a.Pos.Line, b.Pos.Line),
			cmp.Compare(a.Pos.Column, b.Pos.Column),
			cmp.Compare(a.Code, b.Code),
		)
	})
	return out
}

// Err returns nil unless the list has errors, in which case it returns a
// *ListError carrying the whole list.
func (l List) Err() error {
	if !l.HasErrors() {
		return nil
	}
	return &ListError{List: l}
}

// ListError lets diagnostics travel through APIs that return error.
type ListError struct{ List List }

func (e *ListError) Error() string {
	errs := slices.DeleteFunc(slices.Clone(e.List), func(d Diagnostic) bool { return d.Severity != SeverityError })
	if len(errs) == 0 {
		return "no errors"
	}
	msg, _ := English().Message(errs[0])
	if len(errs) > 1 {
		msg += fmt.Sprintf(" (and %d more)", len(errs)-1)
	}
	return msg
}
