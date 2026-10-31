// SPDX-License-Identifier: Apache-2.0

package template

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/expr-lang/expr"
)

// Namespace is a lazily resolved scope, such as secrets that unlock the
// vault on first read. Lookup reports whether the key exists.
type Namespace interface {
	Lookup(key string) (any, bool)
}

// GeneratorFunc produces the value of a $generator.
type GeneratorFunc func(name string, args []any) (any, error)

// Scope is what expressions can read.
type Scope struct {
	// Values maps namespace names (env, vars, nodes, secret, request,
	// response, status, with, …) to map[string]any, []any, Namespace or
	// scalar values.
	Values map[string]any
	// Bare lists the namespaces searched, narrowest first, for a name
	// written without a namespace ({{email}}).
	Bare []string
	// Gen produces generator values; nil makes any generator use fail.
	Gen GeneratorFunc
	// Lenient makes undefined references nil instead of errors. Used for
	// the best-effort view of a request that pre-hooks read.
	Lenient bool
}

// Result is an evaluated template.
type Result struct {
	Value any
	// Tainted is true when a secret.* value was read, so callers can mask
	// the whole field (a Basic header built from a secret, for example).
	Tainted bool
}

// Eval evaluates the template. A template that is exactly one placeholder
// keeps the expression's type; anything else is a string.
func (t Template) Eval(s Scope) (Result, error) {
	if len(t.segments) == 1 && t.segments[0].expr != nil {
		return t.segments[0].expr.run(s)
	}
	return t.Render(s)
}

// Render evaluates the template to a string; nil becomes "". Every
// undefined reference is reported, not just the first.
func (t Template) Render(s Scope) (Result, error) {
	var out strings.Builder
	var errs []error
	tainted := false
	for _, seg := range t.segments {
		if seg.expr == nil {
			out.WriteString(seg.text)
			continue
		}
		r, err := seg.expr.run(s)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		tainted = tainted || r.Tainted
		out.WriteString(stringify(r.Value))
	}
	if len(errs) > 0 {
		return Result{}, errors.Join(errs...)
	}
	return Result{Value: out.String(), Tainted: tainted}, nil
}

// Check evaluates src, a boolean expression without braces (assert checks,
// when conditions).
func Check(src string, s Scope) (bool, Result, error) {
	e, err := compile(src)
	if err != nil {
		return false, Result{}, &SyntaxError{Detail: err.Error()}
	}
	r, err := e.run(s)
	if err != nil {
		return false, r, err
	}
	ok, isBool := r.Value.(bool)
	if !isBool {
		return false, r, fmt.Errorf("%q is %T, not a boolean", src, r.Value)
	}
	return ok, r, nil
}

// run resolves the expression's references and evaluates it.
func (e *expression) run(s Scope) (Result, error) {
	env, tainted, err := e.environment(s)
	if err != nil {
		return Result{}, err
	}
	var genErr error
	env[genFunc] = generatorCall(s.Gen, &genErr)
	env[isEmptyFunc] = isEmpty
	v, err := expr.Run(e.program, env)
	if genErr != nil {
		return Result{}, genErr
	}
	if err != nil {
		return Result{}, errors.New(firstLine(err.Error()))
	}
	return Result{Value: v, Tainted: tainted}, nil
}

// environment resolves every reference chain into the synthetic variables
// the compiled program reads.
func (e *expression) environment(s Scope) (map[string]any, bool, error) {
	env := make(map[string]any, len(e.chains)+2)
	tainted := false
	var errs []error
	for i, c := range e.chains {
		tainted = tainted || c.root() == secretRoot
		v, ok := resolve(c, s)
		if !ok && !c.guarded && !s.Lenient {
			errs = append(errs, &UndefinedError{Name: c.dotted()})
		}
		env[refName(i)] = v
	}
	return env, tainted, errors.Join(errs...)
}

// generatorCall adapts Scope.Gen for expr. Generator errors are captured
// in *errOut because expr does not preserve error types.
func generatorCall(gen GeneratorFunc, errOut *error) func(args ...any) (any, error) {
	return func(args ...any) (any, error) {
		name, _ := args[0].(string)
		if gen == nil {
			*errOut = &UnknownGeneratorError{Name: name}
			return nil, *errOut
		}
		v, err := gen(name, args[1:])
		if err != nil {
			*errOut = err
		}
		return v, err
	}
}

// resolve follows a chain from its root. A root that is not a namespace is
// searched in the Bare namespaces, narrowest first.
func resolve(c chain, s Scope) (any, bool) {
	cur, ok := rootValue(c.root(), s)
	for _, step := range c.path[1:] {
		if !ok {
			return nil, false
		}
		cur, ok = stepInto(cur, step)
	}
	return cur, ok
}

func rootValue(name string, s Scope) (any, bool) {
	if v, ok := s.Values[name]; ok {
		return v, true
	}
	for _, ns := range s.Bare {
		if v, ok := stepInto(s.Values[ns], name); ok {
			return v, true
		}
	}
	return nil, false
}

func stepInto(v any, step any) (any, bool) {
	switch c := v.(type) {
	case map[string]any:
		key, ok := step.(string)
		if !ok {
			return nil, false
		}
		got, found := c[key]
		return got, found
	case map[string]string:
		key, ok := step.(string)
		if !ok {
			return nil, false
		}
		got, found := c[key]
		return got, found
	case Namespace:
		key, ok := step.(string)
		if !ok {
			return nil, false
		}
		return c.Lookup(key)
	case []any:
		i, ok := step.(int)
		if !ok || i < 0 || i >= len(c) {
			return nil, false
		}
		return c[i], true
	}
	return nil, false
}

// stringify converts a value to the text inserted into a template.
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case time.Duration:
		return x.String()
	case time.Time:
		return x.Format(time.RFC3339Nano)
	case fmt.Stringer:
		return x.String()
	case map[string]any, []any:
		return string(marshalJSON(x))
	}
	return fmt.Sprint(v)
}

// marshalJSON encodes v without HTML escaping (bodies are not HTML).
// Values from templates are always encodable; a failure falls back to the
// value's printed form rather than losing it.
func marshalJSON(v any) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return []byte(strconv.Quote(fmt.Sprint(v)))
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

// String converts a value to text the way templates insert it.
func String(v any) string { return stringify(v) }

// CheckSyntax reports whether src, a boolean expression without braces
// (an assert check or a when condition), compiles.
func CheckSyntax(src string) error {
	if _, err := compile(src); err != nil {
		return &SyntaxError{Detail: err.Error()}
	}
	return nil
}
