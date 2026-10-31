// SPDX-License-Identifier: Apache-2.0

package schemainfer

import (
	"errors"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// ErrBadSchema is returned when a stored schema cannot be compiled.
var ErrBadSchema = errors.New("the schema is not a valid JSON Schema")

// Violation is one place a sample breaks a schema.
type Violation struct {
	Path    string // $.user.tags[0]
	Message string
}

const resourceURL = "flowi://schema.json"

// Validate checks sample (decoded JSON) against schema. Formats are asserted
// (a date-time that is not one is a violation). A schema that does not compile
// is ErrBadSchema. Nothing is cached: schemas are small and rarely checked
// more than once per send.
func Validate(schema Schema, sample any) ([]Violation, error) {
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	if err := c.AddResource(resourceURL, map[string]any(schema)); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSchema, err)
	}
	compiled, err := c.Compile(resourceURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrBadSchema, err)
	}
	err = compiled.Validate(sample)
	var verr *jsonschema.ValidationError
	if err == nil {
		return nil, nil
	}
	if !errors.As(err, &verr) {
		return nil, fmt.Errorf("validating: %w", err)
	}
	return leaves(verr, message.NewPrinter(language.English), nil), nil
}

// leaves lists the innermost failures, which are the ones that name a place.
func leaves(e *jsonschema.ValidationError, p *message.Printer, out []Violation) []Violation {
	if len(e.Causes) == 0 {
		return append(out, Violation{Path: pathOf(e.InstanceLocation), Message: e.ErrorKind.LocalizedString(p)})
	}
	for _, c := range e.Causes {
		out = leaves(c, p, out)
	}
	return out
}

// pathOf turns instance location tokens into $.a.b[0].
func pathOf(tokens []string) string {
	var sb strings.Builder
	sb.WriteString("$")
	for _, t := range tokens {
		if isIndex(t) {
			sb.WriteString("[" + t + "]")
		} else {
			sb.WriteString("." + t)
		}
	}
	return sb.String()
}

func isIndex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
