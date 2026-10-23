// SPDX-License-Identifier: Apache-2.0

// Package yamlcodec reads and writes Flow-I files as YAML.
//
// Decoding walks the parsed syntax tree instead of reflecting into structs,
// so that it can report every problem with its line and column, keep keys
// exactly as written, and preserve the order of user-ordered maps. Encoding
// produces the canonical form that `flowi fmt` writes.
package yamlcodec

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/lexer"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"

	"github.com/githugt/flowi-lib/diag"
	"github.com/githugt/flowi-lib/settings"
)

// utf8BOM is the byte order mark some Windows editors prepend.
const utf8BOM = "\xEF\xBB\xBF"

// Codec decodes and encodes Flow-I files.
type Codec struct {
	schema []settings.Key
}

// New returns a codec validating settings blocks against schema.
func New(schema []settings.Key) *Codec {
	return &Codec{schema: schema}
}

// parse turns bytes into the root node of the single YAML document.
// It strips a UTF-8 BOM (with a warning) and treats empty or comment-only
// files as an error. ok is false when nothing more can be decoded.
func (c *Codec) parse(file string, data []byte) (*decoder, ast.Node, bool) {
	d := &decoder{file: file}
	if bytes.HasPrefix(data, []byte(utf8BOM)) {
		data = data[len(utf8BOM):]
		d.diags = append(d.diags, diag.New(diag.CodeLintInvisible,
			diag.Location{File: file, Pos: diag.Position{Line: 1, Column: 1}},
			map[string]string{"char": "U+FEFF", "field": "file"}))
	}
	// Duplicate keys are detected while walking so they get their own code.
	f, err := parser.ParseBytes(data, 0, parser.AllowDuplicateMapKey())
	if err != nil {
		d.diags = append(d.diags, syntaxDiagnostic(file, err))
		return d, nil, false
	}
	if len(f.Docs) > 1 {
		// Later documents would be silently ignored, and lost by `flowi fmt`.
		d.diags = append(d.diags, extraDocument(file, f.Docs[1]))
		return d, nil, false
	}
	root := documentBody(f)
	if isNull(root) {
		d.diags = append(d.diags, diag.New(diag.CodeYAMLEmpty, diag.Location{File: file}, nil))
		return d, nil, false
	}
	return d, root, true
}

func documentBody(f *ast.File) ast.Node {
	if f == nil || len(f.Docs) == 0 {
		return nil
	}
	body := f.Docs[0].Body
	if _, comment := body.(*ast.CommentGroupNode); comment {
		return nil
	}
	return body
}

func extraDocument(file string, doc *ast.DocumentNode) diag.Diagnostic {
	loc := diag.Location{File: file}
	if doc.Start != nil && doc.Start.Position != nil {
		loc.Pos = diag.Position{Line: doc.Start.Position.Line, Column: doc.Start.Position.Column}
	}
	return diag.New(diag.CodeYAMLSyntax, loc, map[string]string{"detail": "a Flow-I file holds exactly one YAML document"})
}

// HasComments reports whether data contains YAML comments, which the
// canonical form does not keep.
func HasComments(data []byte) bool {
	for _, tk := range lexer.Tokenize(string(bytes.TrimPrefix(data, []byte(utf8BOM)))) {
		if tk.Type == token.CommentType {
			return true
		}
	}
	return false
}

func syntaxDiagnostic(file string, err error) diag.Diagnostic {
	loc := diag.Location{File: file}
	detail := err.Error()
	var se *yaml.SyntaxError
	if errors.As(err, &se) {
		detail = se.Message
		if se.Token != nil && se.Token.Position != nil {
			loc.Pos = diag.Position{Line: se.Token.Position.Line, Column: se.Token.Position.Column}
		}
	}
	return diag.New(diag.CodeYAMLSyntax, loc, map[string]string{"detail": detail})
}

// Locate returns the position of the node at a YAML path such as
// "$.nodes[1].request.id", or false when the path does not exist.
func (c *Codec) Locate(data []byte, yamlPath string) (diag.Position, bool) {
	data = bytes.TrimPrefix(data, []byte(utf8BOM))
	f, err := parser.ParseBytes(data, 0, parser.AllowDuplicateMapKey())
	if err != nil {
		return diag.Position{}, false
	}
	p, err := yaml.PathString(yamlPath)
	if err != nil {
		return diag.Position{}, false
	}
	n, err := p.FilterFile(f)
	if err != nil || n == nil {
		return diag.Position{}, false
	}
	pos := positionOf(n)
	return pos, pos.Line > 0
}

// marshal writes the canonical form: two-space indent, indented sequences,
// literal blocks for multi-line text, LF line endings, trailing newline.
func marshal(v yaml.MapSlice) ([]byte, error) {
	out, err := yaml.MarshalWithOptions(v, yaml.Indent(2), yaml.IndentSequence(true), yaml.UseLiteralStyleIfMultiline(true))
	if err != nil {
		return nil, fmt.Errorf("encoding YAML: %w", err)
	}
	return out, nil
}

// HasComments is HasComments as a method, so the codec satisfies
// interfaces that need it.
func (c *Codec) HasComments(data []byte) bool { return HasComments(data) }
